package main

// schema gate e2e test: a producer/consumer refuses to Register against a database
// whose schema requires a newer binary -- fail fast, with a message an
// operator can act on, instead of running against a shape it can't. The gate
// allows min_compatible_version <= build <= current: a database migrated PAST
// the binary by additive steps stays usable (the rolling-deploy window); a
// step declaring MinCompatibleVersion above the build locks it out.
//
// Proves:
//  1. Register succeeds at the supported schema (v1).
//  2. additive skew: schema ahead of the binary with no breaking step ->
//     Register still succeeds.
//  3. a breaking step past the binary (system scope) refuses Register
//     (upgrade the binary).
//  4. a breaking step past ONE stream refuses that stream only -- a sibling
//     stream still registers (per-stream skew).

import (
	"context"
	"errors"
	"fmt"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"os"
	"strings"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/migrate"
	migratecontroller "github.com/agentstax/sqlstreams/pkg/migrate/controller"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/jackc/pgx/v5/pgxpool"
)

type event struct{ V int }

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

// testFailure is what die panics with; run recovers it into its error so
// main's deferred cleanup runs on a failed assertion.
type testFailure struct {
	message string
}

func (f testFailure) Error() string {
	return f.message
}

func run() (err error) {
	defer func() {
		switch recovered := recover().(type) {
		case nil:
		case testFailure:
			err = recovered
		default:
			panic(recovered)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	must(err)
	ds, err := datastore.NewPostgresDatastore(ctx, pool, nil)
	must(err)

	name := fmt.Sprintf("schemagate.e2e.%d", time.Now().UnixNano())
	siblingName := name + ".sibling"
	streamRow, err := client.Stream[event](name).Register(ctx, nil)
	must(err)
	_, err = client.Stream[event](siblingName).Register(ctx, nil)
	must(err)

	controller, err := migratecontroller.NewController(ds, ds.Logger)
	must(err)
	sysOwner, err := controller.SystemOwner(ctx)
	must(err)
	defer func() {
		must(client.Stream[event](name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
		must(client.Stream[event](siblingName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	// 1. supported schema -> Register succeeds -----------------------------------
	section("producer Register succeeds at the supported schema (v1)")
	_, err = client.Stream[event](name).Producer().Register(ctx, nil)
	check(err == nil, "Register accepted at v1")

	// 2. additive skew: schema ahead, nothing breaking -> Register succeeds ------
	section("system schema ahead by an additive step -> Register still succeeds")
	bump(ctx, pool, ds.Schema, sysOwner, 2, 0)
	_, err = client.Stream[event](name).Producer().Register(ctx, nil)
	check(err == nil, "Register accepted at v2 with no breaking step -- the rolling-deploy window")
	unbump(ctx, pool, ds.Schema, sysOwner, 2)

	// 3. breaking step past the binary (system) -> Register refused --------------
	section("system schema ahead by a breaking step -> Register refused")
	bump(ctx, pool, ds.Schema, sysOwner, 2, 2)
	_, err = client.Stream[event](name).Producer().Register(ctx, nil)
	show(err)
	check(errors.Is(err, migrate.ErrSchemaNewerThanBuild) && strings.Contains(err.Error(), "kind system, version 2") && strings.Contains(err.Error(), "min_compatible_version 2") && strings.Contains(err.Error(), "upgrade the binary"),
		"refused, naming the system version, the requirement, and the fix")
	unbump(ctx, pool, ds.Schema, sysOwner, 2)

	// 4. breaking step past ONE stream -> that stream refused, sibling accepted ----
	section("breaking step past one stream -> that stream refused, sibling accepted")
	streamOwner := mustOwner(common.NewStreamOwner(streamRow.SystemId, streamRow.Id, streamRow.Name))
	bump(ctx, pool, ds.Schema, streamOwner, 2, 2)
	_, err = client.Stream[event](name).Producer().Register(ctx, nil)
	show(err)
	check(errors.Is(err, migrate.ErrSchemaNewerThanBuild) && strings.Contains(err.Error(), "kind stream, version 2") && strings.Contains(err.Error(), "min_compatible_version 2"),
		"refused, naming the stream version and the requirement")
	_, err = client.Stream[event](siblingName).Producer().Register(ctx, nil)
	check(err == nil, "sibling stream still registers -- each family gates on its own rows")
	unbump(ctx, pool, ds.Schema, streamOwner, 2)

	fmt.Println("\n✅ SCHEMA GATE E2E TEST PASSED")
	fmt.Println("   Register rides out additive skew and fails fast, legibly, on a breaking step past the build.")
	return nil
}

// bump records a success at ver, so the gate reads that scope as version ver
// without any matching schema change -- a database a newer binary migrated.
// minCompatibleVersion 0 forges an additive step, ver forges a breaking one.
func bump(ctx context.Context, pool *pgxpool.Pool, schema string, owner *common.Owner, ver int64, minCompatibleVersion int64) {
	columns := datastore.NewOwnerColumns(*owner)

	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.migration_log (system_id, stream_id, consumer_group_id, version, min_compatible_version, status) VALUES ($1, $2, $3, $4, $5, 'success');`, schema), columns.SystemId, columns.StreamId, columns.ConsumerGroupId, ver, minCompatibleVersion)
	must(err)
}

func unbump(ctx context.Context, pool *pgxpool.Pool, schema string, owner *common.Owner, ver int64) {
	columns := datastore.NewOwnerColumns(*owner)

	_, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.migration_log WHERE system_id IS NOT DISTINCT FROM $1 AND stream_id IS NOT DISTINCT FROM $2 AND consumer_group_id IS NOT DISTINCT FROM $3 AND version = $4;`, schema), columns.SystemId, columns.StreamId, columns.ConsumerGroupId, ver)
	must(err)
}

func section(title string) { fmt.Printf("\n--- %s ---\n", title) }
func show(err error)       { fmt.Printf("  error: %v\n", err) }

func check(cond bool, msg string) {
	if !cond {
		fmt.Printf("  ✗ %s\n", msg)
		os.Exit(1)
	}
	fmt.Printf("  ✓ %s\n", msg)
}

func must(err error) {
	if err != nil {
		die(err.Error())
	}
}

func die(msg string) {
	panic(testFailure{message: msg})
}

func mustOwner(o *common.Owner, err error) *common.Owner { must(err); return o }

func (event) SchemaVersion() int { return 1 }
