module github.com/agentstax/vulkan/examples

go 1.27.0

// Dev-only module: keeps runnable examples out of the root library module's
// published zip and its `go test ./...` surface. The parent module
// github.com/agentstax/vulkan is resolved locally via the repo-root go.work
// (use ./.examples) and deliberately has NO require line here: it's
// unpublished, so any placeholder version poisons the whole workspace graph.
// Unlike cmd/vulkan and otel, this module is never tagged or published,
// so it never takes a pinned require at release.

require github.com/jackc/pgx/v5 v5.10.0
