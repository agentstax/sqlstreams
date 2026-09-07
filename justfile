set dotenv-required := true

### VERIFY ###

# Build, vet, and race-test every Go module, including the conventions checks.
verify:
    go build ./... && go vet ./... && go test -race ./...
    cd cmd/vulkan && go build ./... && go vet ./...
    cd otelvulkan && go build ./... && go vet ./...
    cd examples && go build ./...
    cd bench && go build ./...
    cd tools && go test -race -count=1 ./...

# Check a release's pinned public API against the working schema.
compat-lab expect="round-trip":
    cd tools/compat && go run . -expect={{ expect }}

# Run a reliability-lab scenario on its own compose stack: producer and
# consumer roles against a fresh Postgres, stopping when the producer's
# phases end. The checker and verdict are not built yet.
reliability-lab scenario="dev" time_scale="1":
    cd bench/reliability && SCENARIO={{ scenario }} TIME_SCALE={{ time_scale }} docker compose up --build --exit-code-from producer
    cd bench/reliability && docker compose down -v

### DATABASE ###

# Start the development PostgreSQL database in the foreground.
database-up:
    docker-compose -f scripts/database/docker-compose.yaml up

# Stop the development PostgreSQL database without deleting its volume.
database-down:
    docker-compose -f scripts/database/docker-compose.yaml down

# Stop the development database and delete all of its data.
database-delete:
    docker-compose -f scripts/database/docker-compose.yaml down -v

# Register the system in the development database. Safe to run repeatedly.
system-register:
    go run examples/phase_1/systemregister/main.go

### SCHEMA ###

# Generate a gitignored ER diagram from a registered development database.
schema-diagram:
    tbls doc -c scripts/database/tbls.yml --force
    tbls out -c scripts/database/tbls.yml -t json -o bin/schema/schema.json
    npx --yes @liam-hq/cli erd build --format tbls --input bin/schema/schema.json
    rm -rf bin/schema/erd && mv dist bin/schema/erd
    @echo "open with: just schema-diagram-serve"

# Serve the generated ER diagram at http://localhost:8377.
schema-diagram-serve:
    python3 -m http.server 8377 -d bin/schema/erd

# Recreate the development database, register the system, then generate its ER diagram.
schema-diagram-fresh:
    docker-compose -f scripts/database/docker-compose.yaml down -v
    docker-compose -f scripts/database/docker-compose.yaml up -d --wait postgres
    just system-register
    just schema-diagram

### EXAMPLES ###

# Run the phase-1 consumer example. EX: just consume learning.v1 0.1 1.0 0.0 -1
consume group="learning.v1" processorsleep="0.1" shutdownsleep="1.0" failrate="0.0" crashafter="-1":
    go run examples/phase_1/consumer/main.go -group={{ group }} -processor-sleep={{ processorsleep }} -shutdown-sleep={{ shutdownsleep }} -fail-rate={{ failrate }} -crash-after={{ crashafter }}

# Produce messages with the phase-1 producer example. EX: just produce 3
produce count="1":
    go run examples/phase_1/producer/main.go -count={{ count }}

### LABS: BUILD ###

# Build a lab binary in bin/. EX: just build-lab reclaimlab
build-lab lab:
    go build -o bin/{{ lab }} examples/phase_1/{{ lab }}/main.go

### LABS: CONSUMERS, DECLARATIONS, AND WORKERS ###

# Verify recovery after a consumer crashes while processing a message range.
reclaim-lab:
    go run examples/phase_1/reclaimlab/main.go

# Verify a failing message holds the committed cursor until it resolves.
exception-lab:
    go run examples/phase_1/exceptionlab/main.go

# Verify consumer-group declaration defaults, validation, and replacement.
group-config-lab:
    go run examples/phase_1/groupconfiglab/main.go

# Verify declaration outcomes reported by consumer registration.
outcome-lab:
    go run examples/phase_1/outcomelab/main.go

# Verify ordered delivery and its key lease behavior.
ordered-lab:
    go run examples/phase_1/orderedlab/main.go

# Verify bindings choose which messages a group receives.
routing-lab:
    go run examples/phase_1/routinglab/main.go

# Verify same-set joins, divergent-set waits, and replacement after a fleet exits.
binding-lab:
    go run examples/phase_1/bindinglab/main.go

# Verify consumer routines abandoned during a snapshot are recorded correctly.
abandoned-routine-snapshot-lab:
    go run examples/phase_1/abandonedroutinesnapshotlab/main.go

