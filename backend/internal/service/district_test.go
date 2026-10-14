package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

type fakeDistrictRepo struct {
	features  []model.DistrictFeature
	detail    model.DistrictDetail
	ref       *model.DistrictRef
	err       error
	gotAdcode string
	gotLngLat [2]float64
}

func (f *fakeDistrictRepo) List(context.Context) ([]model.DistrictFeature, error) {
	return f.features, f.err
}

func (f *fakeDistrictRepo) Get(_ context.Context, adcode string) (model.DistrictDetail, error) {
	f.gotAdcode = adcode
	return f.detail, f.err
}

func (f *fakeDistrictRepo) Locate(_ context.Context, lng, lat float64) (*model.DistrictRef, error) {
	f.gotLngLat = [2]float64{lng, lat}
	return f.ref, f.err
}

func TestDistrictListBuildsFeatureCollection(t *testing.T) {
	geometry := json.RawMessage(`{"type":"MultiPolygon","coordinates":[]}`)
	repo := &fakeDistrictRepo{features: []model.DistrictFeature{
		{Adcode: "110108", Name: "海淀区", AreaKm2: 430.95, POICount: 812, Center: [2]float64{116.2, 40.0}, Geometry: geometry},
	}}

	got, err := NewDistrictService(repo).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"MultiPolygon","coordinates":[]},` +
		`"properties":{"adcode":"110108","name":"海淀区","area_km2":430.95,"poi_count":812,"center":[116.2,40]}}]}`
	if string(body) != want {
		t.Fatalf("JSON =\n%s\nwant\n%s", body, want)
	}
}

func TestDistrictListEncodesEmptyFeaturesAsArray(t *testing.T) {
	got, err := NewDistrictService(&fakeDistrictRepo{}).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	body, _ := json.Marshal(got)
	if string(body) != `{"type":"FeatureCollection","features":[]}` {
		t.Fatalf("JSON = %s, want an empty features array", body)
	}
}

func TestDistrictGetMapsCounts(t *testing.T) {
	repo := &fakeDistrictRepo{detail: model.DistrictDetail{
		Adcode: "110108", Name: "海淀区", AreaKm2: 430.95, Center: [2]float64{116.2, 40.0},
		POICounts: []model.CategoryCount{{Code: "university", Name: "高校", Count: 30}},
	}}

	got, err := NewDistrictService(repo).Get(context.Background(), "110108")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if repo.gotAdcode != "110108" || got.Name != "海淀区" || len(got.POICounts) != 1 ||
		got.POICounts[0] != (CategoryCount{Code: "university", Name: "高校", Count: 30}) {
		t.Fatalf("Get = %+v (repo got %q)", got, repo.gotAdcode)
	}
}

func TestDistrictGetPassesNotFoundThrough(t *testing.T) {
	_, err := NewDistrictService(&fakeDistrictRepo{err: ErrNotFound}).Get(context.Background(), "999999")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDistrictLocateMapsResultAndNil(t *testing.T) {
	repo := &fakeDistrictRepo{ref: &model.DistrictRef{Adcode: "110108", Name: "海淀区"}}
	got, err := NewDistrictService(repo).Locate(context.Background(), 116.3, 39.99)
	if err != nil || got == nil || *got != (DistrictRef{Adcode: "110108", Name: "海淀区"}) || repo.gotLngLat != [2]float64{116.3, 39.99} {
		t.Fatalf("Locate = %+v, %v (repo got %v)", got, err, repo.gotLngLat)
	}

	got, err = NewDistrictService(&fakeDistrictRepo{}).Locate(context.Background(), 117, 41)
	if err != nil || got != nil {
		t.Fatalf("Locate outside = %+v, %v; want nil, nil", got, err)
	}
}
