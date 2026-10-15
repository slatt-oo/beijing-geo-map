package importer

type region struct {
	MinLng, MinLat, MaxLng, MaxLat float64
}

// beijingRegion 是北京市边界外扩约 1° 的范围（WGS-84）。OSM 中少数要素的中心点远在北京之外
// （例如整条长城的 relation，中心点在甘肃），这些 POI 不导入。前端 src/config.ts 的 REGION_BBOX 必须与它一致。
var beijingRegion = region{MinLng: 114.4, MinLat: 38.4, MaxLng: 118.5, MaxLat: 42.1}

func (r region) contains(lng, lat float64) bool {
	return lng >= r.MinLng && lng <= r.MaxLng && lat >= r.MinLat && lat <= r.MaxLat
}

// withinRegion 按原顺序返回落在 r 内的 POI，以及被丢弃的数量。
func withinRegion(pois []POI, r region) ([]POI, int) {
	kept := make([]POI, 0, len(pois))
	for _, p := range pois {
		if r.contains(p.Lng, p.Lat) {
			kept = append(kept, p)
		}
	}
	return kept, len(pois) - len(kept)
}
