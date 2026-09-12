package main

// signal is the e2e test for what a process signal leaves behind. Each case
// starts one of the child programs beside this file (built into .bin/ by
// `just signal-e2e`), waits for the line it prints when ready, signals it,
// and checks what it left behind:
//
//   - a producer holding an open transaction under SIGKILL leaves no
//     prepared transaction, no ungranted lock, and no backend still in a
//     transaction; its uncommitted message rolls back
//   - a producer looping under LifecycleContext exits 0 on SIGTERM, and
//     every message it reported landed, none more
//   - an idle consumer exits 0 promptly on SIGTERM
//   - a consumer whose handler ignores its context force-exits on the second
//     SIGTERM with status 128 + the signal, long before the message timeout

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	sqlstreams "github.com/agentstax/sqlstreams/client"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

const (
	// testTimeout bounds the whole run; a child still alive at the deadline
	// is killed, which ends any wait on its lines.
	testTimeout = 2 * time.Minute
	// promptExit is how long a graceful exit may take.
	promptExit = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	pool, err := common.NewPool(ctx, nil)
	if err != nil {
		return err
	}
	defer pool.Close()
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	if err != nil {
		return err
	}
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("signal.e2e.%d", time.Now().UnixNano())
	handle := client.Stream[common.Work](name)
	registered, err := handle.Register(ctx, &sqlstreams.StreamConfig{})
	if err != nil {
		return err
	}
	defer func() {
		if err := handle.Destroy(context.WithoutCancel(ctx), &sqlstreams.DestroyOptions{Force: true}); err != nil {
			fmt.Fprintln(os.Stderr, "stream destroy:", err)
		}
	}()
	messages := ds.Schema + "." + stream.MessageLogTable(registered.Id)
	producer, err := handle.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	// a killed producer leaves no transaction, lock, or message behind
	holding, holdingLines, err := startChild(ctx, "holdingproducer", name)
	if err != nil {
		return err
	}
	if err := awaitLine(holdingLines, "holding"); err != nil {
		return err
	}
	if err := holding.Process.Signal(syscall.SIGKILL); err != nil {
		return err
	}
	holdingExit, err := exitCode(holding.Wait())
	if err != nil {
		return err
	}
	if err := awaitCount(ctx, ds, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND state LIKE 'idle in transaction%'", 0); err != nil {
		return err
	}
	prepared, err := scalar(ctx, ds, "SELECT count(*) FROM pg_prepared_xacts")
	if err != nil {
		return err
	}
	ungranted, err := scalar(ctx, ds, "SELECT count(*) FROM pg_locks WHERE NOT granted")
	if err != nil {
		return err
	}
	stored, err := scalar(ctx, ds, "SELECT count(*) FROM "+messages)
	if err != nil {
		return err
	}
	if holdingExit != -1 {
		return fmt.Errorf("SIGKILL child exit code = %d, want signal death (-1)", holdingExit)
	}
	if prepared != 0 {
		return fmt.Errorf("prepared transactions after SIGKILL = %d, want 0", prepared)
	}
	if ungranted != 0 {
		return fmt.Errorf("ungranted locks after SIGKILL = %d, want 0", ungranted)
	}
	if stored != 0 {
		return fmt.Errorf("messages after the killed producer = %d, want its uncommitted message rolled back (0)", stored)
	}
	fmt.Println("✓ a killed producer leaves no transaction, lock, or message behind")

	// a producer under SIGTERM exits 0 with every message it reported committed
	looping, loopingLines, err := startChild(ctx, "loopingproducer", name)
	if err != nil {
		return err
	}
	if err := awaitLine(loopingLines, "produced "); err != nil {
		return err
	}
	if err := looping.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	reported := int64(1)
	for loopingLines.Scan() {
		if strings.HasPrefix(loopingLines.Text(), "produced ") {
			reported++
		}
	}
	loopingExit, err := exitCode(looping.Wait())
	if err != nil {
		return err
	}
	stored, err = scalar(ctx, ds, "SELECT count(*) FROM "+messages)
	if err != nil {
		return err
	}
	if loopingExit != 0 {
		return fmt.Errorf("SIGTERM producer exit code = %d, want 0", loopingExit)
	}
	if stored != reported {
		return fmt.Errorf("messages after the SIGTERM producer = %d, want the %d it reported", stored, reported)
	}
	fmt.Printf("✓ a producer under SIGTERM exits 0 with all %d reported messages committed\n", reported)

	// an idle consumer under SIGTERM exits 0 promptly
	idle, idleLines, err := startChild(ctx, "idleconsumer", name)
	if err != nil {
		return err
	}
	if err := awaitLine(idleLines, "consuming"); err != nil {
		return err
	}
	// the session claims its worker row and starts polling after the line
	time.Sleep(time.Second)
	idleSignaled := time.Now()
	if err := idle.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	idleExit, err := exitCode(idle.Wait())
	if err != nil {
		return err
	}
	idleElapsed := time.Since(idleSignaled)
	if idleExit != 0 {
		return fmt.Errorf("SIGTERM idle consumer exit code = %d, want 0", idleExit)
	}
	if idleElapsed >= promptExit {
		return fmt.Errorf("SIGTERM idle consumer exited after %v, want under %v", idleElapsed, promptExit)
	}
	fmt.Printf("✓ an idle consumer under SIGTERM exits 0 in %v\n", idleElapsed.Round(time.Millisecond))

	// a second SIGTERM past a hung handler force-exits with 128 + the signal
	hung, hungLines, err := startChild(ctx, "hungconsumer", name)
	if err != nil {
		return err
	}
	if err := awaitLine(hungLines, "consuming"); err != nil {
		return err
	}
	work, err := common.NewWork(30, "admin@example.com")
	if err != nil {
		return err
	}
	if _, err := producer.Produce(ctx, work, nil); err != nil {
		return err
	}
	if err := awaitLine(hungLines, "handler blocked"); err != nil {
		return err
	}
	hungSignaled := time.Now()
	if err := hung.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	// back-to-back signals coalesce; the second lands after the first was handled
	time.Sleep(500 * time.Millisecond)
	if err := hung.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	hungExit, err := exitCode(hung.Wait())
	if err != nil {
		return err
	}
	hungElapsed := time.Since(hungSignaled)
	forced := 128 + int(syscall.SIGTERM)
	if hungExit != forced {
		return fmt.Errorf("second SIGTERM exit code = %d, want %d", hungExit, forced)
	}
	if hungElapsed >= promptExit {
		return fmt.Errorf("second SIGTERM exited after %v, want under %v (the handler timeout is 5m)", hungElapsed, promptExit)
	}
	fmt.Printf("✓ a second SIGTERM past a hung handler force-exits with status %d in %v\n", hungExit, hungElapsed.Round(time.Millisecond))
	return nil
}

