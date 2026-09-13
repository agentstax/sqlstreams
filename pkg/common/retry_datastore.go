package common

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common/diagnostic"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	"github.com/jackc/pgx/v5/pgconn"
)

type RetryableFunc func() error

// RetryDatastore reruns a datastore call on transient errors, shared by
// producer/consumer/stream: one backoff/attempt machinery -- errors surface
// as-is, never rewrapped.
type RetryDatastore struct {
	*RetryPolicy
	Logger logging.Logger
}

// policy may be nil or sparse.
func NewRetryDatastore(policy *RetryPolicy, log logging.Logger) (*RetryDatastore, error) {
	if log == nil {
		return nil, errors.New("logger must not be nil")
	}
	policy = policy.WithDefaults()
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &RetryDatastore{
		RetryPolicy: policy,
		Logger:      log,
	}, nil
}

// ambiguous outcome -> returned to the caller, whose write may have landed
func (r *RetryDatastore) WrapNonIdempotent(ctx context.Context, retryableFunc RetryableFunc) error {
	return r.wrap(ctx, retryableFunc, IsRetryableDatastoreError)
}

// ambiguous outcome -> retried; retryableFunc's second run must be harmless
func (r *RetryDatastore) WrapIdempotent(ctx context.Context, retryableFunc RetryableFunc) error {
	return r.wrap(ctx, retryableFunc, IsTransientDatastoreError)
}

func (r *RetryDatastore) wrap(ctx context.Context, retryableFunc RetryableFunc, retryable func(error) bool) error {
	var retryErrs []error
	for retryCount := range r.MaxRetries {
		// respect context cancelation
		if ctx.Err() != nil {
			return errors.Join(append(retryErrs, ctx.Err())...)
		}

		err := retryableFunc()

		if err == nil {
			return nil // success -- prior (now-irrelevant) retry errors don't belong in the result
		}

		// permanent, or ambiguous under WrapNonIdempotent -> exit early
		if !retryable(err) {
			return errors.Join(append(retryErrs, err)...)
		}

		retryErrs = append(retryErrs, err)

		// last attempt already spent -- no point sleeping before returning
		if retryCount == r.MaxRetries-1 {
			break
		}

		delay := r.CalculateDelay(retryCount)

		r.Logger.DebugContext(ctx, "retrying datastore call", "attempt", retryCount+1, "max_retries", r.MaxRetries, "delay", delay, "error", err)

		select {
		case <-ctx.Done():
			return errors.Join(append(retryErrs, ctx.Err())...)
		case <-time.After(delay):
			continue
		}
	}

	return errors.Join(retryErrs...)
}

// IsTransientDatastoreError answers "can an unchanged retry succeed?":
// recovery declared on the error decides; a bare error is judged by
// IsTransientPgError. It is WrapIdempotent's classification.
func IsTransientDatastoreError(err error) bool {
	if classified, ok := errors.AsType[*diagnostic.DiagnosticError](err); ok {
		return classified.Recovery() == diagnostic.RecoveryTransient
	}
	return IsTransientPgError(err)
}

// IsRetryableDatastoreError is WrapNonIdempotent's classification: transient, and the
// failed statement cannot have committed. A declared error is a library
// verdict rather than a lost connection, so its recovery decides as it does
// above; a bare error is judged by IsRetryablePgError.
func IsRetryableDatastoreError(err error) bool {
	if classified, ok := errors.AsType[*diagnostic.DiagnosticError](err); ok {
		return classified.Recovery() == diagnostic.RecoveryTransient
	}
	return IsRetryablePgError(err)
}

// IsTransientPgError reports whether a retry could succeed -- never a
// deterministic rejection (a business-logic *pgconn.PgError, ErrLeaseLost).
// It includes the ambiguous outcomes IsRetryablePgError excludes.
func IsTransientPgError(err error) bool {
	if IsRetryablePgError(err) {
		return true
	}
	return isAmbiguousPgError(err)
}

// IsRetryablePgError reports whether a retry is safe for any statement: the
// server rolled the statement back, or never received it.
func IsRetryablePgError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pgErr.Code {
		// deadlock / serialization_failure -- whole txn provably rolled back.
		case "40P01", "40001":
			return true

		// never sent anything -- nothing could have landed.
		case "08001", "08003", "53300":
			return true

		// query_canceled -- only external cancels reach here; ours are
		// already filtered above. Aborts cleanly.
		case "57014":
			return true
		}
		return false
	}

	return pgconn.SafeToRetry(err)
}

// isAmbiguousPgError reports a connection that died after a statement may
// have shipped, so the outcome is genuinely unknown.
func isAmbiguousPgError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pgErr.Code {
		// the connection failed mid-statement.
		case "08000", "08006", "08007", "40003":
			return true

		// the same, caused by an admin command or restart instead.
		case "57P01", "57P02", "57P03", "57P05":
			return true
		}
		return false
	}

	if pgconn.Timeout(err) {
		return true
	}

	_, ok := errors.AsType[net.Error](err)
	return ok
}
