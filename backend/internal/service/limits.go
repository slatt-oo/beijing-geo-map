package service

// 各接口参数的默认值和取值范围，handler 按这些值校验和补全参数。
const (
	DefaultBBoxLimit = 2000
	MaxBBoxLimit     = 5000

	DefaultSearchLimit = 10
	MaxSearchLimit     = 20
	MaxQueryRunes      = 50

	DefaultNearbyLimit   = 50
	MaxNearbyLimit       = 100
	DefaultNearbyRadiusM = 1000
	MinNearbyRadiusM     = 100
	MaxNearbyRadiusM     = 5000
)
