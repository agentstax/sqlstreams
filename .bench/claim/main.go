package main

// measures the cursor-claim hot path statement by statement against a live
// dev database, then a prototype of the one-batch shape parked in ROADMAP.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	messageconsumerdatastore "github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller/datastore"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/producer"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	seedRows      = 20000
	partitionSize = 500
	batchLimit    = 100
	iterations    = 300
)

var outDir = os.Getenv("CLAIMBENCH_OUT")

func main() {
	if err := run(); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
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

	streamName := fmt.Sprintf("claimbench.%d", time.Now().UnixNano())
	tp, err := client.Stream[common.Work](streamName).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: partitionSize})
	if err != nil {
		return err
	}
	defer func() {
		if err := client.Stream[common.Work](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}); err != nil {
			fmt.Println("destroy:", err)
		}
	}()

	producerInstance, err := client.Stream[common.Work](streamName).Producer().Register(ctx, nil)
	if err != nil {
		return err
	}
	start := time.Now()
	for produced := 0; produced < seedRows; produced += 50 {
		items := make([]*producer.ProduceItem[common.Work], 0, 50)
		for range 50 {
			work, _ := common.NewWork(30, "bench@example.com")
			item, err := producer.NewProduceItem(work, nil)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		if _, err := producerInstance.ProduceBatch(ctx, items...); err != nil {
			return err
		}
	}
	fmt.Printf("seeded %d rows in %s (partition size %d)\n", seedRows, time.Since(start).Round(time.Millisecond), partitionSize)

	consumeController, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	if err != nil {
		return err
	}
	group, err := consumeController.RegisterGroup(ctx, tp.Id, "claimbench", consume.Beginning())
	if err != nil {
		return err
	}
	groupId := group.Id
	consumers, err := messageconsumerdatastore.NewMessageConsumerGroupDatastore(ds, ds.Logger)
	if err != nil {
		return err
	}

	schema := ds.Schema
	messageLog := stream.MessageLogTable(tp.Id)
	cursorTable := stream.ConsumerGroupCursorTable(tp.Id)
	leaseTable := stream.ClaimLeaseTable(tp.Id)
	bindingTable := stream.BindingConfigTable(tp.Id)
	compactionHead := stream.CompactionHeadTable(tp.Id)

	var partitions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_inherits WHERE inhparent = ($1)::regclass`, schema+"."+messageLog).Scan(&partitions); err != nil {
		return err
	}
	fmt.Printf("message_log partitions: %d\n\n", partitions)

	// ---------- statements, copied from the datastore ----------
	snapshotSql := fmt.Sprintf(`
		SELECT
			(SELECT COALESCE(MAX(id), 0) FROM %[1]s.%[2]s) AS head,
			pg_snapshot_xmax(pg_current_snapshot())::text AS xmax,
			c.claimed,
			c.settled_head,
			c.pending_head
		FROM %[1]s.%[3]s c
		WHERE c.consumer_group_id = $1;
	`, schema, messageLog, cursorTable)

	// variant: MAX(id) bounded below by the cursor's own settled_head so
	// partition pruning skips every partition wholly below it
	snapshotPrunedSql := fmt.Sprintf(`
		SELECT
			(SELECT COALESCE(MAX(id), c.settled_head) FROM %[1]s.%[2]s WHERE id > c.settled_head) AS head,
			pg_snapshot_xmax(pg_current_snapshot())::text AS xmax,
			c.claimed,
			c.settled_head,
			c.pending_head
		FROM %[1]s.%[3]s c
		WHERE c.consumer_group_id = $1;
	`, schema, messageLog, cursorTable)

	reclaimSql := fmt.Sprintf(`
		UPDATE %[1]s.%[2]s
		SET
			reclaims = reclaims + 1,
			expires_at = now() + make_interval(secs => $2),
			token = gen_random_uuid()
		WHERE (consumer_group_id, token) IN (
			SELECT consumer_group_id, token FROM %[1]s.%[3]s
			WHERE consumer_group_id = $1
				AND expires_at < now()
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING
			token,
			consumer_group_id,
			low,
			high,
			expires_at,
			reclaims;
	`, schema, leaseTable, leaseTable)

	cursorSql := fmt.Sprintf(`
		WITH old_values AS (
			SELECT claimed, settled_head, pending_head, pending_xid
			FROM %[1]s.%[2]s
			WHERE consumer_group_id = $1
			FOR UPDATE
		),
		gate AS (
			SELECT GREATEST(
				o.settled_head,
				CASE WHEN pg_snapshot_xmin(pg_current_snapshot()) >= $4::xid8 THEN $3 ELSE 0 END,
				CASE WHEN o.pending_xid IS NOT NULL AND pg_snapshot_xmin(pg_current_snapshot()) >= o.pending_xid THEN o.pending_head ELSE 0 END
			) AS head
			FROM old_values o
		),
		updated AS (
			UPDATE %[1]s.%[3]s c
			SET
				claimed = LEAST(c.claimed + $2, gate.head),
				settled_head = gate.head,
				pending_head = GREATEST(c.pending_head, $3),
				pending_xid = GREATEST(c.pending_xid, $4::xid8)
			FROM old_values, gate
			WHERE c.consumer_group_id = $1
			RETURNING old_values.claimed AS low, c.claimed AS high
		)
		SELECT u.low, u.high FROM updated u;
	`, schema, cursorTable, cursorTable)

	leaseSql := fmt.Sprintf(`
		INSERT INTO %[1]s.%[2]s (consumer_group_id, low, high, expires_at)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4))
		RETURNING token, consumer_group_id, low, high, expires_at, reclaims;
	`, schema, leaseTable)

	readSql := fmt.Sprintf(`
		SELECT
			m.id,
			m.payload,
			m.created_at,
			COALESCE(m.routing_key, '') AS routing_key,
			COALESCE(m.message_key, '') AS message_key,
			COALESCE(m.compaction_rank, 0) AS compaction_rank,
			(m.compaction_rank IS NOT NULL) AS compacted,
			m.options
		FROM %[1]s.%[2]s m
		WHERE m.id > $1
			AND m.id <= $2
			AND m.schema_version = $4
			AND (
				NOT EXISTS (SELECT 1 FROM %[1]s.%[3]s b WHERE b.consumer_group_id = $3)
				OR EXISTS (SELECT 1 FROM %[1]s.%[4]s b WHERE b.consumer_group_id = $3 AND m.routing_key ~ b.pattern_regex)
			)
			AND (
				m.compaction_rank IS NULL
				OR m.id = (SELECT message_id FROM %[1]s.%[5]s WHERE compaction_key = m.message_key AND message_id IS NOT NULL)
			)
		ORDER BY m.id;
	`, schema, messageLog, bindingTable, bindingTable, compactionHead)

	// prototype: reclaim attempt + snapshot in ONE statement, autocommit
	combinedSql := fmt.Sprintf(`
		WITH reclaimed AS (
			UPDATE %[1]s.%[2]s
			SET
				reclaims = reclaims + 1,
				expires_at = now() + make_interval(secs => $2),
				token = gen_random_uuid()
			WHERE (consumer_group_id, token) IN (
				SELECT consumer_group_id, token FROM %[1]s.%[3]s
				WHERE consumer_group_id = $1
					AND expires_at < now()
				LIMIT 1
				FOR UPDATE SKIP LOCKED
			)
			RETURNING token, low, high, expires_at, reclaims
		)
		SELECT
			(SELECT COALESCE(MAX(id), c.settled_head) FROM %[1]s.%[4]s WHERE id > c.settled_head) AS head,
			pg_snapshot_xmax(pg_current_snapshot())::text AS xmax,
			c.claimed,
			c.settled_head,
			c.pending_head,
			r.token,
			r.low,
			r.high,
			r.expires_at,
			r.reclaims
		FROM %[1]s.%[5]s c
		LEFT JOIN reclaimed r ON true
		WHERE c.consumer_group_id = $1;
	`, schema, leaseTable, leaseTable, messageLog, cursorTable)

	// ---------- floors ----------
	bench("SELECT 1 (round-trip floor)", iterations, func() error {
		var one int
		return pool.QueryRow(ctx, "SELECT 1").Scan(&one)
	})
	bench("BEGIN + ROLLBACK (two round trips)", iterations, func() error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		return tx.Rollback(ctx)
	})

	// ---------- the fresh-claim statements, one at a time inside a rolled-back tx ----------
	var snapshotHead, claimed, settledHead, pendingHead int64
	var snapshotXmax string
	inTx := func(name string, fn func(tx pgx.Tx) error) {
		bench(name, iterations, func() error {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return err
			}
			defer tx.Rollback(ctx)
			return timed(fn, tx)
		})
	}
	inTx("snapshot statement, MAX(id) over every partition", func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, snapshotSql, groupId).Scan(&snapshotHead, &snapshotXmax, &claimed, &settledHead, &pendingHead)
	})
	inTx("snapshot statement, MAX(id) WHERE id > settled_head (pruned)", func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, snapshotPrunedSql, groupId).Scan(&snapshotHead, &snapshotXmax, &claimed, &settledHead, &pendingHead)
	})
	inTx("reclaim UPDATE, no expired lease", func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, reclaimSql, groupId, 30.0)
		if err != nil {
			return err
		}
		_, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[messageconsumerdatastore.ClaimLeaseRow])
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	})
	inTx("cursor statement (old_values / gate / updated)", func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, cursorSql, groupId, batchLimit, snapshotHead, snapshotXmax)
		if err != nil {
			return err
		}
		_, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[messageconsumerdatastore.ConsumerGroupCursorRow])
		return err
	})
	inTx("lease INSERT", func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, leaseSql, groupId, 0, batchLimit, 30.0)
		if err != nil {
			return err
		}
		_, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[messageconsumerdatastore.ClaimLeaseRow])
		return err
	})
	inTx(fmt.Sprintf("readMessages, %d rows", batchLimit), func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, readSql, 0, batchLimit, groupId, 1)
		if err != nil {
			return err
		}
		_, err = pgx.CollectRows(rows, pgx.RowToStructByName[messageconsumerdatastore.MessageLogRow])
		return err
	})
	inTx(fmt.Sprintf("lease INSERT + readMessages pipelined (one round trip)"), func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		batch.Queue(leaseSql, groupId, 0, batchLimit, 30.0)
		batch.Queue(readSql, 0, batchLimit, groupId, 1)
		results := tx.SendBatch(ctx, batch)
		if _, err := results.Exec(); err != nil {
			results.Close()
			return err
		}
		rows, err := results.Query()
		if err != nil {
			results.Close()
			return err
		}
		if _, err := pgx.CollectRows(rows, pgx.RowToStructByName[messageconsumerdatastore.MessageLogRow]); err != nil {
			results.Close()
			return err
		}
		return results.Close()
	})

	// does a zero-row reclaim UPDATE assign a txid (which would taint the snapshot fence)?
	{
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		rows, _ := tx.Query(ctx, reclaimSql, groupId, 30.0)
		rows.Close()
		var assigned *string
		if err := tx.QueryRow(ctx, "SELECT pg_current_xact_id_if_assigned()::text").Scan(&assigned); err != nil {
			return err
		}
		tx.Rollback(ctx)
		fmt.Printf("txid assigned after a zero-row reclaim UPDATE: %v\n\n", assigned != nil)
	}

	bench("prototype: reclaim + snapshot as one autocommit statement", iterations, func() error {
		rows, err := pool.Query(ctx, combinedSql, groupId, 30.0)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
		}
		return rows.Err()
	})

	// ---------- the real path ----------
	var claims []time.Duration
	var commits []time.Duration
	var total int
	for {
		claimStart := time.Now()
		claimed, err := consumers.ClaimMessagesWithCursor(ctx, tp.Id, groupId, 1, batchLimit, 3, 30*time.Second, stream.DeliveryLogModeFailures)
		claims = append(claims, time.Since(claimStart))
		if err != nil {
			return err
		}
		if claimed == nil {
			claims = claims[:len(claims)-1]
			break
		}
		total += len(claimed.Messages)
		commitStart := time.Now()
		if err := consumers.Commit(ctx, tp.Id, groupId, claimed.Lease.Token, nil, 5*time.Second, stream.DeliveryLogModeFailures); err != nil {
			return err
		}
		commits = append(commits, time.Since(commitStart))
	}
	report(fmt.Sprintf("real ClaimMessagesWithCursor, backlog, limit %d (%d claims, %d messages)", batchLimit, len(claims), total), claims)
	report("real Commit, no outcomes", commits)
	bench("real ClaimMessagesWithCursor, caught up (idle poll)", iterations, func() error {
		claimed, err := consumers.ClaimMessagesWithCursor(ctx, tp.Id, groupId, 1, batchLimit, 3, 30*time.Second, stream.DeliveryLogModeFailures)
		if claimed != nil {
			return fmt.Errorf("expected caught up")
		}
		return err
	})

	// =====================================================================
	// prototype: 2 round trips per claim, 1 per idle poll
	//   1. snapshot + "is there an expired lease" flag, autocommit
	//   2. one pipelined batch (implicit transaction): fused cursor UPDATE +
	//      lease INSERT under a client-minted token, then the read bounded by
	//      that lease row
	// =====================================================================
	snapshotProtoSql := fmt.Sprintf(`
		SELECT
			(SELECT COALESCE(MAX(id), 0) FROM %[1]s.%[2]s) AS head,
			pg_snapshot_xmax(pg_current_snapshot())::text AS xmax,
			c.claimed,
			c.settled_head,
			c.pending_head,
			EXISTS (SELECT 1 FROM %[1]s.%[4]s l WHERE l.consumer_group_id = $1 AND l.expires_at < now()) AS reclaimable
		FROM %[1]s.%[3]s c
		WHERE c.consumer_group_id = $1;
	`, schema, messageLog, cursorTable, leaseTable)

	fusedSql := fmt.Sprintf(`
		WITH old_values AS (
			SELECT claimed, settled_head, pending_head, pending_xid
			FROM %[1]s.%[2]s
			WHERE consumer_group_id = $1
			FOR UPDATE
		),
		gate AS (
			SELECT GREATEST(
				o.settled_head,
				CASE WHEN pg_snapshot_xmin(pg_current_snapshot()) >= $4::xid8 THEN $3 ELSE 0 END,
				CASE WHEN o.pending_xid IS NOT NULL AND pg_snapshot_xmin(pg_current_snapshot()) >= o.pending_xid THEN o.pending_head ELSE 0 END
			) AS head
			FROM old_values o
		),
		updated AS (
			UPDATE %[1]s.%[2]s c
			SET
				claimed = LEAST(c.claimed + $2, gate.head),
				settled_head = gate.head,
				pending_head = GREATEST(c.pending_head, $3),
				pending_xid = GREATEST(c.pending_xid, $4::xid8)
			FROM old_values, gate
			WHERE c.consumer_group_id = $1
			RETURNING old_values.claimed AS low, c.claimed AS high
		),
		lease AS (
			INSERT INTO %[1]s.%[3]s (consumer_group_id, token, low, high, expires_at)
			SELECT $1, $6, u.low, u.high, now() + make_interval(secs => $5)
			FROM updated u
			WHERE u.low < u.high
			RETURNING token, low, high, expires_at, reclaims
		)
		SELECT u.low, u.high, (SELECT count(*) FROM lease) AS leased FROM updated u;
	`, schema, cursorTable, leaseTable)

	readByLeaseSql := fmt.Sprintf(`
		SELECT
			m.id,
			m.payload,
			m.created_at,
			COALESCE(m.routing_key, '') AS routing_key,
			COALESCE(m.message_key, '') AS message_key,
			COALESCE(m.compaction_rank, 0) AS compaction_rank,
			(m.compaction_rank IS NOT NULL) AS compacted,
			m.options
		FROM %[1]s.%[2]s m
		WHERE m.id > (SELECT low FROM %[1]s.%[6]s WHERE consumer_group_id = $2 AND token = $1)
			AND m.id <= (SELECT high FROM %[1]s.%[6]s WHERE consumer_group_id = $2 AND token = $1)
			AND m.schema_version = $3
			AND (
				NOT EXISTS (SELECT 1 FROM %[1]s.%[3]s b WHERE b.consumer_group_id = $2)
				OR EXISTS (SELECT 1 FROM %[1]s.%[4]s b WHERE b.consumer_group_id = $2 AND m.routing_key ~ b.pattern_regex)
			)
			AND (
				m.compaction_rank IS NULL
				OR m.id = (SELECT message_id FROM %[1]s.%[5]s WHERE compaction_key = m.message_key AND message_id IS NOT NULL)
			)
		ORDER BY m.id;
	`, schema, messageLog, bindingTable, bindingTable, compactionHead, leaseTable)

	// claimProto returns (nil, nil) when caught up; never handles the reclaim
	// branch (the bench never expires a lease)
	claimProto := func(ctx context.Context, groupId int64) ([]messageconsumerdatastore.MessageLogRow, error) {
		var head, claimed, settledHead, pendingHead int64
		var xmax string
		var reclaimable bool
		if err := pool.QueryRow(ctx, snapshotProtoSql, groupId).Scan(&head, &xmax, &claimed, &settledHead, &pendingHead, &reclaimable); err != nil {
			return nil, err
		}
		if reclaimable {
			return nil, fmt.Errorf("unexpected expired lease")
		}
		if head == pendingHead && pendingHead == settledHead && claimed == settledHead {
			return nil, nil
		}

		token := pgtype.UUID{Bytes: uuid.NewV7(), Valid: true}
		batch := &pgx.Batch{}
		batch.Queue(fusedSql, groupId, batchLimit, head, xmax, 30.0, token)
		batch.Queue(readByLeaseSql, token, groupId, 1)
		results := pool.SendBatch(ctx, batch)
		var low, high, leased int64
		if err := results.QueryRow().Scan(&low, &high, &leased); err != nil {
			results.Close()
			return nil, err
		}
		rows, err := results.Query()
		if err != nil {
			results.Close()
			return nil, err
		}
		messages, err := pgx.CollectRows(rows, pgx.RowToStructByName[messageconsumerdatastore.MessageLogRow])
		if err != nil {
			results.Close()
			return nil, err
		}
		if err := results.Close(); err != nil {
			return nil, err
		}
		if leased == 0 {
			return nil, nil
		}
		return messages, nil
	}

	newGroup := func(name string) (int64, error) {
		group, err := consumeController.RegisterGroup(ctx, tp.Id, name, consume.Beginning())
		if err != nil {
			return 0, err
		}
		return group.Id, nil
	}

	// single instance latency, prototype
	protoGroup, err := newGroup("proto")
	if err != nil {
		return err
	}
	var protoClaims []time.Duration
	protoTotal := 0
	for {
		claimStart := time.Now()
		messages, err := claimProto(ctx, protoGroup)
		if err != nil {
			return err
		}
		if messages == nil {
			break
		}
		protoClaims = append(protoClaims, time.Since(claimStart))
		protoTotal += len(messages)
	}
	report(fmt.Sprintf("prototype claim, backlog, limit %d (%d claims, %d messages)", batchLimit, len(protoClaims), protoTotal), protoClaims)
	bench("prototype claim, caught up (idle poll)", iterations, func() error {
		messages, err := claimProto(ctx, protoGroup)
		if messages != nil {
			return fmt.Errorf("expected caught up")
		}
		return err
	})

	// contention: G instances of one group draining the same backlog
	for _, instances := range []int{1, 4, 8} {
		realGroup, err := newGroup(fmt.Sprintf("real-%d", instances))
		if err != nil {
			return err
		}
		realStart := time.Now()
		var realCount atomic.Int64
		var wg sync.WaitGroup
		for range instances {
			wg.Go(func() {
				for {
					claimed, err := consumers.ClaimMessagesWithCursor(ctx, tp.Id, realGroup, 1, batchLimit, 3, 30*time.Second, stream.DeliveryLogModeFailures)
					if err != nil {
						fmt.Println("real claim:", err)
						return
					}
					if claimed == nil {
						return
					}
					realCount.Add(1)
				}
			})
		}
		wg.Wait()
		realElapsed := time.Since(realStart)

		protoGroup, err := newGroup(fmt.Sprintf("proto-%d", instances))
		if err != nil {
			return err
		}
		protoStart := time.Now()
		var protoCount atomic.Int64
		for range instances {
			wg.Go(func() {
				for {
					messages, err := claimProto(ctx, protoGroup)
					if err != nil {
						fmt.Println("proto claim:", err)
						return
					}
					if messages == nil {
						return
					}
					protoCount.Add(1)
				}
			})
		}
		wg.Wait()
		protoElapsed := time.Since(protoStart)
		fmt.Printf("\n%d instance(s) draining %d messages at limit %d:\n", instances, seedRows, batchLimit)
		fmt.Printf("  real:      %4d claims in %6s  = %6.0f claims/s\n", realCount.Load(), realElapsed.Round(time.Millisecond), float64(realCount.Load())/realElapsed.Seconds())
		fmt.Printf("  prototype: %4d claims in %6s  = %6.0f claims/s\n", protoCount.Load(), protoElapsed.Round(time.Millisecond), float64(protoCount.Load())/protoElapsed.Seconds())
	}

	// ---------- plans ----------
	if outDir != "" {
		explain := func(name string, sql string, args ...any) error {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return err
			}
			defer tx.Rollback(ctx)
			rows, err := tx.Query(ctx, "EXPLAIN (ANALYZE, BUFFERS, COSTS OFF) "+sql, args...)
			if err != nil {
				return err
			}
			lines, err := pgx.CollectRows(rows, pgx.RowTo[string])
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(outDir, name+".txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
		}
		if err := explain("snapshot", snapshotSql, groupId); err != nil {
			return err
		}
		if err := explain("snapshot_pruned", snapshotPrunedSql, groupId); err != nil {
			return err
		}
		if err := explain("cursor", cursorSql, groupId, batchLimit, snapshotHead, snapshotXmax); err != nil {
			return err
		}
		if err := explain("read", readSql, 0, batchLimit, groupId, 1); err != nil {
			return err
		}
		if err := explain("reclaim", reclaimSql, groupId, 30.0); err != nil {
			return err
		}
		if err := explain("combined", combinedSql, groupId, 30.0); err != nil {
			return err
		}
	}
	return nil
}

var lastTimed time.Duration

func timed(fn func(tx pgx.Tx) error, tx pgx.Tx) error {
	start := time.Now()
	err := fn(tx)
	lastTimed = time.Since(start)
	return err
}

// bench reports fn's own time when fn ran through timed (statement only,
// excluding begin/rollback), otherwise the whole call.
func bench(name string, n int, fn func() error) {
	samples := make([]time.Duration, 0, n)
	for range n {
		lastTimed = 0
		start := time.Now()
		if err := fn(); err != nil {
			fmt.Printf("%s: %v\n", name, err)
			return
		}
		elapsed := time.Since(start)
		if lastTimed != 0 {
			elapsed = lastTimed
		}
		samples = append(samples, elapsed)
	}
	report(name, samples)
}

func report(name string, samples []time.Duration) {
	if len(samples) == 0 {
		fmt.Printf("%-70s (no samples)\n", name)
		return
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	at := func(p float64) time.Duration { return samples[int(float64(len(samples)-1)*p)] }
	fmt.Printf("%-70s p50 %7dµs   p90 %7dµs   min %7dµs\n", name, at(0.5).Microseconds(), at(0.9).Microseconds(), samples[0].Microseconds())
}
