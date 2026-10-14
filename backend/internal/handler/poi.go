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

func poisInBBox(s POIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		box, err := parseBBox(c.Query("bbox"))
		if err != nil {
			writeParamError(c, err)
			return
		}
		limit, err := parseOptionalInt("limit", c.Query("limit"), service.DefaultBBoxLimit, 1, service.MaxBBoxLimit)
		if err != nil {
			writeParamError(c, err)
			return
		}
		pois, err := s.InBBox(c, service.BBoxQuery{Box: box, Categories: parseCategories(c.Query("categories")), Limit: limit})
		if err != nil {
			writeServiceError(c, err, "")
			return
		}
		writeGeoJSON(c, pois)
	}
}

func getPOI(s POIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseID(c.Param("id"))
		if err != nil {
			writeParamError(c, err)
			return
		}
		detail, err := s.Get(c, id)
		if err != nil {
			writeServiceError(c, err, "POI 不存在")
			return
		}
		writeData(c, detail)
	}
}

func searchPOIs(s POIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		q, err := parseQuery(c.Query("q"))
		if err != nil {
			writeParamError(c, err)
			return
		}
		limit, err := parseOptionalInt("limit", c.Query("limit"), service.DefaultSearchLimit, 1, service.MaxSearchLimit)
		if err != nil {
			writeParamError(c, err)
			return
		}
		hits, err := s.Search(c, q, limit)
		if err != nil {
			writeServiceError(c, err, "")
			return
		}
		writeData(c, hits)
	}
}

func nearbyPOIs(s POIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		lng, lat, err := parseLngLat(c.Query("lng"), c.Query("lat"))
		if err != nil {
			writeParamError(c, err)
			return
		}
		radius, err := parseOptionalFloat("radius", c.Query("radius"),
			service.DefaultNearbyRadiusM, service.MinNearbyRadiusM, service.MaxNearbyRadiusM)
		if err != nil {
			writeParamError(c, err)
			return
		}
		limit, err := parseOptionalInt("limit", c.Query("limit"), service.DefaultNearbyLimit, 1, service.MaxNearbyLimit)
		if err != nil {
			writeParamError(c, err)
			return
		}
		pois, err := s.Nearby(c, service.NearbyQuery{
			Lng: lng, Lat: lat, RadiusM: radius, Categories: parseCategories(c.Query("categories")), Limit: limit,
		})
		if err != nil {
			writeServiceError(c, err, "")
			return
		}
		writeGeoJSON(c, pois)
	}
}
