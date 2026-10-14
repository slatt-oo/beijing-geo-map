package service

import (
	"context"
	"fmt"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

type POIRepository interface {
	InBBox(ctx context.Context, box model.BBox, categoryIDs []int16, limit int) ([]model.POISummary, bool, error)
	Get(ctx context.Context, id int64) (model.POIDetail, error)
	Search(ctx context.Context, q string, limit int) ([]model.SearchHit, error)
	Nearby(ctx context.Context, lng, lat, radiusM float64, categoryIDs []int16, limit int) ([]model.NearbyPOI, error)
}

type BBoxQuery struct {
	Box        model.BBox
	Categories []string
	Limit      int
}

type NearbyQuery struct {
	Lng, Lat   float64
	RadiusM    float64
	Categories []string
	Limit      int
}

type POIService struct {
	repo    POIRepository
	catalog *Catalog
}

func NewPOIService(repo POIRepository, catalog *Catalog) *POIService {
	return &POIService{repo: repo, catalog: catalog}
}

func (s *POIService) Categories(ctx context.Context) ([]Category, error) {
	return s.catalog.List(ctx)
}

func (s *POIService) InBBox(ctx context.Context, q BBoxQuery) (POICollection, error) {
	ids, err := s.catalog.IDs(ctx, q.Categories)
	if err != nil {
		return POICollection{}, err
	}
	pois, truncated, err := s.repo.InBBox(ctx, q.Box, ids, q.Limit)
	if err != nil {
		return POICollection{}, err
	}
	features := make([]POIFeature, 0, len(pois))
	for _, p := range pois {
		features = append(features, POIFeature{
			Type:       "Feature",
			ID:         p.ID,
			Geometry:   newPoint(p.Location),
			Properties: POIProperties{ID: p.ID, Name: p.Name, Category: p.Category},
		})
	}
	return POICollection{Type: "FeatureCollection", Features: features, Truncated: truncated}, nil
}

func (s *POIService) Get(ctx context.Context, id int64) (POIDetail, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return POIDetail{}, err
	}
	detail := POIDetail{
		ID:       p.ID,
		Name:     p.Name,
		NameEn:   p.NameEn,
		Category: Category{Code: p.Category.Code, Name: p.Category.Name, Color: p.Category.Color},
		Location: p.Location,
		Address:  p.Address,
		Tags:     p.Tags,
		OSMURL:   osmURL(p.OSMType, p.OSMID),
	}
	if p.District != nil {
		detail.District = &DistrictRef{Adcode: p.District.Adcode, Name: p.District.Name}
	}
	return detail, nil
}

var osmElementTypes = map[string]string{"n": "node", "w": "way", "r": "relation"}

func osmURL(osmType string, id int64) string {
	return fmt.Sprintf("https://www.openstreetmap.org/%s/%d", osmElementTypes[osmType], id)
}

func (s *POIService) Search(ctx context.Context, q string, limit int) ([]SearchHit, error) {
	hits, err := s.repo.Search(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	result := make([]SearchHit, 0, len(hits))
	for _, h := range hits {
		result = append(result, SearchHit{ID: h.ID, Name: h.Name, Category: h.Category, DistrictName: h.DistrictName, Location: h.Location})
	}
	return result, nil
}

func (s *POIService) Nearby(ctx context.Context, q NearbyQuery) (NearbyCollection, error) {
	ids, err := s.catalog.IDs(ctx, q.Categories)
	if err != nil {
		return NearbyCollection{}, err
	}
	pois, err := s.repo.Nearby(ctx, q.Lng, q.Lat, q.RadiusM, ids, q.Limit)
	if err != nil {
		return NearbyCollection{}, err
	}
	features := make([]NearbyFeature, 0, len(pois))
	for _, p := range pois {
		features = append(features, NearbyFeature{
			Type:       "Feature",
			ID:         p.ID,
			Geometry:   newPoint(p.Location),
			Properties: NearbyProperties{ID: p.ID, Name: p.Name, Category: p.Category, DistanceM: p.DistanceM},
		})
	}
	return NearbyCollection{Type: "FeatureCollection", Features: features, Center: [2]float64{q.Lng, q.Lat}, Radius: q.RadiusM}, nil
}
