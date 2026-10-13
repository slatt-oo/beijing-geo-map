package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

type DistrictRepo struct {
	db *pgxpool.Pool
}

func NewDistrictRepo(db *pgxpool.Pool) *DistrictRepo {
	return &DistrictRepo{db: db}
}

// ST_SimplifyPreserveTopology 会把只含一个多边形的 MultiPolygon 简化成 Polygon，所以结果要再经过 ST_Multi。
const listDistrictsSQL = `
SELECT d.adcode, d.name, d.area_km2::float8,
       round(ST_X(d.center)::numeric, 6)::float8, round(ST_Y(d.center)::numeric, 6)::float8,
       ST_AsGeoJSON(ST_Multi(ST_SimplifyPreserveTopology(d.geom, 0.0001)), 6),
       count(p.id)
FROM district d
LEFT JOIN poi p ON p.district_id = d.id
GROUP BY d.id
ORDER BY d.adcode`

func (r *DistrictRepo) List(ctx context.Context) ([]model.DistrictFeature, error) {
	rows, err := r.db.Query(ctx, listDistrictsSQL)
	if err != nil {
		return nil, fmt.Errorf("list districts: %w", err)
	}
	districts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.DistrictFeature, error) {
		var d model.DistrictFeature
		var geometry string
		err := row.Scan(&d.Adcode, &d.Name, &d.AreaKm2, &d.Center[0], &d.Center[1], &geometry, &d.POICount)
		d.Geometry = json.RawMessage(geometry)
		return d, err
	})
	if err != nil {
		return nil, fmt.Errorf("list districts: %w", err)
	}
	return districts, nil
}

const getDistrictSQL = `
SELECT id, adcode, name, area_km2::float8,
       round(ST_X(center)::numeric, 6)::float8, round(ST_Y(center)::numeric, 6)::float8
FROM district
WHERE adcode = $1`

const districtCategoryCountsSQL = `
SELECT c.code, c.name, count(p.id)
FROM poi_category c
LEFT JOIN poi p ON p.category_id = c.id AND p.district_id = $1
GROUP BY c.id
ORDER BY c.sort_order`

func (r *DistrictRepo) Get(ctx context.Context, adcode string) (model.DistrictDetail, error) {
	var d model.DistrictDetail
	var id int32
	err := r.db.QueryRow(ctx, getDistrictSQL, adcode).Scan(&id, &d.Adcode, &d.Name, &d.AreaKm2, &d.Center[0], &d.Center[1])
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DistrictDetail{}, ErrNotFound
	}
	if err != nil {
		return model.DistrictDetail{}, fmt.Errorf("get district %s: %w", adcode, err)
	}
	rows, err := r.db.Query(ctx, districtCategoryCountsSQL, id)
	if err != nil {
		return model.DistrictDetail{}, fmt.Errorf("count pois of district %s: %w", adcode, err)
	}
	d.POICounts, err = pgx.CollectRows(rows, pgx.RowToStructByPos[model.CategoryCount])
	if err != nil {
		return model.DistrictDetail{}, fmt.Errorf("count pois of district %s: %w", adcode, err)
	}
	return d, nil
}

const locateDistrictSQL = `
SELECT adcode, name
FROM district
WHERE ST_Contains(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326))
ORDER BY adcode
LIMIT 1`

// Locate 返回包含该点的区；点不在任何区内（包括恰好落在边界线上）时返回 nil, nil。
func (r *DistrictRepo) Locate(ctx context.Context, lng, lat float64) (*model.DistrictRef, error) {
	var ref model.DistrictRef
	err := r.db.QueryRow(ctx, locateDistrictSQL, lng, lat).Scan(&ref.Adcode, &ref.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("locate district: %w", err)
	}
	return &ref, nil
}
