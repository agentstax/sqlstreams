package datastore

import (
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/datastore"
)

func TestNewProduceDatastoreRejectsTimeoutOverflow(t *testing.T) {
	for _, sample := range []struct {
		name   string
		policy common.RetryPolicy
	}{
		{"operation multiplication", common.RetryPolicy{
			MaxRetries: int(time.Duration(math.MaxInt64)/createAheadAttemptAllowance) + 1,
			BaseDelay:  time.Nanosecond, MaxDelay: time.Nanosecond, Exponent: 1,
		}},
		{"combined addition", common.RetryPolicy{
			MaxRetries: 2,
			BaseDelay:  time.Duration(math.MaxInt64) - time.Second,
			MaxDelay:   time.Duration(math.MaxInt64) - time.Second, Exponent: 1,
		}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			if err := sample.policy.Validate(); err != nil {
				t.Fatalf("retry sleep budget should be valid: %v", err)
			}
			created, err := NewProduceDatastore(&datastore.PostgresDatastore{Retry: &sample.policy}, slog.Default())
			if created != nil || err == nil || !strings.Contains(err.Error(), "create-ahead timeout exceeds") {
				t.Fatalf("expected combined-timeout rejection, got %v, %v", created, err)
			}
		})
	}
}

func TestNewProduceDatastoreDefaultTimeout(t *testing.T) {
	created, err := NewProduceDatastore(&datastore.PostgresDatastore{}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	// Six attempts have 31s of sleeps and 36s of operation allowance.
	if created.createAheadTimeout != 67*time.Second {
		t.Fatalf("default timeout = %v, want 67s", created.createAheadTimeout)
	}
}