# Verify expired messages and abandoned routines are reported by maintenance work.
abandoned-events-lab:
    go run examples/phase_1/abandonedeventslab/main.go

# Verify maintenance-worker polling backs off when no work is available.
duty-backoff-lab:
    go run examples/phase_1/dutybackofflab/main.go

# Verify a produce-only deployment warns, and a live consumer resolves that alert.
worker-liveness-lab:
    go run examples/phase_1/workerlivenesslab/main.go

# Verify maintenance-worker claims, failover, and final release across consumers.
worker-claim-lab:
    go run examples/phase_1/workerclaimlab/main.go

# Verify Consume shares one system-manager instance and its claim across sessions.
manager-autorun-lab:
    go run examples/phase_1/managerautorunlab/main.go

# Verify graceful shutdown narrows a lease to the unprocessed message suffix.
shutdown-truncation-lab:
    go run examples/phase_1/shutdowntruncationlab/main.go

# Measure lazy versus synchronous advancement of a consumer group's committed cursor.
rollup-lab:
    go run examples/phase_1/rolluplab/main.go

# Verify exclusive consumer-group behavior.
exclusive-lab:
    go run examples/phase_1/exclusivelab/main.go

# Verify per-key leases prevent concurrent ordered delivery.
key-lease-lab:
    go run examples/phase_1/keyleaselab/main.go

### LABS: TOPICS, RETENTION, AND SCHEMA ###

# Verify partitions prune claim reads to the relevant message-id range.
partition-lab:
    go run examples/phase_1/partitionlab/main.go

# Verify a lagging cursor passes a dropped partition without stalling.
drop-floor-lab:
    go run examples/phase_1/dropfloorlab/main.go

# Verify retention sweeps an expired prefix that cannot justify a partition drop.
sweep-lab:
    go run examples/phase_1/sweeplab/main.go

# Verify per-topic tables, cursors, routing, and retention are isolated by topic.
topic-lab:
    go run examples/phase_1/topiclab/main.go

# Verify users cannot alter the system's reserved topics.
reserved-topic-lab:
    go run examples/phase_1/reservedtopiclab/main.go

# Verify topic registration is idempotent and rejects a conflicting configuration.
register-idempotency-lab:
    go run examples/phase_1/registeridempotencylab/main.go

# Verify topic destruction clears every topic-scoped control-plane and message row.
delete-topic-lab:
    go run examples/phase_1/deletetopiclab/main.go

# Verify system destruction refuses unsafe states and leaves a fresh registration possible.
destroy-system-lab:
    go run examples/phase_1/destroysystemlab/main.go

# Verify independent installations can share one database through separate schemas.
schema-lab:
    go run examples/phase_1/schemalab/main.go

# Verify producers and consumers reject database versions this build cannot support.
schema-gate-lab:
    go run examples/phase_1/schemagatelab/main.go

# Verify the migration registry is reversible, idempotent, and matches fresh creation.
invariant-lab:
    go run examples/phase_1/invariantlab/main.go

# Verify a user-space bridge moves compacted winners from one message schema to another.
schema-evolution-lab:
    go run examples/phase_1/schemaevolutionlab/main.go

### LABS: PRODUCERS AND DELIVERY RECORDS ###

# Verify idempotency keys deduplicate retries and the janitor removes expired claims.
idempotency-keys-lab:
    go run examples/phase_1/idempotencykeyslab/main.go

# Measure idempotency-key storage growth and verify its steady-state cleanup bound.
idempotency-keys-growth-lab:
    go run examples/phase_1/idempotencykeysgrowthlab/main.go

# Verify concurrent calls sharing one idempotency key produce exactly one message.
idempotency-keys-race-lab:
    go run examples/phase_1/idempotencykeysracelab/main.go

# Verify batched production shares transactions without cross-caller failure or deadlock.
producer-batch-lab:
    go run examples/phase_1/producerbatchlab/main.go

# Verify every production path creates the next partition before the boundary.
create-ahead-lab:
    go run examples/phase_1/createaheadlab/main.go

# Verify two in-transaction targets commit or roll back together.
multi-target-lab:
    go run examples/phase_1/multitargetlab/main.go

# Verify failures append delivery records, respecting opt-out and retention.
delivery-log-lab:
    go run examples/phase_1/deliveryloglab/main.go

### LABS: COMPACTION ###

# Verify a compacted topic delivers only its latest eligible message per key.
compaction-lab:
    go run examples/phase_1/compactionlab/main.go

