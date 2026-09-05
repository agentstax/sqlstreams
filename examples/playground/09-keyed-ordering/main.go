// Scenario 09 -- keyed ordering: what a same-key consumer actually sees.
//
// Account balance updates for one account must apply in order and never
// overlap. The producer keys by account; the consumer runs concurrently.
//
// Concepts held before domain code (8): the produce set from scenario 01,
// plus MessageKey, ConsumerConfig.ConcurrencyOverride (ConcurrencyOrdered),
// the session's ConsumeOptions.MessageConcurrency, and the "ordered =
// every same-key message in id order, one at a time, through failures"
// semantics.
//
// Traps hit:
//   - A message key alone orders nothing: MessageConcurrency > 1 delivers
//     two same-key messages at once unless the group declares
//     ConcurrencyOverride. The per-message MessageOptions.Concurrency form
//     also exists, and a second producer that sets the key without it
//     runs in parallel with the first -- so the group form leads here.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

type BalanceChanged struct {
	AccountId string `json:"account_id"`
	Delta     int64  `json:"delta_cents"`
}

// increment on breaking changes
func (BalanceChanged) SchemaVersion() int { return 1 }

func main() {
	if err := run(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := vulkan.LifecycleContext(nil)
	defer stop()

	pool, err := vulkan.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	if err != nil {
		return err
	}
	defer pool.Close()

	client, err := vulkan.NewClient(ctx, pool, nil)
	if err != nil {
		return err
	}
	registered, err := client.Topic[BalanceChanged]("accounts.balance").Register(ctx, nil)
	if err != nil {
		return err
	}
	balances, err := client.Topic[BalanceChanged](registered.Name).Producer().Register(ctx, nil)
	if err != nil {
		return err
	}
	ledger, err := client.Topic[BalanceChanged](registered.Name).Consumer("ledger").Register(ctx, &vulkan.ConsumerConfig{ConcurrencyOverride: vulkan.ConcurrencyOrdered})
	if err != nil {
		return err
	}

	for _, account := range []string{"acct-1", "acct-2"} {
		for _, delta := range []int64{100, -30, 55} {
			if _, err := balances.Produce(ctx, &BalanceChanged{AccountId: account, Delta: delta}, &vulkan.ProduceOptions{MessageKey: account}); err != nil {
				return err
			}
		}
	}

	return ledger.Consume(ctx, applyBalanceChange, &vulkan.ConsumeOptions{MessageConcurrency: 8})
}

// applyBalanceChange fails acct-1's -30 once; its +55 waits for the retry.
func applyBalanceChange(ctx context.Context, change *BalanceChanged) error {
	meta, _ := vulkan.MetaFromContext(ctx)
	if change.AccountId == "acct-1" && change.Delta == -30 && meta.Attempts == 0 {
		return errors.New("ledger row locked")
	}
	fmt.Printf("%s %+d (message %d, attempt %d)\n", change.AccountId, change.Delta, meta.Id, meta.Attempts+1)
	return nil
}
