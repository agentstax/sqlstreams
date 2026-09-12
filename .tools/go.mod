module github.com/allegedlyreliable/sqlstreams/.tools

go 1.27.0

// Dev-only tooling is never imported by production code or published.
// go.work uses local source; the version pin allows standalone tooling.

require github.com/allegedlyreliable/sqlstreams v0.1.0-rc.1

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/text v0.32.0 // indirect
)
