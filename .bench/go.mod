module github.com/allegedlyreliable/sqlstreams/.bench

go 1.27.0

// Dev-only benchmark harnesses stay outside the root module's published zip.
// Never tagged or published; go.work uses local source during development.

require (
	github.com/allegedlyreliable/sqlstreams v0.1.0-rc.1
	github.com/jackc/pgx/v5 v5.10.0
	golang.org/x/sync v0.20.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/text v0.32.0 // indirect
)
