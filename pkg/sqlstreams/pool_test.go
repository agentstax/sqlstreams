package sqlstreams

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// regression: fmt.Sprintf once built a DSN that pgx parsed as another host
// when the password held reserved characters or the host was IPv6.
func TestConnectionStringSurvivesReservedCharacters(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		password string
		host     string
		database string
		port     int
	}{
		{name: "plain", user: "app_user", password: "secret", host: "localhost", database: "app_db", port: 5432},
		{name: "reserved characters in password", user: "app_user", password: "p@ss:w/rd#1", host: "localhost", database: "app_db", port: 5432},
		{name: "no password", user: "app_user", password: "", host: "localhost", database: "app_db", port: 5432},
		{name: "ipv6 host", user: "app_user", password: "secret", host: "::1", database: "app_db", port: 5433},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dsn := connectionString(test.user, test.password, test.host, test.database, test.port)
			parsed, err := pgxpool.ParseConfig(dsn)
			if err != nil {
				t.Fatalf("ParseConfig(%q) = %v, want nil", dsn, err)
			}

			if parsed.ConnConfig.User != test.user {
				t.Errorf("ParseConfig(%q).User = %q, want %q", dsn, parsed.ConnConfig.User, test.user)
			}
			if parsed.ConnConfig.Password != test.password {
				t.Errorf("ParseConfig(%q).Password = %q, want %q", dsn, parsed.ConnConfig.Password, test.password)
			}
			if parsed.ConnConfig.Host != test.host {
				t.Errorf("ParseConfig(%q).Host = %q, want %q", dsn, parsed.ConnConfig.Host, test.host)
			}
			if parsed.ConnConfig.Database != test.database {
				t.Errorf("ParseConfig(%q).Database = %q, want %q", dsn, parsed.ConnConfig.Database, test.database)
			}
			if int(parsed.ConnConfig.Port) != test.port {
				t.Errorf("ParseConfig(%q).Port = %d, want %d", dsn, parsed.ConnConfig.Port, test.port)
			}
		})
	}
}
