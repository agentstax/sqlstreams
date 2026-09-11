package main

// signal is the e2e test for what a process signal leaves behind. Each case
// runs this same binary as a child under a -role and signals it:
//
//   - a producer holding an open transaction under SIGKILL leaves no
//     prepared transaction, no ungranted lock, and no backend still in a
//     transaction; its uncommitted message rolls back
//   - a producer looping under LifecycleContext exits 0 on SIGTERM, and
//     every message it reported landed, none more
//   - an idle consumer exits 0 promptly on SIGTERM
//   - a consumer whose handler ignores its context force-exits on the second
//     SIGTERM with status 128 + the signal, long before the message timeout
//
// The children print one line per event on stdout; the parent waits for the
// line before it signals.

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

const (
	roleProducerHolding = "producer-holding"
	roleProducerLooping = "producer-looping"
	roleConsumerIdle    = "consumer-idle"
	roleConsumerHung    = "consumer-hung"

	// lineTimeout bounds every wait on a child's stdout line.
	lineTimeout = 30 * time.Second
	// exitTimeout bounds every wait on a child's exit.
	exitTimeout = 30 * time.Second
	// promptExit is how long a graceful exit may take.
	promptExit = 10 * time.Second
	// hungTimeout is the message timeout the hung consumer runs under --
	// far past promptExit, so a force exit inside promptExit is one that
	// did not wait it out.
	hungTimeout = 5 * time.Minute
)

var role = flag.String("role", "", "child role; empty runs the parent")
var streamName = flag.String("stream", "", "the stream a child works on")

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() (err error) {
	defer common.Recover(&err)
	switch *role {
	case "":
		return runParent()
	case roleProducerHolding:
		return runProducerHolding()
	case roleProducerLooping:
		return runProducerLooping()
	case roleConsumerIdle:
		return runConsumer(false)
	case roleConsumerHung:
		return runConsumer(true)
	}
	return fmt.Errorf("unrecognized role: %q", *role)
}

