package importer

import (
	"fmt"
	"strings"
	"testing"
)

func TestClassifyFollowsPriorityOrder(t *testing.T) {
	cases := []struct {
		name string
		tags map[string]string
		want string
		ok   bool
	}{
		{"subway via railway tag", map[string]string{"railway": "station", "station": "subway"}, "subway_station", true},
		{"subway via public_transport tag", map[string]string{"public_transport": "station", "subway": "yes"}, "subway_station", true},
		{"university", map[string]string{"amenity": "university"}, "university", true},
		{"hospital", map[string]string{"amenity": "hospital"}, "hospital", true},
		{"museum", map[string]string{"tourism": "museum"}, "attraction", true},
		{"park that is also an attraction", map[string]string{"leisure": "park", "tourism": "attraction"}, "attraction", true},
		{"park", map[string]string{"leisure": "park"}, "park", true},
		{"railway station that is not a subway", map[string]string{"railway": "station"}, "", false},
		{"unrelated", map[string]string{"shop": "supermarket"}, "", false},
	}
	for _, c := range cases {
		got, ok := classify(c.tags)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: classify = (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestPickNamePrefersChineseName(t *testing.T) {
	if got := pickName(map[string]string{"name": "Peking University", "name:zh": "北京大学"}); got != "北京大学" {
		t.Errorf("pickName = %q, want 北京大学", got)
	}
	if got := pickName(map[string]string{"name": " 颐和园 "}); got != "颐和园" {
		t.Errorf("pickName = %q, want 颐和园", got)
	}
	if got := pickName(map[string]string{"name:en": "Only English"}); got != "" {
		t.Errorf("pickName = %q, want empty", got)
	}
}

func TestComposeAddress(t *testing.T) {
	if got := composeAddress(map[string]string{"addr:full": "颐和园路5号", "addr:street": "忽略"}); got != "颐和园路5号" {
		t.Errorf("composeAddress = %q, want 颐和园路5号", got)
	}
	parts := map[string]string{"addr:city": "北京市", "addr:district": "海淀区", "addr:street": "颐和园路", "addr:housenumber": "5号"}
	if got := composeAddress(parts); got != "北京市海淀区颐和园路5号" {
		t.Errorf("composeAddress = %q, want 北京市海淀区颐和园路5号", got)
	}
	if got := composeAddress(map[string]string{}); got != "" {
		t.Errorf("composeAddress = %q, want empty", got)
	}
}

func TestParsePOIsBuildsPOIsFromOverpassElements(t *testing.T) {
	input := `{"elements":[
	  {"type":"node","id":1,"lat":39.99,"lon":116.31,"tags":{"amenity":"university","name":"北京大学","name:en":"Peking University","addr:full":"颐和园路5号"}},
	  {"type":"way","id":2,"center":{"lat":39.999,"lon":116.27},"tags":{"leisure":"park","tourism":"attraction","name":"颐和园"}},
	  {"type":"relation","id":3,"tags":{"leisure":"park","name":"没有中心点的公园"}},
	  {"type":"node","id":4,"lat":39.9,"lon":116.4,"tags":{"amenity":"hospital"}},
	  {"type":"node","id":5,"lat":39.9,"lon":116.4,"tags":{"shop":"supermarket","name":"超市"}}
	]}`

	pois, err := ParsePOIs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParsePOIs: %v", err)
	}
	if len(pois) != 2 {
		t.Fatalf("got %d POIs, want 2: %+v", len(pois), pois)
	}

	pku := pois[0]
	if pku.OSMType != "n" || pku.OSMID != 1 || pku.Name != "北京大学" || pku.Category != "university" ||
		pku.Lng != 116.31 || pku.Lat != 39.99 || pku.Tags["amenity"] != "university" {
		t.Errorf("first POI = %+v", pku)
	}
	if pku.NameEn == nil || *pku.NameEn != "Peking University" {
		t.Errorf("NameEn = %v, want Peking University", pku.NameEn)
	}
	if pku.Address == nil || *pku.Address != "颐和园路5号" {
		t.Errorf("Address = %v, want 颐和园路5号", pku.Address)
	}

	summerPalace := pois[1]
	if summerPalace.OSMType != "w" || summerPalace.OSMID != 2 || summerPalace.Category != "attraction" ||
		summerPalace.Lng != 116.27 || summerPalace.Lat != 39.999 {
		t.Errorf("second POI = %+v", summerPalace)
	}
	if summerPalace.NameEn != nil || summerPalace.Address != nil {
		t.Errorf("second POI should have no English name or address: %+v", summerPalace)
	}
}

func TestDedupeSubwayStationsKeepsOneStationPerNearbyName(t *testing.T) {
	pois := []POI{
		{OSMType: "w", OSMID: 10, Name: "西直门", Category: "subway_station", Lng: 116.3535, Lat: 39.9405},
		{OSMType: "n", OSMID: 30, Name: "西直门", Category: "subway_station", Lng: 116.3550, Lat: 39.9410},
		{OSMType: "n", OSMID: 20, Name: "西直门", Category: "subway_station", Lng: 116.3540, Lat: 39.9400},
		{OSMType: "n", OSMID: 40, Name: "草桥", Category: "subway_station", Lng: 116.3500, Lat: 39.8460},
		{OSMType: "n", OSMID: 50, Name: "草桥", Category: "subway_station", Lng: 116.3500, Lat: 39.8500},
		{OSMType: "n", OSMID: 60, Name: "动物园", Category: "attraction", Lng: 116.3360, Lat: 39.9390},
	}

	got := dedupeSubwayStations(pois)

	var keys []string
	for _, p := range got {
		keys = append(keys, fmt.Sprintf("%s%d", p.OSMType, p.OSMID))
	}
	want := []string{"n60", "n20", "n40", "n50"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("kept = %v, want %v", keys, want)
	}
}

func TestHaversineMeters(t *testing.T) {
	got := haversineMeters(116.05, 40.05, 116.05, 40.059)
	if got < 1000.7 || got > 1000.8 {
		t.Fatalf("haversineMeters = %v, want about 1000.76 (0.009° of latitude on the mean-radius sphere)", got)
	}
}
