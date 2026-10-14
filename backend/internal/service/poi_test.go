package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

type fakePOIRepo struct {
	summaries   []model.POISummary
	truncated   bool
	detail      model.POIDetail
	hits        []model.SearchHit
	nearby      []model.NearbyPOI
	err         error
	calls       int
	gotBox      model.BBox
	gotIDs      []int16
	gotLimit    int
	gotQuery    string
	gotRadius   float64
	gotLngLat   [2]float64
	gotDetailID int64
}

func (f *fakePOIRepo) InBBox(_ context.Context, box model.BBox, ids []int16, limit int) ([]model.POISummary, bool, error) {
	f.calls++
	f.gotBox, f.gotIDs, f.gotLimit = box, ids, limit
	return f.summaries, f.truncated, f.err
}

func (f *fakePOIRepo) Get(_ context.Context, id int64) (model.POIDetail, error) {
	f.calls++
	f.gotDetailID = id
	return f.detail, f.err
}

func (f *fakePOIRepo) Search(_ context.Context, q string, limit int) ([]model.SearchHit, error) {
	f.calls++
	f.gotQuery, f.gotLimit = q, limit
	return f.hits, f.err
}

func (f *fakePOIRepo) Nearby(_ context.Context, lng, lat, radiusM float64, ids []int16, limit int) ([]model.NearbyPOI, error) {
	f.calls++
	f.gotLngLat, f.gotRadius, f.gotIDs, f.gotLimit = [2]float64{lng, lat}, radiusM, ids, limit
	return f.nearby, f.err
}

func newTestPOIService(repo *fakePOIRepo) *POIService {
	return NewPOIService(repo, NewCatalog(seededLister()))
}

func TestPOIInBBoxBuildsFeatureCollection(t *testing.T) {
	repo := &fakePOIRepo{
		summaries: []model.POISummary{{ID: 7, Name: "颐和园", Category: "park", Location: [2]float64{116.27, 39.999}}},
		truncated: true,
	}
	box := model.BBox{MinLng: 116, MinLat: 39.8, MaxLng: 116.6, MaxLat: 40.1}

	got, err := newTestPOIService(repo).InBBox(context.Background(), BBoxQuery{Box: box, Categories: []string{"park"}, Limit: 100})
	if err != nil {
		t.Fatalf("InBBox: %v", err)
	}
	if repo.gotBox != box || !slices.Equal(repo.gotIDs, []int16{5}) || repo.gotLimit != 100 {
		t.Fatalf("repo got box=%v ids=%v limit=%d", repo.gotBox, repo.gotIDs, repo.gotLimit)
	}
	body, _ := json.Marshal(got)
	want := `{"type":"FeatureCollection","features":[{"type":"Feature","id":7,"geometry":{"type":"Point","coordinates":[116.27,39.999]},` +
		`"properties":{"id":7,"name":"颐和园","category":"park"}}],"truncated":true}`
	if string(body) != want {
		t.Fatalf("JSON =\n%s\nwant\n%s", body, want)
	}
}

func TestPOIInBBoxWithoutCategoriesPassesNil(t *testing.T) {
	repo := &fakePOIRepo{}
	got, err := newTestPOIService(repo).InBBox(context.Background(), BBoxQuery{Limit: 10})
	if err != nil {
		t.Fatalf("InBBox: %v", err)
	}
	if repo.gotIDs != nil {
		t.Fatalf("repo got ids %v, want nil", repo.gotIDs)
	}
	body, _ := json.Marshal(got)
	if string(body) != `{"type":"FeatureCollection","features":[],"truncated":false}` {
		t.Fatalf("JSON = %s", body)
	}
}

func TestPOIInBBoxRejectsUnknownCategoryBeforeQuerying(t *testing.T) {
	repo := &fakePOIRepo{}
	_, err := newTestPOIService(repo).InBBox(context.Background(), BBoxQuery{Categories: []string{"zoo"}, Limit: 10})
	var unknown UnknownCategoryError
	if !errors.As(err, &unknown) || unknown.Code != "zoo" {
		t.Fatalf("err = %v, want UnknownCategoryError{zoo}", err)
	}
	if repo.calls != 0 {
		t.Fatalf("repository called %d times, want 0", repo.calls)
	}
}

