//go:build integration

package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/handler"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/importer"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/repository"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/service"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/testdb"
)

type apiFixture struct {
	router http.Handler
	n1001  int64
}

// newAPI 用导入器的测试快照（两个正方形区、6 个 POI）搭建真实的 handler -> service -> repository 链路。
func newAPI(t *testing.T) apiFixture {
	t.Helper()
	ctx := context.Background()
	pool := testdb.New(t, "api")
	if _, err := importer.Run(ctx, pool, importer.Options{DataDir: filepath.Join("..", "importer", "testdata"), MinPOIs: 1}); err != nil {
		t.Fatalf("import fixture: %v", err)
	}
	var n1001 int64
	if err := pool.QueryRow(ctx, `SELECT id FROM poi WHERE osm_type = 'n' AND osm_id = 1001`).Scan(&n1001); err != nil {
		t.Fatalf("find n1001: %v", err)
	}
	router := handler.NewRouter(handler.Deps{
		DB:        pool,
		Districts: service.NewDistrictService(repository.NewDistrictRepo(pool)),
		POIs:      service.NewPOIService(repository.NewPOIRepo(pool), service.NewCatalog(repository.NewCategoryRepo(pool))),
	})
	return apiFixture{router: router, n1001: n1001}
}

func call(t *testing.T, h http.Handler, path string) (int, string, []byte) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	res := rec.Result()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return res.StatusCode, res.Header.Get("Content-Type"), body
}

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return v
}

func TestAPIDistrictEndpoints(t *testing.T) {
	api := newAPI(t)

	if status, _, body := call(t, api.router, "/api/v1/health"); status != http.StatusOK {
		t.Fatalf("health: %d %s", status, body)
	}

	status, contentType, body := call(t, api.router, "/api/v1/districts")
	districts := decode[service.DistrictCollection](t, body)
	if status != http.StatusOK || contentType != "application/geo+json" || len(districts.Features) != 2 {
		t.Fatalf("districts: %d %q %s", status, contentType, body)
	}
	a, b := districts.Features[0].Properties, districts.Features[1].Properties
	if a.Adcode != "990001" || a.POICount != 3 || b.Adcode != "990002" || b.POICount != 2 {
		t.Fatalf("district properties = %+v, %+v", a, b)
	}
	if geometry := decode[struct {
		Type string `json:"type"`
	}](t, districts.Features[0].Geometry); geometry.Type != "MultiPolygon" {
		t.Fatalf("geometry type = %q", geometry.Type)
	}

	status, _, body = call(t, api.router, "/api/v1/districts/990001")
	detail := decode[struct {
		Data service.DistrictDetail `json:"data"`
	}](t, body).Data
	counts := map[string]int{}
	for _, c := range detail.POICounts {
		counts[c.Code] = c.Count
	}
	if status != http.StatusOK || len(detail.POICounts) != 5 || counts["university"] != 1 || counts["hospital"] != 1 ||
		counts["subway_station"] != 1 || counts["park"] != 0 {
		t.Fatalf("district detail: %d %s", status, body)
	}

	if status, _, body := call(t, api.router, "/api/v1/districts/999999"); status != http.StatusNotFound {
		t.Fatalf("unknown district: %d %s", status, body)
	}
}

func TestAPILocate(t *testing.T) {
	api := newAPI(t)

	status, _, body := call(t, api.router, "/api/v1/districts/locate?lng=116.05&lat=40.05")
	ref := decode[struct {
		Data *service.DistrictRef `json:"data"`
	}](t, body).Data
	if status != http.StatusOK || ref == nil || ref.Adcode != "990001" {
		t.Fatalf("locate inside: %d %s", status, body)
	}
	if _, _, body := call(t, api.router, "/api/v1/districts/locate?lng=117&lat=41"); string(body) != `{"data":null}` {
		t.Fatalf("locate outside: %s", body)
	}
	if status, _, body := call(t, api.router, "/api/v1/districts/locate?lng=abc&lat=1"); status != http.StatusBadRequest {
		t.Fatalf("locate invalid: %d %s", status, body)
	}
}

func TestAPICategories(t *testing.T) {
	api := newAPI(t)

	status, _, body := call(t, api.router, "/api/v1/poi-categories")
	categories := decode[struct {
		Data []service.Category `json:"data"`
	}](t, body).Data
	if status != http.StatusOK || len(categories) != 5 || categories[0].Code != "subway_station" {
		t.Fatalf("categories: %d %s", status, body)
	}
}

