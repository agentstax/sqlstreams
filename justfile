set dotenv-required := true

### VERIFY ###

# Build, vet, and race-test every Go module, including the conventions checks.
verify:
    go build ./... && go vet ./... && go test -race -count=1 -shuffle=on ./...
    cd cmd/sqlstreams && go build ./... && go vet ./... && go test -race -count=1 -shuffle=on ./...
    cd otel && go build ./... && go vet ./... && go test -race -count=1 -shuffle=on ./...
    cd .e2e && go build ./...
    cd .examples && go build ./...
    cd .bench && go build ./... && go vet ./... && go test -race -count=1 ./reliability/...
    cd .tools && go test -race -count=1 ./...

# Check a release's pinned public API against the working schema.
compat-lab expect="round-trip":
    cd .tools/compat && go run . -expect={{ expect }}

# Run a reliability-lab scenario on its own compose stack, reps times from a fresh stack, and exit with the worst verdict: 0 pass, 1 fail, 2 unknown, 3 lab failure.
# drain_budget bounds how long the checker waits for the consumers to catch up; a saturating scenario needs more than the default.
# replicas is the number of consumer processes, each running the scenario's instance count on every group.
# sync sets synchronous_commit on the lab database for the run; off is a labelled diagnostic cell, never the headline.
reliability-lab scenario="dev" time_scale="1" drain_budget="2m" reps="1" replicas="1" sync="on":
    #!/usr/bin/env bash
    set -euo pipefail
    cd .bench/reliability
    export SCENARIO={{ scenario }} TIME_SCALE={{ time_scale }} DRAIN_BUDGET={{ drain_budget }}
    # the repo .env just loads names the dev database; the lab's stack is its own
    unset POSTGRES_HOST POSTGRES_PORT POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB
    stats_pid=""
    trap 'kill "$stats_pid" 2>/dev/null || true; docker compose down -v --remove-orphans' EXIT
    # a run killed mid-way can leave a one-off container holding the records volume
    docker compose down -v --remove-orphans
    docker compose --profile checker build
    worst=0
    for rep in $(seq 1 {{ reps }}); do
        echo "rep $rep of {{ reps }}"
        docker compose up --detach --scale consumer={{ replicas }} consumer observer
        docker compose exec -T postgres psql -U lab -d lab -q -c "ALTER DATABASE lab SET synchronous_commit = {{ sync }}"
        mkdir -p results && ./fingerprint.sh > results/fingerprint.json
        ./stats.sh > results/stats.jsonl &
        stats_pid=$!
        docker compose run --rm producer
        kill "$stats_pid" && wait "$stats_pid" || true
        code=0
        docker compose run --rm checker || code=$?
        docker compose down -v --remove-orphans
        # a lab failure outranks a fail, which outranks an unknown
        case "$code:$worst" in
            3:*) worst=3 ;;
            1:0|1:2) worst=1 ;;
            2:0) worst=2 ;;
        esac
    done
    exit "$worst"

# Summarize a scenario's recorded runs from .bench/reliability/results/<scenario>/runs.jsonl: medians per environment identity.
reliability-report scenario="dev":
    cd .bench/reliability && go run . -role report -scenario {{ scenario }}

### DATABASE ###

# Start the development PostgreSQL database in the foreground.
database-up:
    docker-compose -f .tools/database/docker-compose.yaml up

# Stop the development PostgreSQL database without deleting its volume.
database-down:
    docker-compose -f .tools/database/docker-compose.yaml down

# Stop the development database and delete all of its data.
database-delete:
    docker-compose -f .tools/database/docker-compose.yaml down -v

# Register the system in the development database. Safe to run repeatedly.
system-register:
    go run ./.e2e/systemregister/main.go

### SCHEMA ###

# Generate a gitignored ER diagram from a registered development database.
schema-diagram:
    tbls doc -c .tools/database/tbls.yml --force
    tbls out -c .tools/database/tbls.yml -t json -o .bin/schema/schema.json
    cd .bin/schema && npx --yes @liam-hq/cli erd build --format tbls --input schema.json
    rm -rf .bin/schema/erd && mv .bin/schema/dist .bin/schema/erd
    @echo "open with: just schema-diagram-serve"

# Serve the generated ER diagram at http://localhost:8377.
schema-diagram-serve:
    python3 -m http.server 8377 -d .bin/schema/erd

# Recreate the development database, register the system, then generate its ER diagram.
schema-diagram-fresh:
    docker-compose -f .tools/database/docker-compose.yaml down -v
    docker-compose -f .tools/database/docker-compose.yaml up -d --wait postgres
    just system-register
    just schema-diagram

### EXAMPLES ###

# Run the end-to-end consumer. EX: just consume learning.v1 0.1 1.0 0.0 -1
consume group="learning.v1" processorsleep="0.1" shutdownsleep="1.0" failrate="0.0" crashafter="-1":
    go run ./.e2e/consumer/main.go -group={{ group }} -processor-sleep={{ processorsleep }} -shutdown-sleep={{ shutdownsleep }} -fail-rate={{ failrate }} -crash-after={{ crashafter }}

