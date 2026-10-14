package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/service"
)

func TestListDistrictsWritesGeoJSON(t *testing.T) {
	d := &fakeDistrictService{collection: service.DistrictCollection{Type: "FeatureCollection", Features: []service.DistrictFeature{{
		Type:       "Feature",
		Geometry:   json.RawMessage(`{"type":"MultiPolygon","coordinates":[]}`),
		Properties: service.DistrictProperties{Adcode: "110108", Name: "海淀区"},
	}}}}

	rec := get(t, newTestRouter(d, &fakePOIService{}), "/api/v1/districts", nil)

	res := rec.Result()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/geo+json" {
		t.Fatalf("status %d, Content-Type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	body := decodeJSON(t, rec.Body.Bytes())
	features, _ := body["features"].([]any)
	if body["type"] != "FeatureCollection" || len(features) != 1 {
		t.Fatalf("body = %s", rec.Body.String())
	}
	props := features[0].(map[string]any)["properties"].(map[string]any)
	if props["adcode"] != "110108" {
		t.Fatalf("properties = %v", props)
	}
}

func TestGetDistrictReturnsDetail(t *testing.T) {
	d := &fakeDistrictService{detail: service.DistrictDetail{
		Adcode: "110108", Name: "海淀区", POICounts: []service.CategoryCount{{Code: "park", Name: "公园", Count: 3}},
	}}

	rec := get(t, newTestRouter(d, &fakePOIService{}), "/api/v1/districts/110108", nil)

	if rec.Code != http.StatusOK || d.gotAdcode != "110108" {
		t.Fatalf("status %d, service got %q", rec.Code, d.gotAdcode)
	}
	data := decodeJSON(t, rec.Body.Bytes())["data"].(map[string]any)
	if data["name"] != "海淀区" || len(data["poi_counts"].([]any)) != 1 {
		t.Fatalf("data = %v", data)
	}
}

func TestGetDistrictRejectsInvalidAdcode(t *testing.T) {
	d := &fakeDistrictService{}
	rec := get(t, newTestRouter(d, &fakePOIService{}), "/api/v1/districts/11010", nil)

	assertErrorResponse(t, rec, http.StatusBadRequest, CodeInvalidParam, "adcode 必须是 6 位数字")
	if d.calls != 0 {
		t.Fatalf("service called %d times, want 0", d.calls)
	}
}

func TestGetDistrictNotFound(t *testing.T) {
	d := &fakeDistrictService{err: service.ErrNotFound}
	rec := get(t, newTestRouter(d, &fakePOIService{}), "/api/v1/districts/999999", nil)

	assertErrorResponse(t, rec, http.StatusNotFound, CodeNotFound, "行政区不存在")
}

func TestLocateDistrictReturnsDistrict(t *testing.T) {
	d := &fakeDistrictService{ref: &service.DistrictRef{Adcode: "110108", Name: "海淀区"}}

	rec := get(t, newTestRouter(d, &fakePOIService{}), "/api/v1/districts/locate?lng=116.3&lat=39.99", nil)

	if rec.Code != http.StatusOK || d.gotLngLat != [2]float64{116.3, 39.99} {
		t.Fatalf("status %d, service got %v", rec.Code, d.gotLngLat)
	}
	if rec.Body.String() != `{"data":{"adcode":"110108","name":"海淀区"}}` {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestLocateDistrictOutsideReturnsNullData(t *testing.T) {
	rec := get(t, newTestRouter(&fakeDistrictService{}, &fakePOIService{}), "/api/v1/districts/locate?lng=117&lat=41", nil)

	if rec.Code != http.StatusOK || rec.Body.String() != `{"data":null}` {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestLocateDistrictValidatesCoordinates(t *testing.T) {
	d := &fakeDistrictService{}
	r := newTestRouter(d, &fakePOIService{})
	for query, want := range map[string]string{
		"?lat=39.9":          "缺少参数 lng",
		"?lng=NaN&lat=39.9":  "lng 必须是有限的数字",
		"?lng=116&lat=91":    "lat 必须在 -90 到 90 之间",
		"?lng=116.3&lat=abc": "lat 必须是有限的数字",
	} {
		rec := get(t, r, "/api/v1/districts/locate"+query, nil)
		assertErrorResponse(t, rec, http.StatusBadRequest, CodeInvalidParam, want)
	}
	if d.calls != 0 {
		t.Fatalf("service called %d times, want 0", d.calls)
	}
}

func TestServiceFailuresBecomeInternalErrors(t *testing.T) {
	for name, err := range map[string]error{"unexpected": errors.New("db down"), "not found without message": service.ErrNotFound} {
		t.Run(name, func(t *testing.T) {
			d := &fakeDistrictService{err: err}
			rec := get(t, newTestRouter(d, &fakePOIService{}), "/api/v1/districts", nil)
			assertErrorResponse(t, rec, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
		})
	}
}

func TestListCategoriesReturnsData(t *testing.T) {
	p := &fakePOIService{categories: []service.Category{{Code: "park", Name: "公园", Color: "#43A047"}}}

	rec := get(t, newTestRouter(&fakeDistrictService{}, p), "/api/v1/poi-categories", nil)

	if rec.Code != http.StatusOK || rec.Body.String() != `{"data":[{"code":"park","name":"公园","color":"#43A047"}]}` {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestParamErrorsSurviveGzip(t *testing.T) {
	rec := get(t, newTestRouter(&fakeDistrictService{}, &fakePOIService{}), "/api/v1/districts/abc",
		http.Header{"Accept-Encoding": {"gzip"}})

	assertErrorResponse(t, rec, http.StatusBadRequest, CodeInvalidParam, "adcode 必须是 6 位数字")
}
