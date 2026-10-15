package importer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	DataDir string
	// MinPOIs 是范围过滤后 POI 数量的下限。不足这个数量时拒绝导入，避免损坏的快照把 poi 表同步删空。
	MinPOIs int
}

type Stats struct {
	Districts            int
	POIs                 int
	POIsWithDistrict     int
	POIsByCategory       map[string]int
	DeletedPOIs          int64
	SkippedOutsideRegion int
}

// Run 导入 DataDir 下的两份快照。除了快照校验，其余步骤都在同一个事务里完成，任何一步失败都会整体回滚。
func Run(ctx context.Context, pool *pgxpool.Pool, opts Options) (Stats, error) {
	districts, err := readDistricts(filepath.Join(opts.DataDir, DistrictsFile))
	if err != nil {
		return Stats{}, err
	}
	pois, err := readPOIs(filepath.Join(opts.DataDir, POIsFile))
	if err != nil {
		return Stats{}, err
	}
	pois, skipped := withinRegion(pois, beijingRegion)
	if len(pois) < opts.MinPOIs {
		return Stats{}, fmt.Errorf("snapshot has %d POIs inside the Beijing region, fewer than the minimum %d; refusing to import", len(pois), opts.MinPOIs)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := upsertDistricts(ctx, tx, districts); err != nil {
		return Stats{}, err
	}
	categoryIDs, err := loadCategoryIDs(ctx, tx)
	if err != nil {
		return Stats{}, err
	}
	if err := upsertPOIs(ctx, tx, pois, categoryIDs); err != nil {
		return Stats{}, err
	}
	deleted, err := deleteMissingPOIs(ctx, tx, pois)
	if err != nil {
		return Stats{}, err
	}
	if _, err := tx.Exec(ctx, assignDistrictsSQL); err != nil {
		return Stats{}, fmt.Errorf("assign districts: %w", err)
	}
	stats, err := collectStats(ctx, tx)
	if err != nil {
		return Stats{}, err
	}
	stats.DeletedPOIs = deleted
	stats.SkippedOutsideRegion = skipped
	if err := tx.Commit(ctx); err != nil {
		return Stats{}, fmt.Errorf("commit: %w", err)
	}
	return stats, nil
}

func readDistricts(path string) ([]District, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open districts snapshot: %w", err)
	}
	defer f.Close()
	districts, err := ParseDistricts(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return districts, nil
}

func readPOIs(path string) ([]POI, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open POI snapshot: %w", err)
	}
	defer f.Close()
	pois, err := ParsePOIs(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return pois, nil
}

const upsertDistrictSQL = `
WITH src AS (
    SELECT ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON($3::text), 4326)), 3)) AS g
)
INSERT INTO district (adcode, name, center, geom, area_km2)
SELECT $1::text, $2::text, ST_PointOnSurface(g), g, round((ST_Area(g::geography) / 1e6)::numeric, 2)
FROM src
ON CONFLICT (adcode) DO UPDATE
SET name       = EXCLUDED.name,
    center     = EXCLUDED.center,
    geom       = EXCLUDED.geom,
    area_km2   = EXCLUDED.area_km2,
    updated_at = now()`

func upsertDistricts(ctx context.Context, tx pgx.Tx, districts []District) error {
	batch := &pgx.Batch{}
	for _, d := range districts {
		batch.Queue(upsertDistrictSQL, d.Adcode, d.Name, string(d.Geometry))
	}
	results := tx.SendBatch(ctx, batch)
	for _, d := range districts {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("upsert district %s: %w", d.Adcode, err)
		}
	}
	return results.Close()
}

func loadCategoryIDs(ctx context.Context, tx pgx.Tx) (map[string]int16, error) {
	rows, err := tx.Query(ctx, `SELECT code, id FROM poi_category`)
	if err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	ids := map[string]int16{}
	var code string
	var id int16
	if _, err := pgx.ForEachRow(rows, []any{&code, &id}, func() error {
		ids[code] = id
		return nil
	}); err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	return ids, nil
}