func TestAPIPOIsInBBox(t *testing.T) {
	api := newAPI(t)
	const bboxPath = "/api/v1/pois?bbox=115.9,39.9,116.3,40.2"

	status, contentType, body := call(t, api.router, bboxPath)
	pois := decode[service.POICollection](t, body)
	if status != http.StatusOK || contentType != "application/geo+json" || len(pois.Features) != 5 || pois.Truncated {
		t.Fatalf("bbox: %d %q %s", status, contentType, body)
	}
	for _, f := range pois.Features {
		if f.Geometry.Type != "Point" || f.ID != f.Properties.ID {
			t.Fatalf("feature = %+v", f)
		}
	}

	_, _, body = call(t, api.router, bboxPath+"&limit=2")
	if pois := decode[service.POICollection](t, body); len(pois.Features) != 2 || !pois.Truncated {
		t.Fatalf("bbox limit 2: %s", body)
	}

	_, _, body = call(t, api.router, bboxPath+"&categories=park")
	if pois := decode[service.POICollection](t, body); len(pois.Features) != 1 || pois.Features[0].Properties.Name != "乙区公园" {
		t.Fatalf("bbox park: %s", body)
	}

	status, _, body = call(t, api.router, bboxPath+"&categories=zoo")
	if status != http.StatusBadRequest || !strings.Contains(string(body), "未知的分类编码：zoo") {
		t.Fatalf("bbox unknown category: %d %s", status, body)
	}
}

func TestAPIPOIDetail(t *testing.T) {
	api := newAPI(t)

	status, _, body := call(t, api.router, fmt.Sprintf("/api/v1/pois/%d", api.n1001))
	detail := decode[struct {
		Data service.POIDetail `json:"data"`
	}](t, body).Data
	if status != http.StatusOK || detail.Name != "测试大学" || detail.NameEn == nil || *detail.NameEn != "Test University" ||
		detail.OSMURL != "https://www.openstreetmap.org/node/1001" || detail.District == nil || detail.District.Adcode != "990001" ||
		detail.Category.Code != "university" || detail.Address == nil || *detail.Address != "测试路1号" ||
		detail.Location != [2]float64{116.05, 40.05} {
		t.Fatalf("poi detail: %d %s", status, body)
	}

	if status, _, body := call(t, api.router, "/api/v1/pois/999999999"); status != http.StatusNotFound {
		t.Fatalf("unknown poi: %d %s", status, body)
	}
}

func TestAPISearch(t *testing.T) {
	api := newAPI(t)

	status, _, body := call(t, api.router, "/api/v1/pois/search?q="+url.QueryEscape("测试"))
	hits := decode[struct {
		Data []service.SearchHit `json:"data"`
	}](t, body).Data
	found := map[string]bool{}
	for _, h := range hits {
		found[h.Name] = true
	}
	if status != http.StatusOK || len(hits) != 4 || !found["测试大学"] || !found["测试园林"] || !found["测试医院"] || !found["测试站"] {
		t.Fatalf("search 测试: %d %s", status, body)
	}

	_, _, body = call(t, api.router, "/api/v1/pois/search?q="+url.QueryEscape("测试大学"))
	hits = decode[struct {
		Data []service.SearchHit `json:"data"`
	}](t, body).Data
	if len(hits) == 0 || hits[0].Name != "测试大学" || hits[0].DistrictName == nil || *hits[0].DistrictName != "测试甲区" {
		t.Fatalf("search 测试大学: %s", body)
	}
}

func TestAPINearby(t *testing.T) {
	api := newAPI(t)

	status, contentType, body := call(t, api.router, "/api/v1/pois/nearby?lng=116.05&lat=40.05&radius=1000")
	nearby := decode[service.NearbyCollection](t, body)
	if status != http.StatusOK || contentType != "application/geo+json" || len(nearby.Features) != 1 ||
		nearby.Features[0].Properties.Name != "测试大学" || nearby.Features[0].Properties.DistanceM != 0 ||
		nearby.Center != [2]float64{116.05, 40.05} || nearby.Radius != 1000 {
		t.Fatalf("nearby: %d %q %s", status, contentType, body)
	}
}
