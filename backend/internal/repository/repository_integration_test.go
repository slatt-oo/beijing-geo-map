//go:build integration

package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/repository"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/testdb"
)

// 测试数据：两个相邻的正方形区（甲区经度 116.0~116.1，乙区 116.1~116.2，纬度都是 40.0~40.1）和 8 个 POI。
// 北京大学医学部、清华大学分别位于北京大学正北 499.66 米、999.32 米处（WGS-84 椭球上的子午线弧长）。
var boxA = model.BBox{MinLng: 116.00, MinLat: 40.00, MaxLng: 116.10, MaxLat: 40.10}

type fixture struct {
	pool   *pgxpool.Pool
	poiIDs map[string]int64
}

func ptr[T any](v T) *T { return &v }

func setup(t *testing.T) fixture {
	t.Helper()
	pool := testdb.New(t, "repository")
	districtA := insertDistrict(t, pool, "990001", "测试甲区", 116.00, 40.00, 116.10, 40.10)
	districtB := insertDistrict(t, pool, "990002", "测试乙区", 116.10, 40.00, 116.20, 40.10)

	f := fixture{pool: pool, poiIDs: map[string]int64{}}
	for i, p := range []struct {
		name     string
		nameEn   *string
		category string
		district *int32
		lng, lat float64
		tags     map[string]string
	}{
		{"清华大学", nil, "university", &districtA, 116.050, 40.059, map[string]string{}},
		{"北京大学医学部", nil, "university", &districtA, 116.050, 40.0545, map[string]string{}},
		{"北京大学", ptr("Peking University"), "university", &districtA, 116.050, 40.050, map[string]string{"website": "https://www.pku.edu.cn"}},
		{"测试地铁站", nil, "subway_station", &districtA, 116.020, 40.020, map[string]string{}},
		{"a_b 博物馆", nil, "attraction", &districtB, 116.170, 40.050, map[string]string{}},
		{"100%纯净公园", nil, "park", &districtB, 116.160, 40.050, map[string]string{}},
		{"测试公园", nil, "park", &districtB, 116.150, 40.050, map[string]string{}},
		{"境外医院", nil, "hospital", nil, 117.000, 41.000, map[string]string{}},
	} {
		var id int64
		err := pool.QueryRow(context.Background(), `
			INSERT INTO poi (osm_type, osm_id, name, name_en, category_id, district_id, geom, tags)
			VALUES ('n', $1, $2, $3, (SELECT id FROM poi_category WHERE code = $4), $5,
			        ST_SetSRID(ST_MakePoint($6, $7), 4326), $8)
			RETURNING id`,
			int64(i+1), p.name, p.nameEn, p.category, p.district, p.lng, p.lat, p.tags).Scan(&id)
		if err != nil {
			t.Fatalf("insert poi %s: %v", p.name, err)
		}
		f.poiIDs[p.name] = id
	}
	// 插入顺序（id 顺序）与各项查询的期望顺序都不同；再把 id 最小的一行挪到堆末尾，缺少 ORDER BY 时顺序扫描的结果就会暴露出来。
	if _, err := pool.Exec(context.Background(), `UPDATE poi SET tags = tags WHERE name = '清华大学'`); err != nil {
		t.Fatalf("move 清华大学 to the end of the heap: %v", err)
	}
	return f
}

func insertDistrict(t *testing.T, pool *pgxpool.Pool, adcode, name string, minLng, minLat, maxLng, maxLat float64) int32 {
	t.Helper()
	var id int32
	err := pool.QueryRow(context.Background(), `
		INSERT INTO district (adcode, name, center, geom, area_km2)
		SELECT $1::text, $2::text, ST_PointOnSurface(g), g, round((ST_Area(g::geography) / 1e6)::numeric, 2)
		FROM (SELECT ST_Multi(ST_MakeEnvelope($3, $4, $5, $6, 4326)) AS g) s
		RETURNING id`, adcode, name, minLng, minLat, maxLng, maxLat).Scan(&id)
	if err != nil {
		t.Fatalf("insert district %s: %v", adcode, err)
	}
	return id
}

