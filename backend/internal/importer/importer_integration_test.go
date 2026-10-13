//go:build integration

package importer_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/geo"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/importer"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/testdb"
)

const fixtureDir = "testdata"

func runImport(t *testing.T, pool *pgxpool.Pool, dir string, minPOIs int) importer.Stats {
	t.Helper()
	stats, err := importer.Run(context.Background(), pool, importer.Options{DataDir: dir, MinPOIs: minPOIs})
	if err != nil {
		t.Fatalf("import %s: %v", dir, err)
	}
	return stats
}

type storedPOI struct {
	ID       int64
	Name     string
	Category string
	Adcode   *string
}

func storedPOIs(t *testing.T, pool *pgxpool.Pool) map[string]storedPOI {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT p.osm_type || p.osm_id, p.id, p.name, c.code, d.adcode
		FROM poi p
		JOIN poi_category c ON c.id = p.category_id
		LEFT JOIN district d ON d.id = p.district_id`)
	if err != nil {
		t.Fatalf("query pois: %v", err)
	}
	type keyed struct {
		key string
		poi storedPOI
	}
	collected, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (keyed, error) {
		var k keyed
		err := row.Scan(&k.key, &k.poi.ID, &k.poi.Name, &k.poi.Category, &k.poi.Adcode)
		return k, err
	})
	if err != nil {
		t.Fatalf("scan pois: %v", err)
	}
	result := make(map[string]storedPOI, len(collected))
	for _, k := range collected {
		result[k.key] = k.poi
	}
	return result
}

func countPOIs(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM poi`).Scan(&n); err != nil {
		t.Fatalf("count pois: %v", err)
	}
	return n
}

func TestRunLoadsFixtureSnapshot(t *testing.T) {
	pool := testdb.New(t, "importer")

	stats := runImport(t, pool, fixtureDir, 1)

	want := importer.Stats{
		Districts:        2,
		POIs:             6,
		POIsWithDistrict: 5,
		POIsByCategory:   map[string]int{"subway_station": 1, "university": 1, "attraction": 2, "hospital": 1, "park": 1},
	}
	if !reflect.DeepEqual(stats, want) {
		t.Fatalf("stats = %+v, want %+v", stats, want)
	}

	pois := storedPOIs(t, pool)
	wantPOIs := map[string]struct{ name, category, adcode string }{
		"n1001": {"测试大学", "university", "990001"},
		"w2001": {"测试园林", "attraction", "990002"},
		"w2002": {"乙区公园", "park", "990002"},
		"r3001": {"测试医院", "hospital", "990001"},
		"n1002": {"测试站", "subway_station", "990001"},
		"n1004": {"区外博物馆", "attraction", ""},
	}
	if len(pois) != len(wantPOIs) {
		t.Fatalf("stored POIs = %v", pois)
	}
	for key, w := range wantPOIs {
		got, ok := pois[key]
		if !ok {
			t.Errorf("POI %s is missing", key)
			continue
		}
		adcode := ""
		if got.Adcode != nil {
			adcode = *got.Adcode
		}
		if got.Name != w.name || got.Category != w.category || adcode != w.adcode {
			t.Errorf("POI %s = {%s %s %q}, want {%s %s %q}", key, got.Name, got.Category, adcode, w.name, w.category, w.adcode)
		}
	}

	var nameEn, address, website string
	err := pool.QueryRow(context.Background(),
		`SELECT name_en, address, tags->>'website' FROM poi WHERE osm_type = 'n' AND osm_id = 1001`).
		Scan(&nameEn, &address, &website)
	if err != nil {
		t.Fatalf("query n1001: %v", err)
	}
	if nameEn != "Test University" || address != "测试路1号" || website != "https://example.edu" {
		t.Errorf("n1001 = (%q, %q, %q)", nameEn, address, website)
	}

	wantLng, wantLat := geo.GCJ02ToWGS84(116.05, 40.05)
	var gotLng, gotLat float64
	err = pool.QueryRow(context.Background(),
		`SELECT ST_X(ST_Centroid(geom)), ST_Y(ST_Centroid(geom)) FROM district WHERE adcode = '990001'`).
		Scan(&gotLng, &gotLat)
	if err != nil {
		t.Fatalf("query district centroid: %v", err)
	}
	if math.Abs(gotLng-wantLng) > 1e-4 || math.Abs(gotLat-wantLat) > 1e-4 {
		t.Fatalf("district centroid = (%v, %v), want about (%v, %v): boundary was not converted to WGS-84",
			gotLng, gotLat, wantLng, wantLat)
	}
}

