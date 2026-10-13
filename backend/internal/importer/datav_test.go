package importer

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/geo"
)

const datavSample = `{"type":"FeatureCollection","features":[
  {"type":"Feature","properties":{"adcode":110101,"name":"东城区"},
   "geometry":{"type":"MultiPolygon","coordinates":[[[[116.40,39.90],[116.41,39.90],[116.41,39.91],[116.40,39.90]]]]}}
]}`

func TestParseDistrictsConvertsCoordinatesToWGS84(t *testing.T) {
	districts, err := ParseDistricts(strings.NewReader(datavSample))
	if err != nil {
		t.Fatalf("ParseDistricts: %v", err)
	}
	if len(districts) != 1 || districts[0].Adcode != "110101" || districts[0].Name != "东城区" {
		t.Fatalf("districts = %+v", districts)
	}
	var geometry struct {
		Type        string           `json:"type"`
		Coordinates [][][][2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(districts[0].Geometry, &geometry); err != nil {
		t.Fatalf("geometry is not valid JSON: %v", err)
	}
	if geometry.Type != "MultiPolygon" {
		t.Fatalf("geometry type = %q, want MultiPolygon", geometry.Type)
	}
	ring := geometry.Coordinates[0][0]
	wantLng, wantLat := geo.GCJ02ToWGS84(116.40, 39.90)
	if math.Abs(ring[0][0]-wantLng) > 1e-12 || math.Abs(ring[0][1]-wantLat) > 1e-12 {
		t.Fatalf("first vertex = %v, want (%v, %v)", ring[0], wantLng, wantLat)
	}
	if ring[0] != ring[len(ring)-1] {
		t.Fatalf("ring is no longer closed: first %v, last %v", ring[0], ring[len(ring)-1])
	}
}

func TestParseDistrictsRejectsUnsupportedGeometry(t *testing.T) {
	input := strings.Replace(datavSample, `"type":"MultiPolygon"`, `"type":"Polygon"`, 1)
	_, err := ParseDistricts(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "unsupported geometry type") {
		t.Fatalf("err = %v, want unsupported geometry type", err)
	}
}

func TestParseDistrictsRejectsInvalidAdcode(t *testing.T) {
	input := strings.Replace(datavSample, `"adcode":110101`, `"adcode":1101`, 1)
	_, err := ParseDistricts(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "adcode") {
		t.Fatalf("err = %v, want an invalid adcode error", err)
	}
}

func TestParseDistrictsRejectsEmptyCollection(t *testing.T) {
	_, err := ParseDistricts(strings.NewReader(`{"type":"FeatureCollection","features":[]}`))
	if err == nil {
		t.Fatal("expected an error for an empty collection")
	}
}
