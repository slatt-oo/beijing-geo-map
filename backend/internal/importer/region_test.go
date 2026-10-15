package importer

import "testing"

func TestWithinRegionDropsPOIsOutsideTheRegion(t *testing.T) {
	pois := []POI{
		{OSMType: "n", OSMID: 1, Lng: 116.391, Lat: 39.906},      // 天安门
		{OSMType: "r", OSMID: 318110, Lng: 107.925, Lat: 35.722}, // 整条长城的 relation，中心点在甘肃
		{OSMType: "w", OSMID: 2, Lng: 117.665, Lat: 39.227},      // 清河农场飞地：北京管辖，位于天津境内
		{OSMType: "n", OSMID: 3, Lng: 114.4, Lat: 42.1},          // 范围的角点也算在内
	}

	kept, skipped := withinRegion(pois, beijingRegion)

	if skipped != 1 || len(kept) != 3 || kept[0].OSMID != 1 || kept[1].OSMID != 2 || kept[2].OSMID != 3 {
		t.Fatalf("kept %+v, skipped %d", kept, skipped)
	}
}
