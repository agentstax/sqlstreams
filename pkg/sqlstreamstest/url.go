package sqlstreamstest

import (
	"os"
	"testing"
)

// DatabaseURL returns the URL SQLSTREAMS_TEST_DATABASE_URL names, for a
// test whose subject takes a URL rather than a pool (the CLI). The test
// skips when the variable is unset.
func DatabaseURL(t testing.TB) string {
	t.Helper()
	url := os.Getenv(databaseURLVariable)
	if url == "" {
		t.Skipf("%s is unset -- set it to a disposable PostgreSQL database URL to run database tests", databaseURLVariable)
	}
	return url
}
