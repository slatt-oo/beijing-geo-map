package importer

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
)

type POI struct {
	OSMType  string
	OSMID    int64
	Name     string
	NameEn   *string
	Category string
	Lng      float64
	Lat      float64
	Address  *string
	Tags     map[string]string
}

type overpassElement struct {
	Type   string   `json:"type"`
	ID     int64    `json:"id"`
	Lat    *float64 `json:"lat"`
	Lon    *float64 `json:"lon"`
	Center *struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"center"`
	Tags map[string]string `json:"tags"`
}

const subwayDedupeMeters = 300

var osmTypeCodes = map[string]string{"node": "n", "way": "w", "relation": "r"}

// ParsePOIs 解析 Overpass API 的 JSON 输出（out center tags）。无法分类、没有名称或没有坐标的要素会被丢弃。
func ParsePOIs(r io.Reader) ([]POI, error) {
	var resp struct {
		Elements []overpassElement `json:"elements"`
	}
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode overpass response: %w", err)
	}
	pois := make([]POI, 0, len(resp.Elements))
	for _, el := range resp.Elements {
		if poi, ok := toPOI(el); ok {
			pois = append(pois, poi)
		}
	}
	return dedupeSubwayStations(pois), nil
}

func toPOI(el overpassElement) (POI, bool) {
	osmType, ok := osmTypeCodes[el.Type]
	if !ok {
		return POI{}, false
	}
	category, ok := classify(el.Tags)
	if !ok {
		return POI{}, false
	}
	name := pickName(el.Tags)
	if name == "" {
		return POI{}, false
	}
	lng, lat, ok := location(el)
	if !ok {
		return POI{}, false
	}
	return POI{
		OSMType:  osmType,
		OSMID:    el.ID,
		Name:     name,
		NameEn:   optional(el.Tags["name:en"]),
		Category: category,
		Lng:      lng,
		Lat:      lat,
		Address:  optional(composeAddress(el.Tags)),
		Tags:     el.Tags,
	}, true
}

// classify 按优先级匹配，命中第一条即返回：同时带有景点和公园标签的要素（如颐和园）归为景点。
func classify(tags map[string]string) (string, bool) {
	switch {
	case tags["railway"] == "station" && tags["station"] == "subway",
		tags["public_transport"] == "station" && tags["subway"] == "yes":
		return "subway_station", true
	case tags["amenity"] == "university":
		return "university", true
	case tags["amenity"] == "hospital":
		return "hospital", true
	case tags["tourism"] == "attraction", tags["tourism"] == "museum":
		return "attraction", true
	case tags["leisure"] == "park":
		return "park", true
	default:
		return "", false
	}
}

func pickName(tags map[string]string) string {
	if name := strings.TrimSpace(tags["name:zh"]); name != "" {
		return name
	}
	return strings.TrimSpace(tags["name"])
}

var addressParts = []string{"addr:province", "addr:city", "addr:district", "addr:street", "addr:housenumber"}

func composeAddress(tags map[string]string) string {
	if full := strings.TrimSpace(tags["addr:full"]); full != "" {
		return full
	}
	var b strings.Builder
	for _, key := range addressParts {
		b.WriteString(strings.TrimSpace(tags[key]))
	}
	return b.String()
}

func location(el overpassElement) (float64, float64, bool) {
	if el.Lat != nil && el.Lon != nil {
		return *el.Lon, *el.Lat, true
	}
	if el.Center != nil {
		return el.Center.Lon, el.Center.Lat, true
	}
	return 0, 0, false
}

func optional(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

var osmTypeRank = map[string]int{"n": 0, "w": 1, "r": 2}

// dedupeSubwayStations 处理换乘站在 OSM 中每条线路各有一条记录的情况：同名且相距 300 米以内只保留一条，
// 节点优先，其次 osm_id 较小者。非地铁站保持原有顺序，保留下来的地铁站排在它们之后。
func dedupeSubwayStations(pois []POI) []POI {
	result := make([]POI, 0, len(pois))
	var stations []POI
	for _, p := range pois {
		if p.Category == "subway_station" {
			stations = append(stations, p)
		} else {
			result = append(result, p)
		}
	}
	slices.SortFunc(stations, func(a, b POI) int {
		if c := cmp.Compare(osmTypeRank[a.OSMType], osmTypeRank[b.OSMType]); c != 0 {
			return c
		}
		return cmp.Compare(a.OSMID, b.OSMID)
	})
	var kept []POI
	for _, s := range stations {
		duplicate := slices.ContainsFunc(kept, func(k POI) bool {
			return k.Name == s.Name && haversineMeters(k.Lng, k.Lat, s.Lng, s.Lat) <= subwayDedupeMeters
		})
		if !duplicate {
			kept = append(kept, s)
		}
	}
	return append(result, kept...)
}

const earthRadiusMeters = 6371008.8

func haversineMeters(lng1, lat1, lng2, lat2 float64) float64 {
	phi1, phi2 := lat1*math.Pi/180, lat2*math.Pi/180
	dPhi := phi2 - phi1
	dLambda := (lng2 - lng1) * math.Pi / 180
	h := math.Sin(dPhi/2)*math.Sin(dPhi/2) + math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	return 2 * earthRadiusMeters * math.Asin(math.Sqrt(h))
}
