module github.com/allegedlyreliable/sqlstreams/.tools/compat

go 1.27.0

// Dev-only and deliberately outside go.work. just compat-lab sets GOWORK=off
// so the pinned published release cannot resolve to working-tree source.
// At each release checkpoint, pin the prior SQLStreams tag and adapt this
// driver to that tag's client API. Do not add a working-tree replace.
//
// Prepare with the CURRENT build against a fresh development database:
//   just system-register
//   go run ./.tests/e2e/producer -count=0 -stream=compat.lab
// Apply any newer system/stream migrations with the current CLI before:
//   just compat-lab round-trip
// Use refused instead when the current registry excludes the pinned build.

require github.com/allegedlyreliable/sqlstreams v0.1.0-rc.1

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.32.0 // indirect
)
