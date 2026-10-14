package service

import (
	"context"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

type DistrictRepository interface {
	List(ctx context.Context) ([]model.DistrictFeature, error)
	Get(ctx context.Context, adcode string) (model.DistrictDetail, error)
	Locate(ctx context.Context, lng, lat float64) (*model.DistrictRef, error)
}

type DistrictService struct {
	repo DistrictRepository
}

func NewDistrictService(repo DistrictRepository) *DistrictService {
	return &DistrictService{repo: repo}
}

func (s *DistrictService) List(ctx context.Context) (DistrictCollection, error) {
	districts, err := s.repo.List(ctx)
	if err != nil {
		return DistrictCollection{}, err
	}
	features := make([]DistrictFeature, 0, len(districts))
	for _, d := range districts {
		features = append(features, DistrictFeature{
			Type:     "Feature",
			Geometry: d.Geometry,
			Properties: DistrictProperties{
				Adcode:   d.Adcode,
				Name:     d.Name,
				AreaKm2:  d.AreaKm2,
				POICount: d.POICount,
				Center:   d.Center,
			},
		})
	}
	return DistrictCollection{Type: "FeatureCollection", Features: features}, nil
}

func (s *DistrictService) Get(ctx context.Context, adcode string) (DistrictDetail, error) {
	d, err := s.repo.Get(ctx, adcode)
	if err != nil {
		return DistrictDetail{}, err
	}
	counts := make([]CategoryCount, 0, len(d.POICounts))
	for _, c := range d.POICounts {
		counts = append(counts, CategoryCount{Code: c.Code, Name: c.Name, Count: c.Count})
	}
	return DistrictDetail{Adcode: d.Adcode, Name: d.Name, AreaKm2: d.AreaKm2, Center: d.Center, POICounts: counts}, nil
}

// Locate 在点不属于任何区时返回 nil, nil。
func (s *DistrictService) Locate(ctx context.Context, lng, lat float64) (*DistrictRef, error) {
	ref, err := s.repo.Locate(ctx, lng, lat)
	if err != nil || ref == nil {
		return nil, err
	}
	return &DistrictRef{Adcode: ref.Adcode, Name: ref.Name}, nil
}