# Produce messages with the end-to-end producer. EX: just produce 3
produce count="1":
    go run ./.e2e/producer/main.go -count={{ count }}

### E2E TESTS: BUILD ###

# Build an e2e test binary in .bin/. EX: just build-e2e reclaim
build-e2e test:
    go build -o .bin/{{ test }} ./.e2e/{{ test }}/main.go

### E2E TESTS: CONSUMERS, DECLARATIONS, AND WORKERS ###

# Verify recovery after a consumer crashes while processing a message range.
reclaim-e2e:
    go run ./.e2e/reclaim/main.go

# Verify a failing message holds the committed cursor until it resolves.
exception-e2e:
    go run ./.e2e/exception/main.go

# Verify consumer-group declaration defaults, validation, and replacement.
group-config-e2e:
    go run ./.e2e/groupconfig/main.go

# Verify declaration outcomes reported by consumer registration.
outcome-e2e:
    go run ./.e2e/outcome/main.go

# Verify ordered delivery and its key lease behavior.
ordered-e2e:
    go run ./.e2e/ordered/main.go

# Verify bindings choose which messages a group receives.
routing-e2e:
    go run ./.e2e/routing/main.go

# Verify same-set joins, divergent-set waits, and replacement after a fleet exits.
binding-e2e:
    go run ./.e2e/binding/main.go

# Verify consumer routines abandoned during a snapshot are recorded correctly.
abandoned-routine-snapshot-e2e:
    go run ./.e2e/abandonedroutinesnapshot/main.go

# Verify expired messages and abandoned routines are reported by maintenance work.
abandoned-events-e2e:
    go run ./.e2e/abandonedevents/main.go

# Verify maintenance-worker polling backs off when no work is available.
duty-backoff-e2e:
    go run ./.e2e/dutybackoff/main.go

# Verify a produce-only deployment warns, and a live consumer resolves that alert.
worker-liveness-e2e:
    go run ./.e2e/workerliveness/main.go

# Verify maintenance-worker claims, failover, and final release across consumers.
worker-claim-e2e:
    go run ./.e2e/workerclaim/main.go

# Verify Consume shares one system-manager instance and its claim across sessions.
manager-autorun-e2e:
    go run ./.e2e/managerautorun/main.go

# Verify graceful shutdown narrows a lease to the unprocessed message suffix.
shutdown-truncation-e2e:
    go run ./.e2e/shutdowntruncation/main.go

# Measure lazy versus synchronous advancement of a consumer group's committed cursor.
rollup-e2e:
    go run ./.e2e/rollup/main.go

# Verify exclusive consumer-group behavior.
exclusive-e2e:
    go run ./.e2e/exclusive/main.go

# Verify per-key leases prevent concurrent ordered delivery.
key-lease-e2e:
    go run ./.e2e/keylease/main.go

### E2E TESTS: STREAMS, RETENTION, AND SCHEMA ###

# Verify partitions prune claim reads to the relevant message-id range.
partition-e2e:
    go run ./.e2e/partition/main.go

# Verify a lagging cursor passes a dropped partition without stalling.
drop-floor-e2e:
    go run ./.e2e/dropfloor/main.go

# Verify retention sweeps an expired prefix that cannot justify a partition drop.
sweep-e2e:
    go run ./.e2e/sweep/main.go

# Verify per-stream tables, cursors, routing, and retention are isolated by stream.
stream-e2e:
    go run ./.e2e/stream/main.go

# Verify users cannot alter the system's reserved streams.
reserved-stream-e2e:
    go run ./.e2e/reservedstream/main.go

# Verify stream registration is idempotent and rejects a conflicting configuration.
register-idempotency-e2e:
    go run ./.e2e/registeridempotency/main.go

# Verify stream destruction clears every stream-scoped control-plane and message row.
delete-stream-e2e:
    go run ./.e2e/deletestream/main.go

# Verify system destruction refuses unsafe states and leaves a fresh registration possible.
destroy-system-e2e:
    go run ./.e2e/destroysystem/main.go

# Verify independent installations can share one database through separate schemas.
schema-e2e:
    go run ./.e2e/schema/main.go

# Verify producers and consumers reject database versions this build cannot support.
schema-gate-e2e:
    go run ./.e2e/schemagate/main.go

# Verify the migration registry is reversible, idempotent, and matches fresh creation.
invariant-e2e:
    go run ./.e2e/invariant/main.go

# Verify a user-space bridge moves compacted winners from one message schema to another.
schema-evolution-e2e:
    go run ./.e2e/schemaevolution/main.go

### E2E TESTS: PRODUCERS AND DELIVERY RECORDS ###

# Verify idempotency keys deduplicate retries and the janitor removes expired claims.
idempotency-keys-e2e:
    go run ./.e2e/idempotencykeys/main.go

# Measure idempotency-key storage growth and verify its steady-state cleanup bound.
idempotency-keys-growth-e2e:
    go run ./.e2e/idempotencykeysgrowth/main.go

# Verify concurrent calls sharing one idempotency key produce exactly one message.
idempotency-keys-race-e2e:
    go run ./.e2e/idempotencykeysrace/main.go