func TestRunIsIdempotent(t *testing.T) {
	pool := testdb.New(t, "importer")
	runImport(t, pool, fixtureDir, 1)
	before := storedPOIs(t, pool)

	stats := runImport(t, pool, fixtureDir, 1)

	after := storedPOIs(t, pool)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("second import changed POIs:\nbefore %v\nafter  %v", before, after)
	}
	if stats.Districts != 2 || stats.POIs != 6 || stats.DeletedPOIs != 0 {
		t.Fatalf("second import stats = %+v", stats)
	}
}

func TestRunDeletesPOIsMissingFromSnapshot(t *testing.T) {
	pool := testdb.New(t, "importer")
	runImport(t, pool, fixtureDir, 1)

	dir := t.TempDir()
	copyFile(t, filepath.Join(fixtureDir, importer.DistrictsFile), filepath.Join(dir, importer.DistrictsFile))
	writeSnapshotWithout(t, filepath.Join(dir, importer.POIsFile), 2002)

	stats := runImport(t, pool, dir, 1)

	if stats.POIs != 5 || stats.DeletedPOIs != 1 {
		t.Fatalf("stats = %+v, want 5 POIs and 1 deleted", stats)
	}
	if _, ok := storedPOIs(t, pool)["w2002"]; ok {
		t.Fatal("w2002 should have been deleted")
	}
}

func TestRunRejectsSnapshotBelowMinimumWithoutTouchingData(t *testing.T) {
	pool := testdb.New(t, "importer")
	runImport(t, pool, fixtureDir, 1)

	_, err := importer.Run(context.Background(), pool, importer.Options{DataDir: fixtureDir, MinPOIs: 7})

	if err == nil || !strings.Contains(err.Error(), "minimum") {
		t.Fatalf("err = %v, want a minimum-size error", err)
	}
	if n := countPOIs(t, pool); n != 6 {
		t.Fatalf("POIs after rejected import = %d, want 6", n)
	}
}

func TestRunImportsRealSnapshot(t *testing.T) {
	pool := testdb.New(t, "importer")

	stats := runImport(t, pool, filepath.Join("..", "..", "..", "data", "raw"), 1000)

	if stats.Districts != 16 {
		t.Errorf("districts = %d, want 16", stats.Districts)
	}
	if stats.POIs < 4000 || stats.POIs > 4300 {
		t.Errorf("POIs = %d, want between 4000 and 4300", stats.POIs)
	}
	if ratio := float64(stats.POIsWithDistrict) / float64(stats.POIs); ratio < 0.95 {
		t.Errorf("district fill rate = %.4f, want at least 0.95", ratio)
	}
	for _, code := range []string{"subway_station", "university", "attraction", "hospital", "park"} {
		if stats.POIsByCategory[code] == 0 {
			t.Errorf("no POIs imported for category %s", code)
		}
	}
	var totalArea float64
	if err := pool.QueryRow(context.Background(), `SELECT sum(area_km2)::float8 FROM district`).Scan(&totalArea); err != nil {
		t.Fatalf("sum area: %v", err)
	}
	if totalArea < 16300 || totalArea > 16500 {
		t.Errorf("total area = %.2f km², want about 16410 km²", totalArea)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

func writeSnapshotWithout(t *testing.T, dst string, osmID int64) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir, importer.POIsFile))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var doc struct {
		Elements []map[string]any `json:"elements"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	kept := doc.Elements[:0]
	for _, el := range doc.Elements {
		if el["id"] != float64(osmID) {
			kept = append(kept, el)
		}
	}
	out, err := json.Marshal(map[string]any{"elements": kept})
	if err != nil {
		t.Fatalf("encode snapshot: %v", err)
	}
	if err := os.WriteFile(dst, out, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
}
