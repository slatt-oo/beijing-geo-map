package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var snapshotDir = filepath.Join("..", "..", "..", "data", "raw")

func TestRealDistrictSnapshotParses(t *testing.T) {
	f, err := os.Open(filepath.Join(snapshotDir, DistrictsFile))
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer f.Close()

	districts, err := ParseDistricts(f)
	if err != nil {
		t.Fatalf("ParseDistricts: %v", err)
	}
	if len(districts) != 16 {
		t.Fatalf("got %d districts, want 16", len(districts))
	}
	for _, d := range districts {
		if !strings.HasPrefix(d.Adcode, "1101") {
			t.Errorf("district %s %s is not a Beijing district", d.Adcode, d.Name)
		}
	}
}

func TestRealPOISnapshotParses(t *testing.T) {
	f, err := os.Open(filepath.Join(snapshotDir, POIsFile))
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer f.Close()

	pois, err := ParsePOIs(f)
	if err != nil {
		t.Fatalf("ParsePOIs: %v", err)
	}
	if len(pois) < 4000 || len(pois) > 4300 {
		t.Fatalf("got %d POIs, want between 4000 and 4300", len(pois))
	}
	counts := map[string]int{}
	for _, p := range pois {
		counts[p.Category]++
	}
	for _, code := range []string{"subway_station", "university", "attraction", "hospital", "park"} {
		if counts[code] == 0 {
			t.Errorf("no POIs in category %s", code)
		}
	}
}
