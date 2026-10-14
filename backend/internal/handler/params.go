package handler

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/service"
)

type paramError struct {
	message string
}

func (e paramError) Error() string {
	return e.message
}

func invalidParam(format string, args ...any) error {
	return paramError{message: fmt.Sprintf(format, args...)}
}

// strconv.ParseFloat 接受 "NaN"、"Inf"，而 NaN 能绕过 v < min || v > max 这样的范围检查，所以必须显式拒绝。
func parseFinite(name, raw string) (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, invalidParam("%s 必须是有限的数字", name)
	}
	return v, nil
}

func parseRequiredFloat(name, raw string, min, max float64) (float64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, invalidParam("缺少参数 %s", name)
	}
	return parseFloatInRange(name, raw, min, max)
}

func parseFloatInRange(name, raw string, min, max float64) (float64, error) {
	v, err := parseFinite(name, raw)
	if err != nil {
		return 0, err
	}
	if v < min || v > max {
		return 0, invalidParam("%s 必须在 %g 到 %g 之间", name, min, max)
	}
	return v, nil
}

func parseLngLat(rawLng, rawLat string) (float64, float64, error) {
	lng, err := parseRequiredFloat("lng", rawLng, -180, 180)
	if err != nil {
		return 0, 0, err
	}
	lat, err := parseRequiredFloat("lat", rawLat, -90, 90)
	if err != nil {
		return 0, 0, err
	}
	return lng, lat, nil
}

func parseOptionalFloat(name, raw string, def, min, max float64) (float64, error) {
	if strings.TrimSpace(raw) == "" {
		return def, nil
	}
	return parseFloatInRange(name, raw, min, max)
}

func parseOptionalInt(name, raw string, def, min, max int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < min || v > max {
		return 0, invalidParam("%s 必须是 %d 到 %d 之间的整数", name, min, max)
	}
	return v, nil
}

func parseBBox(raw string) (model.BBox, error) {
	if strings.TrimSpace(raw) == "" {
		return model.BBox{}, invalidParam("缺少参数 bbox")
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return model.BBox{}, invalidParam("bbox 格式应为 minLng,minLat,maxLng,maxLat")
	}
	var v [4]float64
	for i, part := range parts {
		f, err := parseFinite("bbox", part)
		if err != nil {
			return model.BBox{}, err
		}
		v[i] = f
	}
	box := model.BBox{MinLng: v[0], MinLat: v[1], MaxLng: v[2], MaxLat: v[3]}
	if box.MinLng < -180 || box.MaxLng > 180 || box.MinLat < -90 || box.MaxLat > 90 {
		return model.BBox{}, invalidParam("bbox 超出经纬度范围")
	}
	if box.MinLng >= box.MaxLng || box.MinLat >= box.MaxLat {
		return model.BBox{}, invalidParam("bbox 的最小值必须小于最大值")
	}
	return box, nil
}

func parseCategories(raw string) []string {
	var codes []string
	for _, part := range strings.Split(raw, ",") {
		if code := strings.TrimSpace(part); code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

// PostgreSQL 会对非法 UTF-8 和 NUL 字符报错（SQLSTATE 22021），必须在入口拒绝；长度按字符数计算，一个汉字算一个。
func parseQuery(raw string) (string, error) {
	if !utf8.ValidString(raw) || strings.ContainsRune(raw, 0) {
		return "", invalidParam("q 包含非法字符")
	}
	q := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(q)
	if n == 0 {
		return "", invalidParam("缺少参数 q")
	}
	if n > service.MaxQueryRunes {
		return "", invalidParam("q 不能超过 %d 个字符", service.MaxQueryRunes)
	}
	return q, nil
}

var adcodePattern = regexp.MustCompile(`^[0-9]{6}$`)

func parseAdcode(raw string) (string, error) {
	if !adcodePattern.MatchString(raw) {
		return "", invalidParam("adcode 必须是 6 位数字")
	}
	return raw, nil
}

func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, invalidParam("id 必须是正整数")
	}
	return id, nil
}
