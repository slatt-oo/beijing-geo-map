package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/service"
)

func TestPOIsInBBoxWritesGeoJSONWithDefaults(t *testing.T) {
	p := &fakePOIService{collection: service.POICollection{Type: "FeatureCollection", Features: []service.POIFeature{}, Truncated: false}}

	rec := get(t, newTestRouter(&fakeDistrictService{}, p), "/api/v1/pois?bbox=116,39.8,116.6,40.1", nil)

	res := rec.Result()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/geo+json" {
		t.Fatalf("status %d, Content-Type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	want := service.BBoxQuery{Box: model.BBox{MinLng: 116, MinLat: 39.8, MaxLng: 116.6, MaxLat: 40.1}, Limit: service.DefaultBBoxLimit}
	if p.gotBBox.Box != want.Box || p.gotBBox.Limit != want.Limit || p.gotBBox.Categories != nil {
		t.Fatalf("service got %+v, want %+v", p.gotBBox, want)
	}
	if rec.Body.String() != `{"type":"FeatureCollection","features":[],"truncated":false}` {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestPOIsInBBoxPassesCategoriesAndLimit(t *testing.T) {
	p := &fakePOIService{}

	get(t, newTestRouter(&fakeDistrictService{}, p), "/api/v1/pois?bbox=116,39.8,116.6,40.1&categories=park,%20university&limit=500", nil)

	if !slices.Equal(p.gotBBox.Categories, []string{"park", "university"}) || p.gotBBox.Limit != 500 {
		t.Fatalf("service got %+v", p.gotBBox)
	}
}

func TestPOIsInBBoxValidatesParams(t *testing.T) {
	p := &fakePOIService{}
	r := newTestRouter(&fakeDistrictService{}, p)
	for query, want := range map[string]string{
		"":                                     "缺少参数 bbox",
		"?bbox=1,2,3":                          "bbox 格式应为 minLng,minLat,maxLng,maxLat",
		"?bbox=116,39.8,116.6,40.1&limit=0":    "limit 必须是 1 到 5000 之间的整数",
		"?bbox=116,39.8,116.6,40.1&limit=5001": "limit 必须是 1 到 5000 之间的整数",
	} {
		rec := get(t, r, "/api/v1/pois"+query, nil)
		assertErrorResponse(t, rec, http.StatusBadRequest, CodeInvalidParam, want)
	}
	if p.calls != 0 {
		t.Fatalf("service called %d times, want 0", p.calls)
	}
}

func TestPOIsInBBoxUnknownCategoryIsBadRequest(t *testing.T) {
	p := &fakePOIService{err: service.UnknownCategoryError{Code: "zoo"}}

	rec := get(t, newTestRouter(&fakeDistrictService{}, p), "/api/v1/pois?bbox=116,39.8,116.6,40.1&categories=zoo", nil)

	assertErrorResponse(t, rec, http.StatusBadRequest, CodeInvalidParam, "未知的分类编码：zoo")
}

func TestGetPOIReturnsDetail(t *testing.T) {
	p := &fakePOIService{detail: service.POIDetail{ID: 42, Name: "北京大学", OSMURL: "https://www.openstreetmap.org/way/1"}}

	rec := get(t, newTestRouter(&fakeDistrictService{}, p), "/api/v1/pois/42", nil)

	if rec.Code != http.StatusOK || p.gotID != 42 {
		t.Fatalf("status %d, service got %d", rec.Code, p.gotID)
	}
	data := decodeJSON(t, rec.Body.Bytes())["data"].(map[string]any)
	if data["name"] != "北京大学" || data["osm_url"] != "https://www.openstreetmap.org/way/1" || data["district"] != nil {
		t.Fatalf("data = %v", data)
	}
}

func TestGetPOIValidatesID(t *testing.T) {
	p := &fakePOIService{}
	r := newTestRouter(&fakeDistrictService{}, p)
	for _, id := range []string{"0", "abc", "-5"} {
		rec := get(t, r, "/api/v1/pois/"+id, nil)
		assertErrorResponse(t, rec, http.StatusBadRequest, CodeInvalidParam, "id 必须是正整数")
	}
	if p.calls != 0 {
		t.Fatalf("service called %d times, want 0", p.calls)
	}
}

func TestGetPOINotFound(t *testing.T) {
	rec := get(t, newTestRouter(&fakeDistrictService{}, &fakePOIService{err: service.ErrNotFound}), "/api/v1/pois/99", nil)

	assertErrorResponse(t, rec, http.StatusNotFound, CodeNotFound, "POI 不存在")
}

func TestSearchPOIsUsesTrimmedQueryAndDefaultLimit(t *testing.T) {
	p := &fakePOIService{hits: []service.SearchHit{{ID: 3, Name: "北京大学", Category: "university"}}}

	rec := get(t, newTestRouter(&fakeDistrictService{}, p), "/api/v1/pois/search?q="+url.QueryEscape(" 北京大学 "), nil)

	if rec.Code != http.StatusOK || p.gotQuery != "北京大学" || p.gotLimit != service.DefaultSearchLimit {
		t.Fatalf("status %d, service got %q / %d", rec.Code, p.gotQuery, p.gotLimit)
	}
	hits := decodeJSON(t, rec.Body.Bytes())["data"].([]any)
	if len(hits) != 1 || hits[0].(map[string]any)["name"] != "北京大学" {
		t.Fatalf("data = %v", hits)
	}
}

func TestSearchPOIsValidatesParams(t *testing.T) {
	p := &fakePOIService{}
	r := newTestRouter(&fakeDistrictService{}, p)
	for query, want := range map[string]string{
		"": "缺少参数 q",
		"?q=" + url.QueryEscape(strings.Repeat("北", 51)): "q 不能超过 50 个字符",
		"?q=a%00b":        "q 包含非法字符",
		"?q=abc&limit=21": "limit 必须是 1 到 20 之间的整数",
	} {
		rec := get(t, r, "/api/v1/pois/search"+query, nil)
		assertErrorResponse(t, rec, http.StatusBadRequest, CodeInvalidParam, want)
	}
	if p.calls != 0 {
		t.Fatalf("service called %d times, want 0", p.calls)
	}
}

func TestNearbyPOIsAppliesDefaults(t *testing.T) {
	p := &fakePOIService{nearby: service.NearbyCollection{Type: "FeatureCollection", Features: []service.NearbyFeature{}, Center: [2]float64{116.3, 39.99}, Radius: 1000}}

	rec := get(t, newTestRouter(&fakeDistrictService{}, p), "/api/v1/pois/nearby?lng=116.3&lat=39.99", nil)

	res := rec.Result()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/geo+json" {
		t.Fatalf("status %d, Content-Type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	got := p.gotNearby
	if got.Lng != 116.3 || got.Lat != 39.99 || got.RadiusM != service.DefaultNearbyRadiusM || got.Limit != service.DefaultNearbyLimit || got.Categories != nil {
		t.Fatalf("service got %+v", got)
	}
}

func TestNearbyPOIsPassesRadiusCategoriesAndLimit(t *testing.T) {
	p := &fakePOIService{}

	get(t, newTestRouter(&fakeDistrictService{}, p), "/api/v1/pois/nearby?lng=116.3&lat=39.99&radius=2000&categories=park&limit=10", nil)

	if got := p.gotNearby; got.RadiusM != 2000 || got.Limit != 10 || !slices.Equal(got.Categories, []string{"park"}) {
		t.Fatalf("service got %+v", got)
	}
}

func TestNearbyPOIsValidatesParams(t *testing.T) {
	p := &fakePOIService{}
	r := newTestRouter(&fakeDistrictService{}, p)
	for query, want := range map[string]string{
		"?lat=39.99":                      "缺少参数 lng",
		"?lng=116.3&lat=39.99&radius=50":  "radius 必须在 100 到 5000 之间",
		"?lng=116.3&lat=39.99&radius=Inf": "radius 必须是有限的数字",
		"?lng=116.3&lat=39.99&limit=101":  "limit 必须是 1 到 100 之间的整数",
	} {
		rec := get(t, r, "/api/v1/pois/nearby"+query, nil)
		assertErrorResponse(t, rec, http.StatusBadRequest, CodeInvalidParam, want)
	}
	if p.calls != 0 {
		t.Fatalf("service called %d times, want 0", p.calls)
	}
}

func TestLocationQueriesAreRedactedInAccessLog(t *testing.T) {
	logs := captureLogs(t)
	r := newTestRouter(&fakeDistrictService{}, &fakePOIService{})

	get(t, r, "/api/v1/pois/nearby?lng=116.391&lat=39.907", nil)
	get(t, r, "/api/v1/districts/locate?lng=116.391&lat=39.907", nil)
	get(t, r, "/api/v1/pois/search?q=abc", nil)

	if strings.Contains(logs.String(), "116.391") || strings.Count(logs.String(), `"query":"[redacted]"`) != 2 {
		t.Fatalf("location must be redacted from the access log:\n%s", logs)
	}
	if !strings.Contains(logs.String(), `"query":"q=abc"`) {
		t.Fatalf("other queries must stay in the access log:\n%s", logs)
	}
}

func TestSuccessfulAPIRequestsAreLoggedAtInfo(t *testing.T) {
	logs := captureLogs(t)

	get(t, newTestRouter(&fakeDistrictService{}, &fakePOIService{}), "/api/v1/poi-categories", nil)

	if !strings.Contains(logs.String(), `"path":"/api/v1/poi-categories"`) || !strings.Contains(logs.String(), `"status":200`) {
		t.Fatalf("successful API request must be logged at info level:\n%s", logs)
	}
}

func TestClientCanceledRequestsAreNotLoggedAsErrors(t *testing.T) {
	logs := captureLogs(t)
	r := newTestRouter(&fakeDistrictService{}, &fakePOIService{err: context.Canceled})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/pois/42", nil).WithContext(ctx))

	if rec.Code != statusClientClosedRequest {
		t.Fatalf("status = %d, want %d", rec.Code, statusClientClosedRequest)
	}
	if strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Fatalf("a client cancellation must not be logged as an error:\n%s", logs)
	}
	if !strings.Contains(logs.String(), `"status":499`) {
		t.Fatalf("the access log must record the cancellation as 499:\n%s", logs)
	}
}

func TestRequestDeadlinesAreInternalErrors(t *testing.T) {
	for _, serviceErr := range []error{context.DeadlineExceeded, context.Canceled} {
		t.Run(serviceErr.Error(), func(t *testing.T) {
			logs := captureLogs(t)
			r := newTestRouter(&fakeDistrictService{}, &fakePOIService{err: serviceErr})
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancel()
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/pois/42", nil).WithContext(ctx))

			assertErrorResponse(t, rec, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
			if !strings.Contains(logs.String(), `"level":"ERROR"`) {
				t.Fatalf("a request that ran out of time must be logged as an error:\n%s", logs)
			}
		})
	}
}

func TestCanceledErrorsWithoutClientCancellationStayInternal(t *testing.T) {
	logs := captureLogs(t)

	rec := get(t, newTestRouter(&fakeDistrictService{}, &fakePOIService{err: context.Canceled}), "/api/v1/pois/42", nil)

	assertErrorResponse(t, rec, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	if !strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Fatalf("a cancellation the client did not cause must be logged as an error:\n%s", logs)
	}
}
