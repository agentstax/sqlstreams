package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"uuid"

	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	streamDomain "github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Scratch-only process: fixed producer concurrency, counters in memory, no per-message files.
var completed atomic.Int64
var rejected atomic.Int64
var duplicates atomic.Int64
var histogram [10001]atomic.Int64
var seen []atomic.Uint64

type Message struct {
	Sequence int64  `json:"sequence"`
	Started  int64  `json:"started"`
	Padding  string `json:"padding"`
}

func (Message) SchemaVersion() int { return 1 }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	role := flag.String("role", "setup", "setup, producer, consumer")
	startFile := flag.String("start-file", "", "wait for this file before connecting")
	host := flag.String("host", "127.0.0.1", "PostgreSQL host or Unix socket directory")
	sequenceStart := flag.Int64("sequence-start", 0, "exclusive lower bound for producer identities")
	seenFile := flag.String("seen-file", "", "write consumer identity bitset at shutdown")
	database := flag.String("database", "scratch", "dedicated database")
	callers := flag.Int("callers", 64, "concurrent Produce calls")
	batch := flag.Int("batch", 1000, "maximum automatic batch size")
	transactions := flag.Int("transactions", 1, "maximum concurrent batch transactions")
	claim := flag.Int("claim", 4000, "consumer claim size")
	queue := flag.Int("queue", 16000, "consumer queue capacity")
	handlers := flag.Int("handlers", 4, "concurrent handlers")
	poll := flag.Duration("poll", 20*time.Millisecond, "idle claim poll")
	duration := flag.Duration("duration", 20*time.Second, "production duration")
	maximum := flag.Int64("maximum", 2000000, "maximum produced messages; consumer identity bound")
	rate := flag.Int64("rate", 0, "messages per second per producer; zero unpaced, explicit batches only")
	rateSchedule := flag.String("rate-schedule", "", "explicit batch rate changes as elapsed:messages/s, starting at 0s")
	minimumConnections := flag.Int("minimum-connections", 0, "minimum pool connections")
	threadLimit := flag.Int("max-threads", 10000, "Go OS-thread safety limit")
	connections := flag.Int("connections", 32, "pool connections")
	partitionSize := flag.Int64("partition-size", 5000000, "messages per stream partition")
	retentionTTL := flag.Duration("retention-ttl", 0, "scratch message retention; zero keeps messages")
	idempotencyTTL := flag.Duration("idempotency-ttl", 24*time.Hour, "scratch duplicate-prevention window")
	queryMode := flag.String("query-mode", "cache_statement", "pgx default query execution mode")
	statementCache := flag.Int("statement-cache", 512, "prepared statement cache capacity")
	planCacheMode := flag.String("plan-cache-mode", "auto", "PostgreSQL prepared plan selection")
	rawSQL := flag.String("raw-sql", "", "production SQL template for direct pgx control")
	streamId := flag.Int64("stream-id", 0, "registered stream id for direct pgx control")
	profile := flag.String("profile", "", "optional CPU profile file")
	contention := flag.Bool("contention", false, "sample mutex and goroutine blocking profiles")
	explicit := flag.Bool("explicit-batch", false, "use ProduceBatch with callers concurrent batch calls")
	flag.Parse()
	if *rate < 0 || (*rate > 0 && !*explicit && *role == "producer") || *minimumConnections < 0 || *minimumConnections > *connections || *threadLimit < 100 {
		return fmt.Errorf("invalid rate, pool bounds, or thread safety limit")
	}
	debug.SetMaxThreads(*threadLimit)
	if *callers < 1 || *maximum < 1 || *maximum > 40000000 || *duration <= 0 {
		return fmt.Errorf("callers, duration and maximum must be positive; maximum must be <= 40000000")
	}
	var rateStarts []time.Duration
	var rateValues []int64
	if *rateSchedule != "" {
		if *role != "producer" || !*explicit || *rate != 0 {
			return fmt.Errorf("rate-schedule requires an explicit producer and rate=0")
		}
		for _, entry := range strings.Split(*rateSchedule, ",") {
			parts := strings.Split(entry, ":")
			if len(parts) != 2 {
				return fmt.Errorf("invalid rate-schedule entry %q", entry)
			}
			start, err := time.ParseDuration(parts[0])
			if err != nil || start < 0 || start >= *duration || (len(rateStarts) == 0 && start != 0) || (len(rateStarts) > 0 && start <= rateStarts[len(rateStarts)-1]) {
				return fmt.Errorf("invalid rate-schedule start %q", parts[0])
			}
			value, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil || value <= 0 || value > 1000000 {
				return fmt.Errorf("invalid rate-schedule rate %q", parts[1])
			}
			rateStarts = append(rateStarts, start)
			rateValues = append(rateValues, value)
		}
	}
	ctx, stop := sqlstreams.LifecycleContext(nil)
	defer stop()
	if *startFile != "" {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			if _, err := os.Stat(*startFile); err == nil {
				break
			} else if !os.IsNotExist(err) {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
		}
	}
	address := url.URL{Scheme: "postgres", Path: "/" + *database, RawQuery: url.Values{"host": {*host}, "port": {"55439"}, "user": {"scratch"}, "default_query_exec_mode": {*queryMode}, "statement_cache_capacity": {strconv.Itoa(*statementCache)}}.Encode()}
	poolConfig, err := pgxpool.ParseConfig(address.String())
	if err != nil {
		return err
	}
	poolConfig.MaxConns = int32(*connections)
	poolConfig.MinConns = int32(*minimumConnections)
	poolConfig.ConnConfig.RuntimeParams["plan_cache_mode"] = *planCacheMode
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return err
	}
	defer pool.Close()
	var effectivePlanCacheMode string
	if err := pool.QueryRow(ctx, "SHOW plan_cache_mode").Scan(&effectivePlanCacheMode); err != nil {
		return err
	}
	emit(map[string]any{"kind": "session_config", "role": *role, "plan_cache_mode": effectivePlanCacheMode})
	emit(map[string]any{"kind": "pool_config", "role": *role, "query_mode": pool.Config().ConnConfig.DefaultQueryExecMode.String(), "statement_cache": pool.Config().ConnConfig.StatementCacheCapacity, "max_connections": pool.Config().MaxConns, "min_connections": pool.Config().MinConns, "max_conn_lifetime": pool.Config().MaxConnLifetime.String(), "max_conn_idle_time": pool.Config().MaxConnIdleTime.String(), "health_check_period": pool.Config().HealthCheckPeriod.String(), "gomaxprocs": runtime.GOMAXPROCS(0), "max_threads": *threadLimit, "gogc": os.Getenv("GOGC"), "gomemlimit": os.Getenv("GOMEMLIMIT"), "host": pool.Config().ConnConfig.Host})
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{Logger: logger})
	if err != nil {
		return err
	}
	stream := client.Stream[Message]("orders")
	consumer := stream.Consumer("processor")
	if *role == "setup" {
		if err = client.System().Register(ctx, nil); err != nil {
			return err
		}
		if _, err = stream.Register(ctx, &sqlstreams.StreamConfig{PartitionSize: *partitionSize, RetentionTTL: *retentionTTL, IdempotencyKeyTTL: *idempotencyTTL, DeliveryLogMode: sqlstreams.DeliveryLogModeFailures}); err != nil {
			return err
		}
		_, err = consumer.Register(ctx, nil)
		return err
	}
	if *profile != "" {
		file, err := os.Create(*profile)
		if err != nil {
			return err
		}
		defer file.Close()
		if err = pprof.StartCPUProfile(file); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
		if *contention {
			runtime.SetMutexProfileFraction(100)
			runtime.SetBlockProfileRate(1000000)
		}
		defer func() {
			for _, name := range []string{"allocs", "heap", "mutex", "block"} {
				file, err := os.Create(*profile + "." + name)
				if err != nil {
					panic(err)
				}
				if err = pprof.Lookup(name).WriteTo(file, 0); err != nil {
					panic(err)
				}
				file.Close()
			}
		}()
	}
	var memory runtime.MemStats
	start := time.Now()
	monitorCtx, cancelMonitor := context.WithCancel(ctx)
	var monitor sync.WaitGroup
	monitor.Add(1)
	go func() {
		defer monitor.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		previous := int64(0)
		ticks := 0
		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				count := completed.Load()
				ticks++
				if ticks%10 != 0 {
					emit(map[string]any{"kind": "progress", "role": *role, "at": time.Now().UTC(), "completed": count})
					continue
				}
				runtime.ReadMemStats(&memory)
				stats := pool.Stat()
				emit(map[string]any{"kind": "sample", "role": *role, "at": time.Now().UTC(), "elapsed_s": time.Since(start).Seconds(), "completed": count, "per_interval": count - previous, "errors": rejected.Load(), "duplicates": duplicates.Load(), "heap_bytes": memory.HeapAlloc, "gc_cycles": memory.NumGC, "gc_pause_ns": memory.PauseTotalNs, "goroutines": runtime.NumGoroutine(), "pool_acquired": stats.AcquiredConns(), "pool_acquire_ns": stats.AcquireDuration().Nanoseconds(), "pool_empty_acquires": stats.EmptyAcquireCount(), "pool_wait_ns": stats.EmptyAcquireWaitTime().Nanoseconds(), "pool_acquires": stats.AcquireCount(), "pool_total": stats.TotalConns(), "allocated_bytes": memory.TotalAlloc, "gc_cpu_fraction": memory.GCCPUFraction, "p99_ms_upper_bound": percentile()})
				previous = count
			}
		}
	}()
	defer func() {
		cancelMonitor()
		monitor.Wait()
		emit(map[string]any{"kind": "final", "role": *role, "at": time.Now().UTC(), "completed": completed.Load(), "errors": rejected.Load(), "duplicates": duplicates.Load(), "elapsed_s": time.Since(start).Seconds(), "p99_ms_upper_bound": percentile()})
	}()
	switch *role {
	case "consumer":
		seen = make([]atomic.Uint64, (*maximum+64)/64)
		if *seenFile != "" {
			defer func() {
				data := make([]byte, len(seen)*8)
				for i := range seen {
					binary.LittleEndian.PutUint64(data[i*8:], seen[i].Load())
				}
				if err := os.WriteFile(*seenFile, data, 0600); err != nil {
					panic(err)
				}
			}()
		}
		options := (&sqlstreams.ConsumeOptions{BatchLimit: *claim, QueueSize: *queue, MessageConcurrency: *handlers, ClaimPollRate: *poll}).WithDefaults()
		if err = options.Validate(); err != nil {
			return err
		}
		emit(map[string]any{"kind": "config", "role": *role, "options": options, "pool_max": pool.Config().MaxConns, "postgres_host": pool.Config().ConnConfig.Host, "gomaxprocs": runtime.GOMAXPROCS(0)})
		instance, err := consumer.Register(ctx, nil)
		if err != nil {
			return err
		}
		return instance.Consume(ctx, handle, options)
	case "producer":
		cfg := (&sqlstreams.ProducerConfig{}).WithDefaults()
		cfg.Batch.MaxSize = *batch
		cfg.Batch.ConcurrencyLimit = *transactions
		if err = cfg.Validate(); err != nil {
			return err
		}
		instance, err := stream.Producer().Register(ctx, cfg)
		if err != nil {
			return err
		}
		var statement string
		if *rawSQL != "" {
			if !*explicit || *streamId < 1 {
				return fmt.Errorf("raw-sql requires explicit-batch and a positive stream-id")
			}
			template, err := os.ReadFile(*rawSQL)
			if err != nil {
				return err
			}
			statement = fmt.Sprintf(string(template), "sqlstreams", streamDomain.IdempotencyKeyTable(*streamId), streamDomain.MessageLogTable(*streamId))
			emit(map[string]any{"kind": "raw_sql", "statement": statement})
		}
		emit(map[string]any{"kind": "config", "role": *role, "config": cfg, "pool_max": pool.Config().MaxConns, "postgres_host": pool.Config().ConnConfig.Host, "callers": *callers, "explicit_batch": *explicit, "target_messages_s": *rate, "application_name": pool.Config().ConnConfig.RuntimeParams["application_name"], "gomaxprocs": runtime.GOMAXPROCS(0)})
		var sequence atomic.Int64
		sequence.Store(*sequenceStart)
		var routines sync.WaitGroup
		productionStart := time.Now()
		deadline := productionStart.Add(*duration)
		var admission sync.Mutex
		nextAdmission := productionStart
		ratePosition := -1
		pace := func(messages int) bool {
			if *rate == 0 && len(rateValues) == 0 {
				return time.Now().Before(deadline) && ctx.Err() == nil
			}
			admission.Lock()
			due := nextAdmission
			if now := time.Now(); now.After(due) {
				due = now
			}
			currentRate := *rate
			if len(rateValues) > 0 {
				position := 0
				for position+1 < len(rateStarts) && due.Sub(productionStart) >= rateStarts[position+1] {
					position++
				}
				currentRate = rateValues[position]
				if position != ratePosition {
					emit(map[string]any{"kind": "rate_change", "role": *role, "target_messages_s": currentRate, "scheduled_elapsed_s": rateStarts[position].Seconds(), "admission_elapsed_s": due.Sub(productionStart).Seconds(), "production_start": productionStart})
					ratePosition = position
				}
			}
			nextAdmission = due.Add(time.Duration(int64(messages) * int64(time.Second) / currentRate))
			admission.Unlock()
			if !due.Before(deadline) {
				return false
			}
			timer := time.NewTimer(time.Until(due))
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return false
			case <-timer.C:
				return time.Now().Before(deadline)
			}
		}
		// The timestamp is 19 digits; sequence padding keeps every encoded payload exactly 1000 bytes.
		padding := strings.Repeat("x", 1000)
		prototype := Message{Sequence: 1, Started: time.Now().UnixNano(), Padding: ""}
		encoded, err := json.Marshal(prototype)
		if err != nil {
			return err
		}
		paddingSize := 1000 - len(encoded)
		emit(map[string]any{"kind": "payload", "encoded_bytes": len(encoded) + paddingSize})
		if *explicit {
			for range *callers {
				routines.Add(1)
				go func() {
					defer routines.Done()
					for time.Now().Before(deadline) && ctx.Err() == nil && rejected.Load() == 0 {
						if !pace(*batch) {
							return
						}
						items := make([]*sqlstreams.ProduceItem[Message], 0, *batch)
						for range *batch {
							number := sequence.Add(1)
							if number > *maximum {
								break
							}
							message := &Message{Sequence: number, Started: time.Now().UnixNano(), Padding: padding[:paddingSize+1-len(strconv.FormatInt(number, 10))]}
							item, err := sqlstreams.NewProduceItem(message, nil)
							if err != nil {
								panic(err)
							}
							items = append(items, item)
						}
						if len(items) == 0 {
							return
						}
						var err error
						if statement != "" {
							err = produceRaw(ctx, pool, statement, items)
						} else {
							_, err = instance.ProduceBatch(ctx, items...)
						}
						if err != nil {
							rejected.Add(1)
							fmt.Fprintln(os.Stderr, err)
							return
						}
						finished := time.Now()
						for _, item := range items {
							histogram[min(10000, finished.Sub(time.Unix(0, item.Message.Started)).Milliseconds())].Add(1)
						}
						completed.Add(int64(len(items)))
					}
				}()
			}
			routines.Wait()
			if rejected.Load() != 0 {
				return fmt.Errorf("produce errors: %d", rejected.Load())
			}
			return nil
		}
		for range *callers {
			routines.Add(1)
			go func() {
				defer routines.Done()
				for time.Now().Before(deadline) && ctx.Err() == nil && rejected.Load() == 0 {
					number := sequence.Add(1)
					if number > *maximum {
						return
					}
					message := Message{Sequence: number, Started: time.Now().UnixNano(), Padding: padding[:paddingSize+1-len(strconv.FormatInt(number, 10))]}
					_, err := instance.Produce(ctx, &message, nil)
					if err != nil {
						rejected.Add(1)
						fmt.Fprintln(os.Stderr, err)
						return
					}
					histogram[min(10000, time.Since(time.Unix(0, message.Started)).Milliseconds())].Add(1)
					completed.Add(1)
				}
			}()
		}
		routines.Wait()
		if rejected.Load() != 0 {
			return fmt.Errorf("produce errors: %d", rejected.Load())
		}
		return nil
	default:
		return fmt.Errorf("unrecognized role %q", *role)
	}
}

