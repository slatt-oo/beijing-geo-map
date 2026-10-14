package service

import "encoding/json"

type Point struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

func newPoint(location [2]float64) Point {
	return Point{Type: "Point", Coordinates: location}
}

type DistrictProperties struct {
	Adcode   string     `json:"adcode"`
	Name     string     `json:"name"`
	AreaKm2  float64    `json:"area_km2"`
	POICount int        `json:"poi_count"`
	Center   [2]float64 `json:"center"`
}

type DistrictFeature struct {
	Type       string             `json:"type"`
	Geometry   json.RawMessage    `json:"geometry"`
	Properties DistrictProperties `json:"properties"`
}

type DistrictCollection struct {
	Type     string            `json:"type"`
	Features []DistrictFeature `json:"features"`
}

type POIProperties struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type POIFeature struct {
	Type       string        `json:"type"`
	ID         int64         `json:"id"`
	Geometry   Point         `json:"geometry"`
	Properties POIProperties `json:"properties"`
}

type POICollection struct {
	Type      string       `json:"type"`
	Features  []POIFeature `json:"features"`
	Truncated bool         `json:"truncated"`
}

type NearbyProperties struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	DistanceM int    `json:"distance_m"`
}

type NearbyFeature struct {
	Type       string           `json:"type"`
	ID         int64            `json:"id"`
	Geometry   Point            `json:"geometry"`
	Properties NearbyProperties `json:"properties"`
}

type NearbyCollection struct {
	Type     string          `json:"type"`
	Features []NearbyFeature `json:"features"`
	Center   [2]float64      `json:"center"`
	Radius   float64         `json:"radius"`
}

type Category struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type DistrictRef struct {
	Adcode string `json:"adcode"`
	Name   string `json:"name"`
}

type CategoryCount struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type DistrictDetail struct {
	Adcode    string          `json:"adcode"`
	Name      string          `json:"name"`
	AreaKm2   float64         `json:"area_km2"`
	Center    [2]float64      `json:"center"`
	POICounts []CategoryCount `json:"poi_counts"`
}

type SearchHit struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Category     string     `json:"category"`
	DistrictName *string    `json:"district_name"`
	Location     [2]float64 `json:"location"`
}

type POIDetail struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	NameEn   *string           `json:"name_en"`
	Category Category          `json:"category"`
	District *DistrictRef      `json:"district"`
	Location [2]float64        `json:"location"`
	Address  *string           `json:"address"`
	Tags     map[string]string `json:"tags"`
	OSMURL   string            `json:"osm_url"`
}
