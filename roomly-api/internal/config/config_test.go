package config

import (
	"testing"

	mysql "github.com/go-sql-driver/mysql"
)

func TestDSNUsesUTCForMySQLSessionAndParsing(t *testing.T) {
	cfg := &Config{DBUser: "roomly", DBPassword: "p@ss:/word", DBHost: "127.0.0.1", DBPort: 3306, DBName: "roomly_test"}
	parsed, err := mysql.ParseDSN(cfg.DSN())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Loc.String() != "UTC" {
		t.Fatalf("parsed location = %q, want UTC", parsed.Loc)
	}
	if parsed.Params["time_zone"] != "'+00:00'" {
		t.Fatalf("time_zone param = %q, want '+00:00'", parsed.Params["time_zone"])
	}
	if parsed.Passwd != cfg.DBPassword {
		t.Fatalf("parsed password did not preserve special characters")
	}
}