const upsertPOISQL = `
INSERT INTO poi (osm_type, osm_id, name, name_en, category_id, geom, address, tags)
VALUES ($1, $2, $3, $4, $5, ST_SetSRID(ST_MakePoint($6, $7), 4326), $8, $9)
ON CONFLICT (osm_type, osm_id) DO UPDATE
SET name        = EXCLUDED.name,
    name_en     = EXCLUDED.name_en,
    category_id = EXCLUDED.category_id,
    geom        = EXCLUDED.geom,
    address     = EXCLUDED.address,
    tags        = EXCLUDED.tags,
    updated_at  = now()`

func upsertPOIs(ctx context.Context, tx pgx.Tx, pois []POI, categoryIDs map[string]int16) error {
	batch := &pgx.Batch{}
	for _, p := range pois {
		categoryID, ok := categoryIDs[p.Category]
		if !ok {
			return fmt.Errorf("poi %s%d: unknown category %q", p.OSMType, p.OSMID, p.Category)
		}
		batch.Queue(upsertPOISQL, p.OSMType, p.OSMID, p.Name, p.NameEn, categoryID, p.Lng, p.Lat, p.Address, p.Tags)
	}
	results := tx.SendBatch(ctx, batch)
	for _, p := range pois {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("upsert poi %s%d: %w", p.OSMType, p.OSMID, err)
		}
	}
	return results.Close()
}

func deleteMissingPOIs(ctx context.Context, tx pgx.Tx, pois []POI) (int64, error) {
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE snapshot_keys (osm_type char(1), osm_id bigint) ON COMMIT DROP`); err != nil {
		return 0, fmt.Errorf("create snapshot_keys: %w", err)
	}
	keys := make([][]any, len(pois))
	for i, p := range pois {
		keys[i] = []any{p.OSMType, p.OSMID}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"snapshot_keys"}, []string{"osm_type", "osm_id"}, pgx.CopyFromRows(keys)); err != nil {
		return 0, fmt.Errorf("copy snapshot keys: %w", err)
	}
	tag, err := tx.Exec(ctx, `
DELETE FROM poi p
WHERE NOT EXISTS (
    SELECT 1 FROM snapshot_keys k
    WHERE k.osm_type = p.osm_type AND k.osm_id = p.osm_id
)`)
	if err != nil {
		return 0, fmt.Errorf("delete missing pois: %w", err)
	}
	return tag.RowsAffected(), nil
}

// 一条语句同时完成分配和清空：不在任何区内的 POI 得到 NULL。多个区都包含同一个点时取 adcode 最小的区，
// 保证重复导入的结果稳定。
const assignDistrictsSQL = `
UPDATE poi p
SET district_id = m.district_id, updated_at = now()
FROM (
    SELECT p2.id AS poi_id,
           (SELECT d.id FROM district d WHERE ST_Contains(d.geom, p2.geom) ORDER BY d.adcode LIMIT 1) AS district_id
    FROM poi p2
) m
WHERE m.poi_id = p.id
  AND p.district_id IS DISTINCT FROM m.district_id`

func collectStats(ctx context.Context, tx pgx.Tx) (Stats, error) {
	var s Stats
	err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM district), (SELECT count(*) FROM poi), (SELECT count(district_id) FROM poi)`).
		Scan(&s.Districts, &s.POIs, &s.POIsWithDistrict)
	if err != nil {
		return Stats{}, fmt.Errorf("count rows: %w", err)
	}
	rows, err := tx.Query(ctx, `SELECT c.code, count(*) FROM poi p JOIN poi_category c ON c.id = p.category_id GROUP BY c.code`)
	if err != nil {
		return Stats{}, fmt.Errorf("count pois by category: %w", err)
	}
	s.POIsByCategory = map[string]int{}
	var code string
	var n int
	if _, err := pgx.ForEachRow(rows, []any{&code, &n}, func() error {
		s.POIsByCategory[code] = n
		return nil
	}); err != nil {
		return Stats{}, fmt.Errorf("count pois by category: %w", err)
	}
	return s, nil
}