// ***************
// *** HELPERS ***
// ***************

// startChild starts the built child program on the stream and returns it
// with a scanner over its stdout lines. The ctx deadline kills it.
func startChild(ctx context.Context, program string, streamName string) (*exec.Cmd, *bufio.Scanner, error) {
	command := exec.CommandContext(ctx, ".bin/"+program, "-stream", streamName)
	command.Stderr = os.Stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := command.Start(); err != nil {
		return nil, nil, err
	}
	return command, bufio.NewScanner(stdout), nil
}

// awaitLine reads lines until one starts with prefix; a child that exits
// first is the error.
func awaitLine(lines *bufio.Scanner, prefix string) error {
	for lines.Scan() {
		if strings.HasPrefix(lines.Text(), prefix) {
			return nil
		}
	}
	return fmt.Errorf("child exited before printing %q", prefix)
}

// exitCode is a child's exit status from its Wait error: -1 when a signal
// ended it, 0 when it exited clean.
func exitCode(exit error) (int, error) {
	var exitErr *exec.ExitError
	if errors.As(exit, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	if exit != nil {
		return 0, exit
	}
	return 0, nil
}

// scalar reads one count.
func scalar(ctx context.Context, ds *iDatastore.PostgresDatastore, sql string) (int64, error) {
	var value int64
	err := ds.Pool.QueryRow(ctx, sql).Scan(&value)
	return value, err
}

// awaitCount polls the count until it reads want -- the server notices a
// killed client on its next socket read. The ctx deadline bounds the poll.
func awaitCount(ctx context.Context, ds *iDatastore.PostgresDatastore, sql string, want int64) error {
	for {
		got, err := scalar(ctx, ds, sql)
		if err != nil {
			return err
		}
		if got == want {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("%s = %d at the test deadline, want %d", sql, got, want)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