func runParent() error {
	ctx := context.Background()
	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	common.Must(err)
	defer pool.Close()
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	name := fmt.Sprintf("signal.e2e.%d", time.Now().UnixNano())
	registered, err := client.Stream[common.Work](name).Register(ctx, &sqlstreams.StreamConfig{})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[common.Work](name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()
	messages := ds.Schema + "." + stream.MessageLogTable(registered.Id)
	producer, err := client.Stream[common.Work](name).Producer().Register(ctx, nil)
	common.Must(err)

	// a producer holding an open transaction under SIGKILL
	holding := startChild(roleProducerHolding, name)
	holding.await("holding")
	common.Must(holding.command.Process.Signal(syscall.SIGKILL))
	holding.wait()
	common.Assert(holding.exitCode() == -1, "SIGKILL child exit code = %d, want signal death (-1)", holding.exitCode())
	awaitCount(ctx, ds, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND state LIKE 'idle in transaction%'", 0)
	common.Assert(scalar(ctx, ds, "SELECT count(*) FROM pg_prepared_xacts") == 0, "prepared transactions after SIGKILL = %d, want 0", scalar(ctx, ds, "SELECT count(*) FROM pg_prepared_xacts"))
	common.Assert(scalar(ctx, ds, "SELECT count(*) FROM pg_locks WHERE NOT granted") == 0, "ungranted locks after SIGKILL = %d, want 0", scalar(ctx, ds, "SELECT count(*) FROM pg_locks WHERE NOT granted"))
	common.Assert(scalar(ctx, ds, "SELECT count(*) FROM "+messages) == 0, "messages after the killed producer = %d, want its uncommitted message rolled back (0)", scalar(ctx, ds, "SELECT count(*) FROM "+messages))
	fmt.Println("✓ a killed producer leaves no transaction, lock, or message behind")

	// a producer looping under LifecycleContext under SIGTERM
	looping := startChild(roleProducerLooping, name)
	looping.await("producing")
	time.Sleep(300 * time.Millisecond)
	common.Must(looping.command.Process.Signal(syscall.SIGTERM))
	looping.wait()
	common.Assert(looping.exitCode() == 0, "SIGTERM producer exit code = %d, want 0", looping.exitCode())
	reported := looping.countLines("produced ")
	common.Assert(reported > 0, "SIGTERM producer reported %d messages, want some produced before the signal", reported)
	stored := scalar(ctx, ds, "SELECT count(*) FROM "+messages)
	common.Assert(stored == reported, "messages after the SIGTERM producer = %d, want the %d it reported", stored, reported)
	fmt.Printf("✓ a producer under SIGTERM exits 0 with all %d reported messages committed\n", reported)

	// an idle consumer under SIGTERM
	idle := startChild(roleConsumerIdle, name)
	idle.await("consuming")
	// the session claims its worker row and starts polling after the line
	time.Sleep(time.Second)
	signaled := time.Now()
	common.Must(idle.command.Process.Signal(syscall.SIGTERM))
	idle.wait()
	common.Assert(idle.exitCode() == 0, "SIGTERM idle consumer exit code = %d, want 0", idle.exitCode())
	common.Assert(time.Since(signaled) < promptExit, "SIGTERM idle consumer exited after %v, want under %v", time.Since(signaled), promptExit)
	fmt.Printf("✓ an idle consumer under SIGTERM exits 0 in %v\n", time.Since(signaled).Round(time.Millisecond))

	// a hung handler under a second SIGTERM
	hung := startChild(roleConsumerHung, name)
	hung.await("consuming")
	work, err := common.NewWork(30, "admin@example.com")
	common.Must(err)
	_, err = producer.Produce(ctx, work, nil)
	common.Must(err)
	hung.await("handler blocked")
	signaled = time.Now()
	common.Must(hung.command.Process.Signal(syscall.SIGTERM))
	time.Sleep(500 * time.Millisecond)
	common.Must(hung.command.Process.Signal(syscall.SIGTERM))
	hung.wait()
	common.Assert(hung.exitCode() == 128+int(syscall.SIGTERM), "second SIGTERM exit code = %d, want %d", hung.exitCode(), 128+int(syscall.SIGTERM))
	common.Assert(time.Since(signaled) < promptExit, "second SIGTERM exited after %v, want under %v (the handler timeout is %v)", time.Since(signaled), promptExit, hungTimeout)
	fmt.Printf("✓ a second SIGTERM past a hung handler force-exits with status %d in %v\n", hung.exitCode(), time.Since(signaled).Round(time.Millisecond))
	return nil
}

// runProducerHolding produces one message inside a transaction it never
// ends, and waits to be killed.
func runProducerHolding() error {
	ctx := context.Background()
	client, producer := childClient(ctx)
	return client.InTransaction(ctx, func(ctx context.Context, tx sqlstreams.Tx) error {
		work, err := common.NewWork(30, "admin@example.com")
		if err != nil {
			return err
		}
		if _, err := producer.ProduceInTx(ctx, tx, work, nil); err != nil {
			return err
		}
		fmt.Println("holding")
		select {}
	})
}

// runProducerLooping produces until the lifecycle context ends. Each produce
// runs under its own context: cancelling a produce's ctx stops the wait,
// not the message, so a message is reported only once its call returned.
func runProducerLooping() error {
	ctx, stop := sqlstreams.LifecycleContext(nil)
	defer stop()
	_, producer := childClient(ctx)
	fmt.Println("producing")
	for ctx.Err() == nil {
		work, err := common.NewWork(30, "admin@example.com")
		if err != nil {
			return err
		}
		produced, err := producer.Produce(context.Background(), work, nil)
		if err != nil {
			return err
		}
		fmt.Printf("produced %d\n", produced.Id)
	}
	return nil
}

// runConsumer consumes under the lifecycle context; a hung consumer's
// handler blocks forever, ignoring its context.
func runConsumer(hung bool) error {
	ctx, stop := sqlstreams.LifecycleContext(nil)
	defer stop()
	client, _ := childClient(ctx)
	cfg := &sqlstreams.ConsumerConfig{}
	if hung {
		cfg.Message = &sqlstreams.MessageOptions{Timeout: hungTimeout}
	}
	instance, err := client.Stream[common.Work](*streamName).Consumer("signal.e2e").Register(ctx, cfg)
	if err != nil {
		return err
	}
	fmt.Println("consuming")
	err = instance.Consume(ctx, func(ctx context.Context, work *common.Work) error {
		if hung {
			fmt.Println("handler blocked")
			select {}
		}
		return nil
	}, &sqlstreams.ConsumeOptions{ClaimPollRate: 200 * time.Millisecond})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// ***************
// *** HELPERS ***
// ***************

// child is one signaled process with its stdout lines.
type child struct {
	command *exec.Cmd
	lines   chan string
	seen    []string
	exit    error
}

// startChild runs this binary under role on the stream, reading its stdout
// line by line.
func startChild(role string, name string) *child {
	command := exec.Command(os.Args[0], "-role", role, "-stream", name)
	command.Stderr = os.Stderr
	stdout, err := command.StdoutPipe()
	common.Must(err)
	common.Must(command.Start())
	lines := make(chan string, 1024)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	return &child{command: command, lines: lines}
}

// await reads lines until one starts with prefix, within lineTimeout.
func (c *child) await(prefix string) {
	deadline := time.After(lineTimeout)
	for {
		select {
		case line, ok := <-c.lines:
			common.Assert(ok, "%s exited before printing %q", c.command.Args[2], prefix)
			c.seen = append(c.seen, line)
			if strings.HasPrefix(line, prefix) {
				return
			}
		case <-deadline:
			common.Die("%s did not print %q within %v", c.command.Args[2], prefix, lineTimeout)
		}
	}
}

// wait drains the remaining lines and waits for the exit, within exitTimeout.
func (c *child) wait() {
	deadline := time.After(exitTimeout)
	for {
		select {
		case line, ok := <-c.lines:
			if !ok {
				c.exit = c.command.Wait()
				return
			}
			c.seen = append(c.seen, line)
		case <-deadline:
			_ = c.command.Process.Kill()
			common.Die("%s did not exit within %v", c.command.Args[2], exitTimeout)
		}
	}
}

// exitCode is the child's exit status: -1 when a signal ended it.
func (c *child) exitCode() int {
	var exitErr *exec.ExitError
	if errors.As(c.exit, &exitErr) {
		return exitErr.ExitCode()
	}
	if c.exit != nil {
		common.Die("%s wait error = %v", c.command.Args[2], c.exit)
	}
	return 0
}

// countLines counts the lines the child printed with the prefix.
func (c *child) countLines(prefix string) int64 {
	var count int64
	for _, line := range c.seen {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}

// childClient is a child's client and producer over the parent's stream.
func childClient(ctx context.Context) (*sqlstreams.Client, *sqlstreams.ProducerInstance[common.Work]) {
	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	common.Must(err)
	client, err := sqlstreams.NewClient(ctx, pool, nil)
	common.Must(err)
	producer, err := client.Stream[common.Work](*streamName).Producer().Register(ctx, nil)
	common.Must(err)
	return client, producer
}

// scalar reads one count.
func scalar(ctx context.Context, ds *iDatastore.PostgresDatastore, sql string) int64 {
	var value int64
	common.Must(ds.Pool.QueryRow(ctx, sql).Scan(&value))
	return value
}

// awaitCount polls the count until it reads want, within exitTimeout --
// the server notices a killed client on its next socket read.
func awaitCount(ctx context.Context, ds *iDatastore.PostgresDatastore, sql string, want int64) {
	deadline := time.Now().Add(exitTimeout)
	for {
		got := scalar(ctx, ds, sql)
		if got == want {
			return
		}
		common.Assert(time.Now().Before(deadline), "%s = %d after %v, want %d", sql, got, exitTimeout, want)
		time.Sleep(100 * time.Millisecond)
	}
}
