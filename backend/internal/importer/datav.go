package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/geo"
)

type District struct {
	Adcode   string
	Name     string
	Geometry json.RawMessage
}

type datavFeature struct {
	Properties struct {
		Adcode json.Number `json:"adcode"`
		Name   string      `json:"name"`
	} `json:"properties"`
	Geometry struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
}

type multiPolygon struct {
	Type        string           `json:"type"`
	Coordinates [][][][2]float64 `json:"coordinates"`
}

// ParseDistricts 解析 DataV.GeoAtlas 的 FeatureCollection。输入坐标是 GCJ-02，返回的几何已转换为 WGS-84。
func ParseDistricts(r io.Reader) ([]District, error) {
	var collection struct {
		Features []datavFeature `json:"features"`
	}
	if err := json.NewDecoder(r).Decode(&collection); err != nil {
		return nil, fmt.Errorf("decode districts: %w", err)
	}
	if len(collection.Features) == 0 {
		return nil, errors.New("districts snapshot has no features")
	}
	districts := make([]District, 0, len(collection.Features))
	for i, f := range collection.Features {
		adcode := f.Properties.Adcode.String()
		if len(adcode) != 6 {
			return nil, fmt.Errorf("feature %d: invalid adcode %q", i, adcode)
		}
		if f.Geometry.Type != "MultiPolygon" {
			return nil, fmt.Errorf("feature %d (%s): unsupported geometry type %q", i, adcode, f.Geometry.Type)
		}
		var polygons [][][][2]float64
		if err := json.Unmarshal(f.Geometry.Coordinates, &polygons); err != nil {
			return nil, fmt.Errorf("feature %d (%s): decode coordinates: %w", i, adcode, err)
		}
		toWGS84(polygons)
		geometry, err := json.Marshal(multiPolygon{Type: "MultiPolygon", Coordinates: polygons})
		if err != nil {
			return nil, fmt.Errorf("feature %d (%s): encode geometry: %w", i, adcode, err)
		}
		districts = append(districts, District{Adcode: adcode, Name: f.Properties.Name, Geometry: geometry})
	}
	return districts, nil
}

func toWGS84(polygons [][][][2]float64) {
	for _, polygon := range polygons {
		for _, ring := range polygon {
			for i, pt := range ring {
				ring[i][0], ring[i][1] = geo.GCJ02ToWGS84(pt[0], pt[1])
			}
		}
	}
}
