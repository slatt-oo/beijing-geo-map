package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

type POIRepo struct {
	db *pgxpool.Pool
}

func NewPOIRepo(db *pgxpool.Pool) *POIRepo {
	return &POIRepo{db: db}
}

const poiLocationColumns = `round(ST_X(p.geom)::numeric, 6)::float8, round(ST_Y(p.geom)::numeric, 6)::float8`

const poisInBBoxSQL = `
SELECT p.id, p.name, c.code, ` + poiLocationColumns + `
FROM poi p
JOIN poi_category c ON c.id = p.category_id
WHERE p.geom && ST_MakeEnvelope($1, $2, $3, $4, 4326)
  AND ($5::smallint[] IS NULL OR p.category_id = ANY($5))
ORDER BY md5(p.id::text), p.id
LIMIT $6`

// InBBox 返回落在 box 内的 POI。categoryIDs 为 nil 或空表示不按分类过滤。
// 结果按 md5(id) 打散的固定顺序排列：id 按导入顺序分配，各分类在 id 上分布不均（地铁站全部排在最后），
// 按 id 截断会让排在后面的分类整类缺失。多查 1 条来判断是否截断，比额外执行 count(*) 更省。
func (r *POIRepo) InBBox(ctx context.Context, box model.BBox, categoryIDs []int16, limit int) ([]model.POISummary, bool, error) {
	if len(categoryIDs) == 0 {
		categoryIDs = nil
	}
	rows, err := r.db.Query(ctx, poisInBBoxSQL, box.MinLng, box.MinLat, box.MaxLng, box.MaxLat, categoryIDs, limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("pois in bbox: %w", err)
	}
	pois, err := pgx.CollectRows(rows, scanPOISummary)
	if err != nil {
		return nil, false, fmt.Errorf("pois in bbox: %w", err)
	}
	if len(pois) > limit {
		return pois[:limit], true, nil
	}
	return pois, false, nil
}

func scanPOISummary(row pgx.CollectableRow) (model.POISummary, error) {
	var p model.POISummary
	err := row.Scan(&p.ID, &p.Name, &p.Category, &p.Location[0], &p.Location[1])
	return p, err
}

const getPOISQL = `
SELECT p.id, p.name, p.name_en, p.osm_type, p.osm_id, p.address, p.tags,
       c.id, c.code, c.name, c.color,
       d.adcode, d.name,
       ` + poiLocationColumns + `
FROM poi p
JOIN poi_category c ON c.id = p.category_id
LEFT JOIN district d ON d.id = p.district_id
WHERE p.id = $1`

func (r *POIRepo) Get(ctx context.Context, id int64) (model.POIDetail, error) {
	var p model.POIDetail
	var adcode, districtName *string
	err := r.db.QueryRow(ctx, getPOISQL, id).Scan(
		&p.ID, &p.Name, &p.NameEn, &p.OSMType, &p.OSMID, &p.Address, &p.Tags,
		&p.Category.ID, &p.Category.Code, &p.Category.Name, &p.Category.Color,
		&adcode, &districtName,
		&p.Location[0], &p.Location[1],
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.POIDetail{}, ErrNotFound
	}
	if err != nil {
		return model.POIDetail{}, fmt.Errorf("get poi %d: %w", id, err)
	}
	if adcode != nil && districtName != nil {
		p.District = &model.DistrictRef{Adcode: *adcode, Name: *districtName}
	}
	return p, nil
}

const searchPOIsSQL = `
SELECT p.id, p.name, c.code, ` + poiLocationColumns + `, d.name
FROM poi p
JOIN poi_category c ON c.id = p.category_id
LEFT JOIN district d ON d.id = p.district_id
WHERE p.name ILIKE $1::text
ORDER BY (p.name = $2::text) DESC, similarity(p.name, $2::text) DESC, length(p.name), p.id
LIMIT $3`

// Search 按名称模糊搜索，q 中的 % 和 _ 按字面匹配。排序规则：完全匹配优先，然后按相似度从高到低，再按名称从短到长。
func (r *POIRepo) Search(ctx context.Context, q string, limit int) ([]model.SearchHit, error) {
	rows, err := r.db.Query(ctx, searchPOIsSQL, "%"+escapeLike(q)+"%", q, limit)
	if err != nil {
		return nil, fmt.Errorf("search pois: %w", err)
	}
	hits, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.SearchHit, error) {
		var h model.SearchHit
		err := row.Scan(&h.ID, &h.Name, &h.Category, &h.Location[0], &h.Location[1], &h.DistrictName)
		return h, err
	})
	if err != nil {
		return nil, fmt.Errorf("search pois: %w", err)
	}
	return hits, nil
}

const nearbyPOIsSQL = `
WITH origin AS (
    SELECT ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography AS g
)
SELECT p.id, p.name, c.code, ` + poiLocationColumns + `,
       round(ST_Distance(p.geom::geography, o.g))::int AS distance_m
FROM poi p
JOIN poi_category c ON c.id = p.category_id
CROSS JOIN origin o
WHERE ST_DWithin(p.geom::geography, o.g, $3)
  AND ($4::smallint[] IS NULL OR p.category_id = ANY($4))
ORDER BY distance_m, p.id
LIMIT $5`

// Nearby 返回距 (lng, lat) radiusM 米以内的 POI，按距离从近到远排序。categoryIDs 为 nil 或空表示不按分类过滤。
// 过滤条件必须写成 p.geom::geography，与表达式索引 poi_geog_gist 一致，否则用不上索引。
func (r *POIRepo) Nearby(ctx context.Context, lng, lat, radiusM float64, categoryIDs []int16, limit int) ([]model.NearbyPOI, error) {
	if len(categoryIDs) == 0 {
		categoryIDs = nil
	}
	rows, err := r.db.Query(ctx, nearbyPOIsSQL, lng, lat, radiusM, categoryIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("nearby pois: %w", err)
	}
	pois, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.NearbyPOI, error) {
		var p model.NearbyPOI
		err := row.Scan(&p.ID, &p.Name, &p.Category, &p.Location[0], &p.Location[1], &p.DistanceM)
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("nearby pois: %w", err)
	}
	return pois, nil
}