# Verify compaction ranks keep pinned or bridge messages from being superseded.
compaction-rank-lab:
    go run examples/phase_1/compactionranklab/main.go

# Measure the partition-scan cost of finding the latest message for a key.
compaction-width-lab:
    go run examples/phase_1/compactionwidthlab/main.go

# Measure how latest-message lookup cost grows with compacted-topic history.
compaction-scale-lab:
    go run examples/phase_1/compactionscalelab/main.go

# Verify concurrent production converges compaction heads to the highest message id.
compaction-head-race-lab:
    go run examples/phase_1/compactionheadracelab/main.go

# Verify lockable compaction-head rows serialize first writes and race safely with cleanup.
compaction-head-lock-lab:
    go run -race examples/phase_1/compactionheadlocklab/main.go

# Verify retention removes a compaction head only after its key has no surviving message.
compaction-head-retention-lab:
    go run examples/phase_1/compactionheadretentionlab/main.go

# Measure compaction-head write cost, hot-key contention, and dead-tuple growth.
compaction-head-write-lab:
    go run examples/phase_1/compactionheadwritelab/main.go

# Verify batched production avoids hot-key deadlocks while caller transactions surface them.
compaction-deadlock-lab:
    go run examples/phase_1/compactiondeadlocklab/main.go

### LABS: METRICS, ALERTS, AND SCHEDULES ###

# Verify stored metric reads and the CLI's metrics surface.
metrics-lab:
    go run examples/phase_1/metricslab/main.go

# Verify concurrent metric collection and an HTTP scrape from a manager process.
metrics-collector-lab:
    cd cmd/vulkan && go build -o ../../bin/vulkan .
    go run -race examples/phase_1/metricscollectorlab/main.go

# Verify built-in alert thresholds classify, refresh, change severity, and resolve.
alert-lab:
    go run examples/phase_1/alertlab/main.go

# Verify cron validation, schedule lifecycle, production, and consumer delivery.
schedule-lab:
    go run examples/phase_1/schedulelab/main.go

# Verify concurrent Schedule calls run only one system-manager reconciliation loop.
schedule-concurrency-lab:
    go run examples/phase_1/scheduleconcurrencylab/main.go

### INSPECT ###

# List the messages stored for one topic. EX: just peek 1
peek topic_id:
    psql "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" \
      -c "SELECT * FROM message_log_{{ topic_id }} ORDER BY id;"

# List rows in the example users table.
peek-users:
    psql "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" \
      -c "SELECT * FROM users ORDER BY id;"

# List each group cursor and its distance from a topic's message-log head. EX: just lag 1
lag topic_id:
    psql "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" \
      -c "SELECT g.name AS consumer_group, c.claimed, COALESCE((SELECT max(id) FROM message_log_{{ topic_id }}), 0) AS head, COALESCE((SELECT max(id) FROM message_log_{{ topic_id }}), 0) - c.claimed AS lag FROM consumer_group_cursor_{{ topic_id }} c JOIN consumer_group_config g ON g.id = c.consumer_group_id ORDER BY lag DESC;"

### DOC SITE (https://vulkan-5ss.pages.dev) ###

# Start the documentation site in development mode.
site-dev:
    cd website && npm run dev

# Regenerate site data, require it to be committed, then run every site check.
site-verify:
    just site-compat
    git diff --exit-code --stat website/src/data/compat.json
    just site-codes
    git diff --exit-code --stat website/src/data/codes.json
    cd website && npm run verify

# Regenerate the migration compatibility matrix rendered by the documentation site.
site-compat:
    cd tools && go run ./compatexport -out ../website/src/data/compat.json

# Regenerate the documentation site's Vulkan error-code records.
site-codes:
    cd tools && go run ./codeexport -out ../website/src/data/codes.json

# Build and serve the documentation site, including its Pagefind index.
site-preview:
    cd website && npm run build && npm run preview

# Start the documentation site's component explorer at http://localhost:6006.
site-storybook:
    cd website && ./node_modules/.bin/storybook dev -p 6006

# Build and deploy the documentation site to its main branch.
site-deploy:
    cd website && npm run build && ./node_modules/.bin/wrangler pages deploy dist --project-name vulkan --branch main

# Freeze a release site at a permanent version alias; aliases never change.
site-freeze slug:
    cd website && npm run build && ./node_modules/.bin/wrangler pages deploy dist --project-name vulkan --branch {{ slug }}