func TestPOIGetBuildsDetailWithOSMURL(t *testing.T) {
	nameEn, address := "Peking University", "颐和园路5号"
	for osmType, element := range map[string]string{"n": "node", "w": "way", "r": "relation"} {
		repo := &fakePOIRepo{detail: model.POIDetail{
			ID: 3, Name: "北京大学", NameEn: &nameEn, OSMType: osmType, OSMID: 123,
			Category: model.Category{ID: 2, Code: "university", Name: "高校", Color: "#1E88E5"},
			District: &model.DistrictRef{Adcode: "110108", Name: "海淀区"},
			Location: [2]float64{116.305, 39.99}, Address: &address, Tags: map[string]string{"website": "https://www.pku.edu.cn"},
		}}

		got, err := newTestPOIService(repo).Get(context.Background(), 3)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.OSMURL != "https://www.openstreetmap.org/"+element+"/123" {
			t.Errorf("osm_url for %s = %q", osmType, got.OSMURL)
		}
		if repo.gotDetailID != 3 || got.Category != (Category{Code: "university", Name: "高校", Color: "#1E88E5"}) ||
			got.District == nil || *got.District != (DistrictRef{Adcode: "110108", Name: "海淀区"}) ||
			*got.NameEn != nameEn || *got.Address != address || got.Tags["website"] != "https://www.pku.edu.cn" {
			t.Errorf("detail = %+v", got)
		}
	}
}

func TestPOIGetWithoutDistrictAndNotFound(t *testing.T) {
	got, err := newTestPOIService(&fakePOIRepo{detail: model.POIDetail{ID: 9, OSMType: "n", OSMID: 1}}).Get(context.Background(), 9)
	if err != nil || got.District != nil {
		t.Fatalf("Get = %+v, %v; want nil district", got, err)
	}
	_, err = newTestPOIService(&fakePOIRepo{err: ErrNotFound}).Get(context.Background(), 1)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPOISearchMapsHits(t *testing.T) {
	district := "海淀区"
	repo := &fakePOIRepo{hits: []model.SearchHit{{
		POISummary:   model.POISummary{ID: 3, Name: "北京大学", Category: "university", Location: [2]float64{116.305, 39.99}},
		DistrictName: &district,
	}}}

	got, err := newTestPOIService(repo).Search(context.Background(), "北京大学", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	body, _ := json.Marshal(got)
	want := `[{"id":3,"name":"北京大学","category":"university","district_name":"海淀区","location":[116.305,39.99]}]`
	if string(body) != want || repo.gotQuery != "北京大学" || repo.gotLimit != 10 {
		t.Fatalf("JSON = %s (repo got %q, %d)", body, repo.gotQuery, repo.gotLimit)
	}
}

func TestPOISearchEncodesNoHitsAsEmptyArray(t *testing.T) {
	got, err := newTestPOIService(&fakePOIRepo{}).Search(context.Background(), "x", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if body, _ := json.Marshal(got); string(body) != `[]` {
		t.Fatalf("JSON = %s, want []", body)
	}
}

func TestPOINearbyEchoesCenterAndRadius(t *testing.T) {
	repo := &fakePOIRepo{nearby: []model.NearbyPOI{{
		POISummary: model.POISummary{ID: 3, Name: "北京大学", Category: "university", Location: [2]float64{116.305, 39.99}},
		DistanceM:  320,
	}}}

	got, err := newTestPOIService(repo).Nearby(context.Background(), NearbyQuery{
		Lng: 116.3, Lat: 39.99, RadiusM: 1000, Categories: []string{"subway_station"}, Limit: 50,
	})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	if repo.gotLngLat != [2]float64{116.3, 39.99} || repo.gotRadius != 1000 || !slices.Equal(repo.gotIDs, []int16{1}) || repo.gotLimit != 50 {
		t.Fatalf("repo got %v %v %v %d", repo.gotLngLat, repo.gotRadius, repo.gotIDs, repo.gotLimit)
	}
	body, _ := json.Marshal(got)
	want := `{"type":"FeatureCollection","features":[{"type":"Feature","id":3,"geometry":{"type":"Point","coordinates":[116.305,39.99]},` +
		`"properties":{"id":3,"name":"北京大学","category":"university","distance_m":320}}],"center":[116.3,39.99],"radius":1000}`
	if string(body) != want {
		t.Fatalf("JSON =\n%s\nwant\n%s", body, want)
	}
}

func TestPOICategoriesComeFromCatalog(t *testing.T) {
	got, err := newTestPOIService(&fakePOIRepo{}).Categories(context.Background())
	if err != nil || len(got) != 2 || got[0].Code != "subway_station" {
		t.Fatalf("Categories = %+v, %v", got, err)
	}
}
