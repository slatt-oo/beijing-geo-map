//go:build integration

package testdb

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestURLIgnoresDatabaseQueryParameters(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Fatal("TEST_DATABASE_URL is not set")
	}
	t.Setenv("TEST_DATABASE_URL", base+"&dbname=geo")

	dbURL := URL(t, "testdb_guard")
	for _, param := range []string{"dbname=", "database="} {
		if strings.Contains(dbURL, param) {
			t.Errorf("URL still contains %q", param)
		}
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)
	var current string
	if err := conn.QueryRow(ctx, `SELECT current_database()`).Scan(&current); err != nil {
		t.Fatalf("current_database: %v", err)
	}
	if current != "geo_test_testdb_guard" {
		t.Fatalf("current_database() = %q, want %q", current, "geo_test_testdb_guard")
	}
}
