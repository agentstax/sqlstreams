module github.com/agentstax/vulkan/e2e

go 1.27.0

// Dev-only module: keeps end-to-end e2e tests out of the root library module's
// published zip and its `go test ./...` surface. The parent module
// github.com/agentstax/vulkan is resolved locally via the repo-root go.work
// (use ./.e2e) and deliberately has NO require line here: it's
// unpublished, so any placeholder version poisons the whole workspace graph.
// Unlike cmd/vulkan and otel, this module is never tagged or published,
// so it never takes a pinned require at release.

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.10.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.32.0 // indirect
)