func produceRaw(ctx context.Context, pool *pgxpool.Pool, statement string, items []*sqlstreams.ProduceItem[Message]) error {
	keys := make([]uuid.UUID, len(items))
	for i := range keys {
		keys[i] = uuid.NewV7()
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	batch := &pgx.Batch{}
	for i, item := range items {
		encoded, err := json.Marshal(item.Message)
		if err != nil {
			return err
		}
		batch.Queue(statement, keys[i], json.RawMessage(encoded), item.Options.RoutingKey, int64(item.Message.SchemaVersion()), item.Options.MessageKey, item.Options.Message)
	}
	results := tx.SendBatch(ctx, batch)
	for range items {
		var id int64
		if err := results.QueryRow().Scan(&id); err != nil {
			results.Close()
			return err
		}
	}
	if err := results.Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func handle(ctx context.Context, message *Message) error {
	word := message.Sequence / 64
	mask := uint64(1) << uint(message.Sequence%64)
	if message.Sequence < 1 || word >= int64(len(seen)) {
		rejected.Add(1)
		return fmt.Errorf("sequence outside scratch identity bound")
	}
	if seen[word].Or(mask)&mask != 0 {
		duplicates.Add(1)
	}
	histogram[min(10000, time.Since(time.Unix(0, message.Started)).Milliseconds())].Add(1)
	completed.Add(1)
	return nil
}
func percentile() int {
	var count int64
	for i := range histogram {
		count += histogram[i].Load()
	}
	target := (count*99 + 99) / 100
	var cumulative int64
	for i := range histogram {
		cumulative += histogram[i].Load()
		if cumulative >= target {
			if i == len(histogram)-1 {
				return -1
			}
			return i + 1
		}
	}
	return 10001
}
func emit(value any) {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		panic(err)
	}
}
