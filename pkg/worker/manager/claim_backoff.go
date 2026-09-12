package manager

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
)

type claimBackoff struct {
	policy         *common.RetryPolicy
	jitterFraction float64
	rows           map[int64]declinedClaim // by worker row id
}

type declinedClaim struct {
	attempts int
	until    time.Time
}

func newClaimBackoff(policy *common.RetryPolicy, jitterFraction float64) (*claimBackoff, error) {
	if policy == nil {
		return nil, errors.New("policy must not be nil")
	}
	if jitterFraction < 0 || jitterFraction >= 1 {
		return nil, fmt.Errorf("jitterFraction must be in [0, 1), got %v", jitterFraction)
	}

	return &claimBackoff{
		policy:         policy,
		jitterFraction: jitterFraction,
		rows:           make(map[int64]declinedClaim),
	}, nil
}

func (b *claimBackoff) waiting(id int64, now time.Time) bool {
	return now.Before(b.rows[id].until)
}

func (b *claimBackoff) declined(id int64, now time.Time) time.Duration {
	attempts := b.rows[id].attempts + 1

	// Fresh jitter prevents replicas from repeating simultaneous claim attempts.
	jitter := 1 + b.jitterFraction*(2*rand.Float64()-1)
	delay := time.Duration(float64(b.policy.CalculateDelay(min(attempts-1, b.policy.MaxRetries))) * jitter)
	b.rows[id] = declinedClaim{attempts: attempts, until: now.Add(delay)}
	return delay
}

func (b *claimBackoff) clear(id int64) {
	delete(b.rows, id)
}

func (b *claimBackoff) keep(want map[int64]bool) {
	for id := range b.rows {
		if !want[id] {
			delete(b.rows, id)
		}
	}
}
