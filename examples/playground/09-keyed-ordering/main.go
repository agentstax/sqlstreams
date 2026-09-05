// Scenario 09 -- keyed ordering: what a same-key consumer actually sees.
//
// Account balance updates for one account must apply in order and never
// overlap. The producer keys by account; the consumer runs concurrently.
//
// Concepts held before domain code (9): the produce set from scenario 01,
// plus MessageKey, ProduceOptions.Message, MessageOptions.Concurrency
// (ConcurrencyOrdered), the session's ConsumeOptions.MessageConcurrency,
// and the "ordered = every same-key message in id order, one at a time,
// through failures" semantics.
//
// Traps hit:
//   - A message key alone orders nothing: MessageConcurrency > 1 delivers
//     two same-key messages at once unless Concurrency is exclusive or
//     ordered -- and that is a per-MESSAGE option the producer sets, not a
//     topic or group property (ConcurrencyOverride on the consumer is the
//     group-wide form).
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
	ledger, err := client.Topic[BalanceChanged](registered.Name).Consumer("ledger").Register(ctx, nil)
	if err != nil {
		return err
	}

	for _, account := range []string{"acct-1", "acct-2"} {
		for _, delta := range []int64{100, -30, 55} {
			if _, err := balances.Produce(ctx, &BalanceChanged{AccountId: account, Delta: delta}, orderedByAccount(account)); err != nil {
				return err
			}
		}
	}

	return ledger.Consume(ctx, applyBalanceChange, &vulkan.ConsumeOptions{MessageConcurrency: 8})
}

// orderedByAccount keys the message by account and runs same-account
// deliveries one at a time in id order.
func orderedByAccount(account string) *vulkan.ProduceOptions {
	return &vulkan.ProduceOptions{
		MessageKey: account,
		Message:    &vulkan.MessageOptions{Concurrency: vulkan.ConcurrencyOrdered},
	}
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