func categoryID(t *testing.T, pool *pgxpool.Pool, code string) int16 {
	t.Helper()
	var id int16
	if err := pool.QueryRow(context.Background(), `SELECT id FROM poi_category WHERE code = $1`, code).Scan(&id); err != nil {
		t.Fatalf("category %s: %v", code, err)
	}
	return id
}

func summaryNames(pois []model.POISummary) []string {
	names := make([]string, len(pois))
	for i, p := range pois {
		names[i] = p.Name
	}
	return names
}

func nearbyNames(pois []model.NearbyPOI) []string {
	names := make([]string, len(pois))
	for i, p := range pois {
		names[i] = p.Name
	}
	return names
}

func hitNames(hits []model.SearchHit) []string {
	names := make([]string, len(hits))
	for i, h := range hits {
		names[i] = h.Name
	}
	return names
}

func TestLocateReturnsContainingDistrict(t *testing.T) {
	f := setup(t)
	repo := repository.NewDistrictRepo(f.pool)
	for _, tc := range []struct {
		lng, lat float64
		adcode   string
	}{{116.05, 40.05, "990001"}, {116.15, 40.05, "990002"}} {
		ref, err := repo.Locate(context.Background(), tc.lng, tc.lat)
		if err != nil {
			t.Fatalf("Locate: %v", err)
		}
		if ref == nil || ref.Adcode != tc.adcode {
			t.Fatalf("Locate(%v, %v) = %+v, want %s", tc.lng, tc.lat, ref, tc.adcode)
		}
	}
}

func TestLocateReturnsNilOutsideAllDistricts(t *testing.T) {
	f := setup(t)
	ref, err := repository.NewDistrictRepo(f.pool).Locate(context.Background(), 117.0, 41.0)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if ref != nil {
		t.Fatalf("Locate outside = %+v, want nil", ref)
	}
}

func TestListDistrictsReturnsCountsCentersAndGeometry(t *testing.T) {
	f := setup(t)
	districts, err := repository.NewDistrictRepo(f.pool).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(districts) != 2 || districts[0].Adcode != "990001" || districts[1].Adcode != "990002" {
		t.Fatalf("districts = %+v", districts)
	}
	a, b := districts[0], districts[1]
	if a.Name != "测试甲区" || a.POICount != 4 || b.POICount != 3 {
		t.Errorf("A = {%s %d}, B count = %d; want {测试甲区 4} and 3", a.Name, a.POICount, b.POICount)
	}
	if a.AreaKm2 < 90 || a.AreaKm2 > 100 {
		t.Errorf("A area = %v km², want about 94.6 km² (0.1° × 0.1° at 40°N)", a.AreaKm2)
	}
	if a.Center[0] <= 116.00 || a.Center[0] >= 116.10 || a.Center[1] <= 40.00 || a.Center[1] >= 40.10 {
		t.Errorf("A center %v is not inside the district", a.Center)
	}
	var geometry struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(a.Geometry, &geometry); err != nil || geometry.Type != "MultiPolygon" {
		t.Errorf("A geometry = %s (err %v), want a GeoJSON MultiPolygon", a.Geometry, err)
	}
}

func TestGetDistrictReturnsCountsForEveryCategory(t *testing.T) {
	f := setup(t)
	d, err := repository.NewDistrictRepo(f.pool).Get(context.Background(), "990001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if d.Adcode != "990001" || d.Name != "测试甲区" {
		t.Fatalf("district = %+v", d)
	}
	want := []model.CategoryCount{
		{Code: "subway_station", Name: "地铁站", Count: 1},
		{Code: "university", Name: "高校", Count: 3},
		{Code: "attraction", Name: "景点/博物馆", Count: 0},
		{Code: "hospital", Name: "医院", Count: 0},
		{Code: "park", Name: "公园", Count: 0},
	}
	if !slices.Equal(d.POICounts, want) {
		t.Fatalf("POICounts = %+v, want %+v", d.POICounts, want)
	}
}

