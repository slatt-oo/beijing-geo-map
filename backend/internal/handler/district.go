package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/service"
)

type DistrictService interface {
	List(ctx context.Context) (service.DistrictCollection, error)
	Get(ctx context.Context, adcode string) (service.DistrictDetail, error)
	Locate(ctx context.Context, lng, lat float64) (*service.DistrictRef, error)
}

func listDistricts(s DistrictService) gin.HandlerFunc {
	return func(c *gin.Context) {
		districts, err := s.List(c)
		if err != nil {
			writeServiceError(c, err, "")
			return
		}
		writeGeoJSON(c, districts)
	}
}

func getDistrict(s DistrictService) gin.HandlerFunc {
	return func(c *gin.Context) {
		adcode, err := parseAdcode(c.Param("adcode"))
		if err != nil {
			writeParamError(c, err)
			return
		}
		detail, err := s.Get(c, adcode)
		if err != nil {
			writeServiceError(c, err, "行政区不存在")
			return
		}
		writeData(c, detail)
	}
}

func locateDistrict(s DistrictService) gin.HandlerFunc {
	return func(c *gin.Context) {
		lng, lat, err := parseLngLat(c.Query("lng"), c.Query("lat"))
		if err != nil {
			writeParamError(c, err)
			return
		}
		ref, err := s.Locate(c, lng, lat)
		if err != nil {
			writeServiceError(c, err, "")
			return
		}
		writeData(c, ref)
	}
}
