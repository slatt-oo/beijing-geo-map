// Package geo 实现 WGS-84 与 GCJ-02 坐标的互相转换（公开的标准算法，克拉索夫斯基椭球参数）。
package geo

import "math"

const (
	krasovskyA  = 6378245.0
	krasovskyEE = 0.00669342162296594323

	inverseTolerance     = 1e-7
	maxInverseIterations = 10
)

func OutOfChina(lng, lat float64) bool {
	return lng < 72.004 || lng > 137.8347 || lat < 0.8293 || lat > 55.8271
}

func WGS84ToGCJ02(lng, lat float64) (float64, float64) {
	if OutOfChina(lng, lat) {
		return lng, lat
	}
	dLng, dLat := offset(lng, lat)
	return lng + dLng, lat + dLat
}

// GCJ02ToWGS84 没有解析解，用迭代法反算：每轮按正向转换的残差修正估计值。
func GCJ02ToWGS84(lng, lat float64) (float64, float64) {
	if OutOfChina(lng, lat) {
		return lng, lat
	}
	wLng, wLat := lng, lat
	for range maxInverseIterations {
		gLng, gLat := WGS84ToGCJ02(wLng, wLat)
		dLng, dLat := gLng-lng, gLat-lat
		wLng, wLat = wLng-dLng, wLat-dLat
		if math.Abs(dLng) < inverseTolerance && math.Abs(dLat) < inverseTolerance {
			break
		}
	}
	return wLng, wLat
}

func offset(lng, lat float64) (float64, float64) {
	x, y := lng-105.0, lat-35.0
	dLat := transformLat(x, y)
	dLng := transformLng(x, y)
	radLat := lat / 180.0 * math.Pi
	magic := 1 - krasovskyEE*math.Sin(radLat)*math.Sin(radLat)
	sqrtMagic := math.Sqrt(magic)
	dLat = (dLat * 180.0) / ((krasovskyA * (1 - krasovskyEE)) / (magic * sqrtMagic) * math.Pi)
	dLng = (dLng * 180.0) / (krasovskyA / sqrtMagic * math.Cos(radLat) * math.Pi)
	return dLng, dLat
}

func transformLat(x, y float64) float64 {
	r := -100.0 + 2.0*x + 3.0*y + 0.2*y*y + 0.1*x*y + 0.2*math.Sqrt(math.Abs(x))
	r += (20.0*math.Sin(6.0*x*math.Pi) + 20.0*math.Sin(2.0*x*math.Pi)) * 2.0 / 3.0
	r += (20.0*math.Sin(y*math.Pi) + 40.0*math.Sin(y/3.0*math.Pi)) * 2.0 / 3.0
	r += (160.0*math.Sin(y/12.0*math.Pi) + 320.0*math.Sin(y*math.Pi/30.0)) * 2.0 / 3.0
	return r
}

func transformLng(x, y float64) float64 {
	r := 300.0 + x + 2.0*y + 0.1*x*x + 0.1*x*y + 0.1*math.Sqrt(math.Abs(x))
	r += (20.0*math.Sin(6.0*x*math.Pi) + 20.0*math.Sin(2.0*x*math.Pi)) * 2.0 / 3.0
	r += (20.0*math.Sin(x*math.Pi) + 40.0*math.Sin(x/3.0*math.Pi)) * 2.0 / 3.0
	r += (150.0*math.Sin(x/12.0*math.Pi) + 300.0*math.Sin(x/30.0*math.Pi)) * 2.0 / 3.0
	return r
}