func TestGetDistrictReturnsNotFoundForUnknownAdcode(t *testing.T) {
	f := setup(t)
	_, err := repository.NewDistrictRepo(f.pool).Get(context.Background(), "999999")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListCategoriesInDisplayOrder(t *testing.T) {
	f := setup(t)
	categories, err := repository.NewCategoryRepo(f.pool).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var codes []string
	for _, c := range categories {
		codes = append(codes, c.Code)
		if c.ID == 0 || len(c.Color) != 7 || !strings.HasPrefix(c.Color, "#") {
			t.Errorf("category %+v has an invalid id or color", c)
		}
	}
	if want := []string{"subway_station", "university", "attraction", "hospital", "park"}; !slices.Equal(codes, want) {
		t.Fatalf("codes = %v, want %v", codes, want)
	}
}

func TestInBBoxFiltersByEnvelopeAndCategory(t *testing.T) {
	f := setup(t)
	repo := repository.NewPOIRepo(f.pool)

	pois, truncated, err := repo.InBBox(context.Background(), boxA, nil, 100)
	if err != nil {
		t.Fatalf("InBBox: %v", err)
	}
	if want := []string{"清华大学", "北京大学医学部", "北京大学", "测试地铁站"}; !slices.Equal(summaryNames(pois), want) || truncated {
		t.Fatalf("InBBox = %v (truncated %v), want %v", summaryNames(pois), truncated, want)
	}
	pku := pois[slices.IndexFunc(pois, func(p model.POISummary) bool { return p.Name == "北京大学" })]
	if pku.Category != "university" || pku.Location != [2]float64{116.05, 40.05} {
		t.Errorf("北京大学 = %+v", pku)
	}

	pois, _, err = repo.InBBox(context.Background(), boxA, []int16{categoryID(t, f.pool, "subway_station")}, 100)
	if err != nil {
		t.Fatalf("InBBox with category: %v", err)
	}
	if want := []string{"测试地铁站"}; !slices.Equal(summaryNames(pois), want) {
		t.Fatalf("InBBox subway = %v, want %v", summaryNames(pois), want)
	}
}

func TestInBBoxReportsTruncation(t *testing.T) {
	f := setup(t)
	repo := repository.NewPOIRepo(f.pool)

	pois, truncated, err := repo.InBBox(context.Background(), boxA, nil, 2)
	if err != nil {
		t.Fatalf("InBBox: %v", err)
	}
	if want := []string{"清华大学", "北京大学医学部"}; !slices.Equal(summaryNames(pois), want) || !truncated {
		t.Fatalf("limit 2: got %v, truncated %v; want %v, true", summaryNames(pois), truncated, want)
	}

	pois, truncated, err = repo.InBBox(context.Background(), boxA, nil, 4)
	if err != nil {
		t.Fatalf("InBBox: %v", err)
	}
	if want := []string{"清华大学", "北京大学医学部", "北京大学", "测试地铁站"}; !slices.Equal(summaryNames(pois), want) || truncated {
		t.Fatalf("limit 4: got %v, truncated %v; want %v, false", summaryNames(pois), truncated, want)
	}
}

func TestGetPOIReturnsDetail(t *testing.T) {
	f := setup(t)
	p, err := repository.NewPOIRepo(f.pool).Get(context.Background(), f.poiIDs["北京大学"])
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.Name != "北京大学" || p.NameEn == nil || *p.NameEn != "Peking University" || p.OSMType != "n" || p.OSMID != 3 {
		t.Errorf("identity fields = %+v", p)
	}
	if p.Category.Code != "university" || p.Category.Name != "高校" || p.Category.Color != "#1E88E5" {
		t.Errorf("category = %+v", p.Category)
	}
	if p.District == nil || *p.District != (model.DistrictRef{Adcode: "990001", Name: "测试甲区"}) {
		t.Errorf("district = %+v", p.District)
	}
	if p.Location != [2]float64{116.05, 40.05} || p.Address != nil || p.Tags["website"] != "https://www.pku.edu.cn" {
		t.Errorf("location/address/tags = %v / %v / %v", p.Location, p.Address, p.Tags)
	}
}

func TestGetPOIWithoutDistrict(t *testing.T) {
	f := setup(t)
	p, err := repository.NewPOIRepo(f.pool).Get(context.Background(), f.poiIDs["境外医院"])
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.District != nil {
		t.Fatalf("district = %+v, want nil", p.District)
	}
}

func TestGetPOIReturnsNotFound(t *testing.T) {
	f := setup(t)
	_, err := repository.NewPOIRepo(f.pool).Get(context.Background(), 999999)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSearchRanksExactMatchFirst(t *testing.T) {
	f := setup(t)
	hits, err := repository.NewPOIRepo(f.pool).Search(context.Background(), "北京大学", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if want := []string{"北京大学", "北京大学医学部"}; !slices.Equal(hitNames(hits), want) {
		t.Fatalf("Search = %v, want %v", hitNames(hits), want)
	}
	if hits[0].DistrictName == nil || *hits[0].DistrictName != "测试甲区" || hits[0].Category != "university" {
		t.Errorf("first hit = %+v", hits[0])
	}
}

func TestSearchTreatsWildcardsLiterally(t *testing.T) {
	f := setup(t)
	repo := repository.NewPOIRepo(f.pool)
	for q, want := range map[string][]string{"%": {"100%纯净公园"}, "_": {"a_b 博物馆"}} {
		hits, err := repo.Search(context.Background(), q, 10)
		if err != nil {
			t.Fatalf("Search(%q): %v", q, err)
		}
		if !slices.Equal(hitNames(hits), want) {
			t.Errorf("Search(%q) = %v, want %v", q, hitNames(hits), want)
		}
	}
}

func TestSearchMatchesTwoCharacterChinese(t *testing.T) {
	f := setup(t)
	hits, err := repository.NewPOIRepo(f.pool).Search(context.Background(), "清华", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if want := []string{"清华大学"}; !slices.Equal(hitNames(hits), want) {
		t.Fatalf("Search = %v, want %v", hitNames(hits), want)
	}
}

func TestChineseNamesProduceTrigrams(t *testing.T) {
	f := setup(t)
	var trigrams []string
	if err := f.pool.QueryRow(context.Background(), `SELECT show_trgm('北京大学')`).Scan(&trigrams); err != nil {
		t.Fatalf("show_trgm: %v", err)
	}
	if len(trigrams) == 0 {
		t.Fatal("show_trgm('北京大学') is empty: the database locale does not treat CJK characters as word characters")
	}
}

func TestNearbyOrdersByDistanceWithinRadius(t *testing.T) {
	f := setup(t)
	repo := repository.NewPOIRepo(f.pool)

	near, err := repo.Nearby(context.Background(), 116.05, 40.05, 600, nil, 50)
	if err != nil {
		t.Fatalf("Nearby 600: %v", err)
	}
	if want := []string{"北京大学", "北京大学医学部"}; !slices.Equal(nearbyNames(near), want) {
		t.Fatalf("Nearby 600 = %v, want %v", nearbyNames(near), want)
	}
	if near[0].DistanceM != 0 || abs(near[1].DistanceM-500) > 1 {
		t.Errorf("distances = %d, %d; want 0 and about 500", near[0].DistanceM, near[1].DistanceM)
	}

	near, err = repo.Nearby(context.Background(), 116.05, 40.05, 1000, nil, 50)
	if err != nil {
		t.Fatalf("Nearby 1000: %v", err)
	}
	if want := []string{"北京大学", "北京大学医学部", "清华大学"}; !slices.Equal(nearbyNames(near), want) {
		t.Fatalf("Nearby 1000 = %v, want %v", nearbyNames(near), want)
	}
	if abs(near[2].DistanceM-999) > 1 {
		t.Errorf("distance to 清华大学 = %d, want about 999", near[2].DistanceM)
	}
}

func TestNearbyFiltersByCategoryAndLimit(t *testing.T) {
	f := setup(t)
	repo := repository.NewPOIRepo(f.pool)

	near, err := repo.Nearby(context.Background(), 116.15, 40.05, 3000, []int16{categoryID(t, f.pool, "park")}, 50)
	if err != nil {
		t.Fatalf("Nearby park: %v", err)
	}
	if want := []string{"测试公园", "100%纯净公园"}; !slices.Equal(nearbyNames(near), want) {
		t.Fatalf("Nearby park = %v, want %v", nearbyNames(near), want)
	}

	near, err = repo.Nearby(context.Background(), 116.15, 40.05, 3000, nil, 1)
	if err != nil {
		t.Fatalf("Nearby limit 1: %v", err)
	}
	if want := []string{"测试公园"}; !slices.Equal(nearbyNames(near), want) {
		t.Fatalf("Nearby limit 1 = %v, want %v", nearbyNames(near), want)
	}
}

func TestEmptyCategoryIDsMeanNoFilter(t *testing.T) {
	f := setup(t)
	repo := repository.NewPOIRepo(f.pool)
	ctx := context.Background()

	all, _, err := repo.InBBox(ctx, boxA, nil, 100)
	if err != nil {
		t.Fatalf("InBBox nil: %v", err)
	}
	empty, _, err := repo.InBBox(ctx, boxA, []int16{}, 100)
	if err != nil {
		t.Fatalf("InBBox empty: %v", err)
	}
	if !slices.Equal(summaryNames(empty), summaryNames(all)) {
		t.Errorf("InBBox with empty categories = %v, want %v", summaryNames(empty), summaryNames(all))
	}

	allNear, err := repo.Nearby(ctx, 116.05, 40.05, 1000, nil, 50)
	if err != nil {
		t.Fatalf("Nearby nil: %v", err)
	}
	emptyNear, err := repo.Nearby(ctx, 116.05, 40.05, 1000, []int16{}, 50)
	if err != nil {
		t.Fatalf("Nearby empty: %v", err)
	}
	if !slices.Equal(nearbyNames(emptyNear), nearbyNames(allNear)) {
		t.Errorf("Nearby with empty categories = %v, want %v", nearbyNames(emptyNear), nearbyNames(allNear))
	}
}

func TestQueriesCanUseTheirIndexes(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// 测试数据只有几行，规划器默认会选全表扫描；关掉顺序扫描后才能确认索引确实可用。
	if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
		t.Fatalf("disable seqscan: %v", err)
	}
	for name, tc := range map[string]struct{ query, index string }{
		"nearby": {`SELECT id FROM poi WHERE ST_DWithin(geom::geography, ST_SetSRID(ST_MakePoint(116.05, 40.05), 4326)::geography, 1000)`, "poi_geog_gist"},
		"bbox":   {`SELECT id FROM poi WHERE geom && ST_MakeEnvelope(116.0, 40.0, 116.1, 40.1, 4326)`, "poi_geom_gist"},
		"locate": {`SELECT adcode FROM district WHERE ST_Contains(geom, ST_SetSRID(ST_MakePoint(116.05, 40.05), 4326))`, "district_geom_gist"},
		"search": {`SELECT id FROM poi WHERE name ILIKE '%北京大学%'`, "poi_name_trgm"},
	} {
		rows, err := tx.Query(ctx, "EXPLAIN "+tc.query)
		if err != nil {
			t.Fatalf("explain %s: %v", name, err)
		}
		lines, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatalf("collect plan %s: %v", name, err)
		}
		if plan := strings.Join(lines, "\n"); !strings.Contains(plan, tc.index) {
			t.Errorf("%s query does not use %s:\n%s", name, tc.index, plan)
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
