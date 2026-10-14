package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/service"
)

type POIService interface {
	Categories(ctx context.Context) ([]service.Category, error)
	InBBox(ctx context.Context, q service.BBoxQuery) (service.POICollection, error)
	Get(ctx context.Context, id int64) (service.POIDetail, error)
	Search(ctx context.Context, q string, limit int) ([]service.SearchHit, error)
	Nearby(ctx context.Context, q service.NearbyQuery) (service.NearbyCollection, error)
}

func listCategories(s POIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		categories, err := s.Categories(c)
		if err != nil {
			writeServiceError(c, err, "")
			return
		}
		writeData(c, categories)
	}
}
