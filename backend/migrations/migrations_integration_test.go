//go:build integration

package migrations_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/testdb"
	"github.com/slatt-oo/beijing-geo-map/backend/migrations"
)

func connect(t *testing.T, dbURL string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

func freshDatabase(t *testing.T) string {
	t.Helper()
	dbURL := testdb.URL(t, "migrations")
	conn := connect(t, dbURL)
	if _, err := conn.Exec(context.Background(), `DROP TABLE IF EXISTS poi, poi_category, district, schema_migrations CASCADE`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	return dbURL
}

func queryStrings(t *testing.T, conn *pgx.Conn, sql string) []string {
	t.Helper()
	rows, err := conn.Query(context.Background(), sql)
	if err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	values, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect %q: %v", sql, err)
	}
	return values
}

const appTablesSQL = `SELECT tablename FROM pg_tables
WHERE schemaname = 'public' AND tablename IN ('district', 'poi_category', 'poi')
ORDER BY tablename`

func TestUpCreatesSchemaAndIsIdempotent(t *testing.T) {
	dbURL := freshDatabase(t)

	if err := migrations.Up(dbURL); err != nil {
		t.Fatalf("first Up: %v", err)
	}
	if err := migrations.Up(dbURL); err != nil {
		t.Fatalf("second Up must be a no-op: %v", err)
	}

	conn := connect(t, dbURL)
	if got := queryStrings(t, conn, appTablesSQL); !slices.Equal(got, []string{"district", "poi", "poi_category"}) {
		t.Fatalf("tables = %v", got)
	}
	indexes := queryStrings(t, conn, `SELECT indexname FROM pg_indexes WHERE schemaname = 'public'`)
	for _, want := range []string{
		"district_geom_gist", "poi_geom_gist", "poi_geog_gist", "poi_name_trgm",
		"poi_category_id_idx", "poi_district_id_idx",
		"district_adcode_uk", "poi_osm_uk", "poi_category_code_uk",
	} {
		if !slices.Contains(indexes, want) {
			t.Errorf("missing index %s", want)
		}
	}
	codes := queryStrings(t, conn, `SELECT code FROM poi_category ORDER BY sort_order`)
	if want := []string{"subway_station", "university", "attraction", "hospital", "park"}; !slices.Equal(codes, want) {
		t.Fatalf("category codes = %v, want %v", codes, want)
	}
}

func TestDownRemovesSchemaAndUpRestoresIt(t *testing.T) {
	dbURL := freshDatabase(t)
	if err := migrations.Up(dbURL); err != nil {
		t.Fatalf("Up: %v", err)
	}

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("iofs: %v", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, strings.Replace(dbURL, "postgres://", "pgx5://", 1))
	if err != nil {
		t.Fatalf("init migrate: %v", err)
	}
	if err := m.Down(); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
		t.Fatalf("close migrate: %v, %v", srcErr, dbErr)
	}

	conn := connect(t, dbURL)
	if got := queryStrings(t, conn, appTablesSQL); len(got) != 0 {
		t.Fatalf("tables left after Down: %v", got)
	}

	if err := migrations.Up(dbURL); err != nil {
		t.Fatalf("Up after Down: %v", err)
	}
	var categories int
	if err := conn.QueryRow(context.Background(), `SELECT count(*) FROM poi_category`).Scan(&categories); err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if categories != 5 {
		t.Fatalf("categories after Up = %d, want 5", categories)
	}
}
