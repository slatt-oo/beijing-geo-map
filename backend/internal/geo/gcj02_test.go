package geo

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

const tolerance = 1e-6

type coordCase struct {
	Name  string     `json:"name"`
	WGS84 [2]float64 `json:"wgs84"`
	GCJ02 [2]float64 `json:"gcj02"`
}

func loadCases(t *testing.T) []coordCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "coord_cases.json"))
	if err != nil {
		t.Fatalf("read shared fixture: %v", err)
	}
	var fixture struct {
		Cases []coordCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode shared fixture: %v", err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("shared fixture has no cases")
	}
	return fixture.Cases
}

func near(a, b [2]float64) bool {
	return math.Abs(a[0]-b[0]) <= tolerance && math.Abs(a[1]-b[1]) <= tolerance
}

func TestWGS84ToGCJ02MatchesReference(t *testing.T) {
	for _, c := range loadCases(t) {
		lng, lat := WGS84ToGCJ02(c.WGS84[0], c.WGS84[1])
		if got := [2]float64{lng, lat}; !near(got, c.GCJ02) {
			t.Errorf("%s: WGS84ToGCJ02 = %.9f, want %.9f", c.Name, got, c.GCJ02)
		}
	}
}

func TestGCJ02ToWGS84InvertsForwardTransform(t *testing.T) {
	for _, c := range loadCases(t) {
		wLng, wLat := GCJ02ToWGS84(c.GCJ02[0], c.GCJ02[1])
		gLng, gLat := WGS84ToGCJ02(wLng, wLat)
		if got := [2]float64{gLng, gLat}; !near(got, c.GCJ02) {
			t.Errorf("%s: forward(inverse(g)) = %.9f, want %.9f", c.Name, got, c.GCJ02)
		}
		if got := [2]float64{wLng, wLat}; !near(got, c.WGS84) {
			t.Errorf("%s: GCJ02ToWGS84 = %.9f, want %.9f", c.Name, got, c.WGS84)
		}
	}
}

func TestOutOfChinaPointsAreUnchanged(t *testing.T) {
	const lng, lat = 139.767125, 35.681236
	if !OutOfChina(lng, lat) {
		t.Fatal("Tokyo should be out of China")
	}
	if OutOfChina(116.391275, 39.906217) {
		t.Fatal("Tiananmen should be inside China")
	}
	if gLng, gLat := WGS84ToGCJ02(lng, lat); gLng != lng || gLat != lat {
		t.Errorf("WGS84ToGCJ02 changed an out-of-China point: (%v, %v)", gLng, gLat)
	}
	if wLng, wLat := GCJ02ToWGS84(lng, lat); wLng != lng || wLat != lat {
		t.Errorf("GCJ02ToWGS84 changed an out-of-China point: (%v, %v)", wLng, wLat)
	}
}
