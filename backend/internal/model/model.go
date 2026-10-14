// Package model 定义 repository 返回的数据结构。坐标一律为 WGS-84 的 [经度, 纬度]。
package model

import "encoding/json"

type Category struct {
	ID    int16
	Code  string
	Name  string
	Color string
}

type DistrictRef struct {
	Adcode string
	Name   string
}

type DistrictFeature struct {
	Adcode   string
	Name     string
	AreaKm2  float64
	POICount int
	Center   [2]float64
	// Geometry 是按约 10 米容差简化后的 GeoJSON MultiPolygon，坐标保留 6 位小数。
	Geometry json.RawMessage
}

type CategoryCount struct {
	Code  string
	Name  string
	Count int
}

type DistrictDetail struct {
	Adcode    string
	Name      string
	AreaKm2   float64
	Center    [2]float64
	POICounts []CategoryCount
}

type POISummary struct {
	ID       int64
	Name     string
	Category string
	Location [2]float64
}

type NearbyPOI struct {
	POISummary
	DistanceM int
}

type SearchHit struct {
	POISummary
	DistrictName *string
}

type POIDetail struct {
	ID       int64
	Name     string
	NameEn   *string
	OSMType  string
	OSMID    int64
	Category Category
	District *DistrictRef
	Location [2]float64
	Address  *string
	Tags     map[string]string
}

type BBox struct {
	MinLng, MinLat, MaxLng, MaxLat float64
}
