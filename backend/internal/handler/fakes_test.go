package handler

import (
	"context"
	"net/http"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/service"
)

type fakeDistrictService struct {
	collection service.DistrictCollection
	detail     service.DistrictDetail
	ref        *service.DistrictRef
	err        error
	calls      int
	gotAdcode  string
	gotLngLat  [2]float64
}

func (f *fakeDistrictService) List(context.Context) (service.DistrictCollection, error) {
	f.calls++
	return f.collection, f.err
}

func (f *fakeDistrictService) Get(_ context.Context, adcode string) (service.DistrictDetail, error) {
	f.calls++
	f.gotAdcode = adcode
	return f.detail, f.err
}

func (f *fakeDistrictService) Locate(_ context.Context, lng, lat float64) (*service.DistrictRef, error) {
	f.calls++
	f.gotLngLat = [2]float64{lng, lat}
	return f.ref, f.err
}

type fakePOIService struct {
	categories []service.Category
	collection service.POICollection
	detail     service.POIDetail
	hits       []service.SearchHit
	nearby     service.NearbyCollection
	err        error
	calls      int
	gotBBox    service.BBoxQuery
	gotID      int64
	gotQuery   string
	gotLimit   int
	gotNearby  service.NearbyQuery
}

func (f *fakePOIService) Categories(context.Context) ([]service.Category, error) {
	f.calls++
	return f.categories, f.err
}

func (f *fakePOIService) InBBox(_ context.Context, q service.BBoxQuery) (service.POICollection, error) {
	f.calls++
	f.gotBBox = q
	return f.collection, f.err
}

func (f *fakePOIService) Get(_ context.Context, id int64) (service.POIDetail, error) {
	f.calls++
	f.gotID = id
	return f.detail, f.err
}

func (f *fakePOIService) Search(_ context.Context, q string, limit int) ([]service.SearchHit, error) {
	f.calls++
	f.gotQuery, f.gotLimit = q, limit
	return f.hits, f.err
}

func (f *fakePOIService) Nearby(_ context.Context, q service.NearbyQuery) (service.NearbyCollection, error) {
	f.calls++
	f.gotNearby = q
	return f.nearby, f.err
}

func newTestRouter(d *fakeDistrictService, p *fakePOIService) http.Handler {
	return NewRouter(Deps{DB: fakePinger{}, Districts: d, POIs: p})
}
