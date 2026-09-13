package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"testing/synctest"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common/diagnostic"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	errTestConnection = diagnostic.NewDiagnosticError("SQL9905", diagnostic.RecoveryTransient,
		"could not reach the test database", "")
	errTestStreamMissing = diagnostic.NewDiagnosticError("SQL9906", diagnostic.RecoveryPermanent,
		"test stream not found", "")
)

// closed set: what IsTransientDatastoreError concludes for each kind of
// error it can meet -- a declared recovery wins over a wrapped cause, a bare
// SQLSTATE speaks for itself, and our own cancellation is never transient
// even though the driver reports it with the retryable 57014.
func TestIsTransientDatastoreErrorByErrorKind(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "transient recovery", err: errTestConnection.With("host", "db.local"), want: true},
		{name: "permanent recovery", err: errTestStreamMissing.With("stream", "orders"), want: false},
		{name: "transient recovery under fmt.Errorf", err: fmt.Errorf("list streams: %w", errTestConnection), want: true},
		{name: "permanent recovery over a wrapped deadlock", err: errTestStreamMissing.Wrap(&pgconn.PgError{Code: "40P01"}), want: false},
		{name: "bare deadlock", err: &pgconn.PgError{Code: "40P01"}, want: true},
		{name: "bare query_canceled", err: &pgconn.PgError{Code: "57014"}, want: true},
		{name: "own cancellation", err: fmt.Errorf("claim: %w", context.Canceled), want: false},
		{name: "unclassified", err: errors.New("no classification anywhere"), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsTransientDatastoreError(test.err); got != test.want {
				t.Fatalf("IsTransientDatastoreError(%s) = %v, want %v", test.name, got, test.want)
			}
		})
	}
}

// behavior: WrapNonIdempotent retries a transient error through the whole curve and
// returns the declared error, and stops at the first attempt on a permanent
// or unclassified one.
func TestWrapNonIdempotentRetriesOnlyTransientErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantAttempts int
	}{
		{name: "transient recovery", err: errTestConnection.With("host", "db.local"), wantAttempts: 3},
		{name: "permanent recovery", err: errTestStreamMissing.With("stream", "orders"), wantAttempts: 1},
		{name: "unclassified", err: errors.New("no classification anywhere"), wantAttempts: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				retryDatastore := newTestRetryDatastore(t)

				attempts := 0
				err := retryDatastore.WrapNonIdempotent(t.Context(), func() error {
					attempts++
					return test.err
				})
				if !errors.Is(err, test.err) {
					t.Fatalf("WrapNonIdempotent(%s) = %v, want the returned error", test.name, err)
				}
				if attempts != test.wantAttempts {
					t.Fatalf("WrapNonIdempotent(%s) attempts = %d, want %d", test.name, attempts, test.wantAttempts)
				}
			})
		})
	}
}

// closed set: the SQLSTATEs the two verbs retry. A code the server rolled
// back or never received retries under both; a code where the statement may
// have committed retries only under WrapIdempotent, and WrapNonIdempotent
// returns it on the first attempt because the caller's write may have landed.
func TestWrapNonIdempotentRetriesOnlyStatementsThatCannotHaveCommitted(t *testing.T) {
	tests := []struct {
		code                      string
		wantNonIdempotentAttempts int
	}{
		{code: "40P01", wantNonIdempotentAttempts: 2}, {code: "40001", wantNonIdempotentAttempts: 2},
		{code: "08001", wantNonIdempotentAttempts: 2}, {code: "08003", wantNonIdempotentAttempts: 2}, {code: "53300", wantNonIdempotentAttempts: 2},
		{code: "57014", wantNonIdempotentAttempts: 2},
		{code: "08000", wantNonIdempotentAttempts: 1}, {code: "08006", wantNonIdempotentAttempts: 1}, {code: "08007", wantNonIdempotentAttempts: 1}, {code: "40003", wantNonIdempotentAttempts: 1},
		{code: "57P01", wantNonIdempotentAttempts: 1}, {code: "57P02", wantNonIdempotentAttempts: 1}, {code: "57P03", wantNonIdempotentAttempts: 1}, {code: "57P05", wantNonIdempotentAttempts: 1},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				retryDatastore := newTestRetryDatastore(t)
				cause := &pgconn.PgError{Code: test.code}

				nonIdempotentAttempts := 0
				nonIdempotentErr := retryDatastore.WrapNonIdempotent(t.Context(), func() error {
					nonIdempotentAttempts++
					if nonIdempotentAttempts == 1 {
						return cause
					}
					return nil
				})
				idempotentAttempts := 0
				idempotentErr := retryDatastore.WrapIdempotent(t.Context(), func() error {
					idempotentAttempts++
					if idempotentAttempts == 1 {
						return cause
					}
					return nil
				})

				if nonIdempotentAttempts != test.wantNonIdempotentAttempts {
					t.Errorf("WrapNonIdempotent(%s) attempts = %d, want %d", test.code, nonIdempotentAttempts, test.wantNonIdempotentAttempts)
				}
				if test.wantNonIdempotentAttempts == 1 && !errors.Is(nonIdempotentErr, cause) {
					t.Errorf("WrapNonIdempotent(%s) = %v, want the ambiguous error returned as is", test.code, nonIdempotentErr)
				}
				if test.wantNonIdempotentAttempts == 2 && nonIdempotentErr != nil {
					t.Errorf("WrapNonIdempotent(%s) = %v, want nil after the retry", test.code, nonIdempotentErr)
				}
				if idempotentErr != nil || idempotentAttempts != 2 {
					t.Errorf("WrapIdempotent(%s) = %v after %d attempts, want nil after 2", test.code, idempotentErr, idempotentAttempts)
				}
			})
		})
	}
}

// closed set: the SQLSTATEs WrapNonIdempotent never retries -- listed deliberately rather
// than left to the switch's default, so a code moving between the two tables
// is a visible edit.
func TestWrapNonIdempotentStopsOnPermanentSqlStates(t *testing.T) {
	codes := []string{
		"08004", "08P01",
		"40002",
		"53000", "53100", "53200", "53400",
		"57P04",
		"58000", "58030", "58P01", "58P02",
		"XX000", "XX001", "XX002",
		"25P02",
		"42P01", "42P07", "23514", "23505",
	}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			retryDatastore := newTestRetryDatastore(t)

			attempts := 0
			cause := &pgconn.PgError{Code: code}
			err := retryDatastore.WrapNonIdempotent(t.Context(), func() error {
				attempts++
				return cause
			})
			if !errors.Is(err, cause) || attempts != 1 {
				t.Fatalf("WrapNonIdempotent(%s) = %v after %d attempts, want the error after 1", code, err, attempts)
			}
		})
	}
}

// ***************
// *** HELPERS ***
// ***************

func newTestRetryDatastore(t testing.TB) *RetryDatastore {
	t.Helper()
	policy := &RetryPolicy{MaxRetries: 3, BaseDelay: time.Second, MaxDelay: time.Second, Exponent: 1}
	retryDatastore, err := NewRetryDatastore(policy, logging.NewDefaultLogger(io.Discard))
	if err != nil {
		t.Fatal(err)
	}
	return retryDatastore
}
