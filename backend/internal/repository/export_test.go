package repository

// 导出给集成测试执行 EXPLAIN，保证检查的是真实执行的 SQL，而不是手写的副本。
var (
	PoisInBBoxSQL     = poisInBBoxSQL
	NearbyPOIsSQL     = nearbyPOIsSQL
	LocateDistrictSQL = locateDistrictSQL
	SearchPOIsSQL     = searchPOIsSQL
)
