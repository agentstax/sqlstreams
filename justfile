set dotenv-required := true

### VERIFY ###

# Build, vet, and race-test every Go module, including the conventions checks.
verify:
    go build ./... && go vet ./... && go test -race -count=1 -shuffle=on ./...
    cd cmd/sqlstreams && go build ./... && go vet ./... && go test -race -count=1 -shuffle=on ./...
    cd otel && go build ./... && go vet ./... && go test -race -count=1 -shuffle=on ./...
    cd .tests && go build ./... && go vet ./... && go test -race -count=1 ./integration/...
    cd examples && go build ./...
    cd .bench && go build $(go list -e ./... | grep -v /results/) && go vet $(go list -e ./... | grep -v /results/) && go test -race -count=1 $(go list -e ./... | grep -v /results/)
    cd .tools && go test -race -count=1 ./...

# Run the integration tests in .tests/integration/ -- needs Docker, or SQLSTREAMS_TEST_DATABASE_URL naming a disposable server.
test-integration:
    cd .tests && go test -race -count=1 ./integration/...

# Check a release's pinned public API against the working schema.
compat-lab expect="round-trip":
    cd .tools/compat && go run . -expect={{ expect }}

# Run a scenario through Manager: Compose owns a fresh stack per repetition; native uses one supplied empty database.
# Exit with the worst verdict: 0 pass, 1 fail, 2 unknown, 3 lab failure.
# drain_budget bounds how long the checker waits for the consumers to catch up; a saturating scenario needs more than the default.
# replicas is the number of consumer processes, each running the scenario's instance count on every group.
# sync sets synchronous_commit on the lab database for the run; off is a labelled diagnostic cell, never the headline.
bench scenario time_scale="1" drain_budget="2m" reps="1" replicas="1" sync="on" execution="compose":
    #!/usr/bin/env bash
    set -euo pipefail
    cd .bench
    go build -o bench .
    exec ./bench -role manager -scenario {{ scenario }} -time-scale {{ time_scale }} -drain-budget {{ drain_budget }} -reps {{ reps }} -replicas {{ replicas }} -sync {{ sync }} -execution {{ execution }}

# Run one minute of the quiet scenario as a smoke check.
bench-smoke: (bench "quiet" "0.016666666666666666")

# Summarize a scenario's recorded runs from .bench/results/<scenario>/runs.jsonl: medians per environment identity.
bench-report scenario="quiet":
    cd .bench && go run . -role report -scenario {{ scenario }}

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
    go run ./.tests/e2e/systemregister/main.go

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

# Produce messages with the end-to-end producer. EX: just produce 3
produce count="1":
    go run ./.tests/e2e/producer/main.go -count={{ count }}

### E2E TESTS ###

# Build an e2e test binary in .bin/. EX: just build-e2e signal
build-e2e test:
    go build -o .bin/{{ test }} ./.tests/e2e/{{ test }}

# Signal cases: a killed producer, a producer and a consumer under SIGTERM, a second SIGTERM past a hung handler.
signal-e2e:
    go build -o .bin/ ./.tests/e2e/signal/...
    go run ./.tests/e2e/signal

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

### DOC SITE (https://sqlstreams.io) ###

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
    cd .website && npm run build && ./node_modules/.bin/wrangler pages deploy dist --project-name sqlstreams --branch main

# Freeze a release site at a permanent version alias; aliases never change.
site-freeze slug:
    cd .website && npm run build && ./node_modules/.bin/wrangler pages deploy dist --project-name sqlstreams --branch {{ slug }}