# Verify batched production shares transactions without cross-caller failure or deadlock.
producer-batch-e2e:
    go run ./.e2e/producerbatch/main.go

# Verify every production path creates the next partition before the boundary.
create-ahead-e2e:
    go run ./.e2e/createahead/main.go

# Verify two in-transaction targets commit or roll back together.
multi-target-e2e:
    go run ./.e2e/multitarget/main.go

# Verify failures append delivery records, respecting opt-out and retention.
delivery-log-e2e:
    go run ./.e2e/deliverylog/main.go

### E2E TESTS: COMPACTION ###

# Verify a compacted stream delivers only its latest eligible message per key.
compaction-e2e:
    go run ./.e2e/compaction/main.go

# Verify compaction ranks keep pinned or bridge messages from being superseded.
compaction-rank-e2e:
    go run ./.e2e/compactionrank/main.go

# Measure the partition-scan cost of finding the latest message for a key.
compaction-width-e2e:
    go run ./.e2e/compactionwidth/main.go

# Measure how latest-message lookup cost grows with compacted-stream history.
compaction-scale-e2e:
    go run ./.e2e/compactionscale/main.go

# Verify concurrent production converges compaction heads to the highest message id.
compaction-head-race-e2e:
    go run ./.e2e/compactionheadrace/main.go

# Verify lockable compaction-head rows serialize first writes and race safely with cleanup.
compaction-head-lock-e2e:
    go run -race ./.e2e/compactionheadlock/main.go

# Verify retention removes a compaction head only after its key has no surviving message.
compaction-head-retention-e2e:
    go run ./.e2e/compactionheadretention/main.go

# Measure compaction-head write cost, hot-key contention, and dead-tuple growth.
compaction-head-write-e2e:
    go run ./.e2e/compactionheadwrite/main.go

# Verify batched production avoids hot-key deadlocks while caller transactions surface them.
compaction-deadlock-e2e:
    go run ./.e2e/compactiondeadlock/main.go

### E2E TESTS: METRICS, ALERTS, AND SCHEDULES ###

# Verify stored metric reads and the CLI's metrics surface.
metrics-e2e:
    go run ./.e2e/metrics/main.go

# Verify concurrent metric collection and an HTTP scrape from a manager process.
metrics-collector-e2e:
    cd cmd/sqlstreams && go build -o ../../.bin/sqlstreams .
    go run -race ./.e2e/metricscollector/main.go

# Verify built-in alert thresholds classify, refresh, change severity, and resolve.
alert-e2e:
    go run ./.e2e/alert/main.go

# Verify cron validation, schedule lifecycle, production, and consumer delivery.
schedule-e2e:
    go run ./.e2e/schedule/main.go

# Verify concurrent Schedule calls run only one system-manager reconciliation loop.
schedule-concurrency-e2e:
    go run ./.e2e/scheduleconcurrency/main.go

### INSPECT ###

# List the messages stored for one stream. EX: just peek 1
peek stream_id:
    psql "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" \
      -c "SELECT * FROM message_log_{{ stream_id }} ORDER BY id;"

# List rows in the example users table.
peek-users:
    psql "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" \
      -c "SELECT * FROM users ORDER BY id;"

# List each group cursor and its distance from a stream's message-log head. EX: just lag 1
lag stream_id:
    psql "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" \
      -c "SELECT g.name AS consumer_group, c.claimed, COALESCE((SELECT max(id) FROM message_log_{{ stream_id }}), 0) AS head, COALESCE((SELECT max(id) FROM message_log_{{ stream_id }}), 0) - c.claimed AS lag FROM consumer_group_cursor_{{ stream_id }} c JOIN consumer_group_config g ON g.id = c.consumer_group_id ORDER BY lag DESC;"

### DOC SITE (https://vulkan-5ss.pages.dev) ###

# Start the documentation site in development mode.
site-dev:
    cd .website && npm run dev

# Regenerate site data, require it to be committed, then run every site check.
site-verify:
    just site-compat
    git diff --exit-code --stat .website/src/data/compat.json
    just site-codes
    git diff --exit-code --stat .website/src/data/codes.json
    cd .website && npm run verify

# Regenerate the migration compatibility matrix rendered by the documentation site.
site-compat:
    cd .tools && go run ./compatexport -out ../.website/src/data/compat.json

# Regenerate the documentation site's SQLStreams error-code records.
site-codes:
    cd .tools && go run ./codeexport -out ../.website/src/data/codes.json

# Build and serve the documentation site, including its Pagefind index.
site-preview:
    cd .website && npm run build && npm run preview

# Start the documentation site's component explorer at http://localhost:6006.
site-storybook:
    cd .website && ./node_modules/.bin/storybook dev -p 6006

# Build and deploy the documentation site to its main branch.
site-deploy:
    cd .website && npm run build && ./node_modules/.bin/wrangler pages deploy dist --project-name vulkan --branch main

# Freeze a release site at a permanent version alias; aliases never change.
site-freeze slug:
    cd .website && npm run build && ./node_modules/.bin/wrangler pages deploy dist --project-name vulkan --branch {{ slug }}
