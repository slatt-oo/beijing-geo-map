package handler

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

func paramMessage(t *testing.T, err error) string {
	t.Helper()
	var pe paramError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, want a paramError", err)
	}
	return pe.message
}

func TestParseLngLat(t *testing.T) {
	lng, lat, err := parseLngLat("116.39", " 39.9 ")
	if err != nil || lng != 116.39 || lat != 39.9 {
		t.Fatalf("parseLngLat = %v, %v, %v", lng, lat, err)
	}
	for _, tc := range []struct{ lng, lat, want string }{
		{"", "39.9", "缺少参数 lng"},
		{"116.39", "", "缺少参数 lat"},
		{"abc", "39.9", "lng 必须是有限的数字"},
		{"NaN", "39.9", "lng 必须是有限的数字"},
		{"116.39", "Inf", "lat 必须是有限的数字"},
		{"181", "39.9", "lng 必须在 -180 到 180 之间"},
		{"116.39", "-90.5", "lat 必须在 -90 到 90 之间"},
	} {
		_, _, err := parseLngLat(tc.lng, tc.lat)
		if got := paramMessage(t, err); got != tc.want {
			t.Errorf("parseLngLat(%q, %q) = %q, want %q", tc.lng, tc.lat, got, tc.want)
		}
	}
}

func TestParseOptionalInt(t *testing.T) {
	if v, err := parseOptionalInt("limit", "", 2000, 1, 5000); err != nil || v != 2000 {
		t.Fatalf("default = %v, %v; want 2000", v, err)
	}
	if v, err := parseOptionalInt("limit", " 5000 ", 2000, 1, 5000); err != nil || v != 5000 {
		t.Fatalf("explicit = %v, %v; want 5000", v, err)
	}
	for _, raw := range []string{"0", "5001", "1.5", "abc", "-3"} {
		_, err := parseOptionalInt("limit", raw, 2000, 1, 5000)
		if got := paramMessage(t, err); got != "limit 必须是 1 到 5000 之间的整数" {
			t.Errorf("parseOptionalInt(%q) = %q", raw, got)
		}
	}
}

func TestParseOptionalFloat(t *testing.T) {
	if v, err := parseOptionalFloat("radius", "", 1000, 100, 5000); err != nil || v != 1000 {
		t.Fatalf("default = %v, %v; want 1000", v, err)
	}
	if v, err := parseOptionalFloat("radius", "250.5", 1000, 100, 5000); err != nil || v != 250.5 {
		t.Fatalf("explicit = %v, %v; want 250.5", v, err)
	}
	for raw, want := range map[string]string{
		"50":   "radius 必须在 100 到 5000 之间",
		"5001": "radius 必须在 100 到 5000 之间",
		"Inf":  "radius 必须是有限的数字",
		"NaN":  "radius 必须是有限的数字",
	} {
		_, err := parseOptionalFloat("radius", raw, 1000, 100, 5000)
		if got := paramMessage(t, err); got != want {
			t.Errorf("parseOptionalFloat(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseBBox(t *testing.T) {
	box, err := parseBBox("116.0,39.8,116.6,40.1")
	if err != nil || box != (model.BBox{MinLng: 116.0, MinLat: 39.8, MaxLng: 116.6, MaxLat: 40.1}) {
		t.Fatalf("parseBBox = %+v, %v", box, err)
	}
	for raw, want := range map[string]string{
		"":                      "缺少参数 bbox",
		"1,2,3":                 "bbox 格式应为 minLng,minLat,maxLng,maxLat",
		"1,2,x,4":               "bbox 必须是有限的数字",
		"1,NaN,2,3":             "bbox 必须是有限的数字",
		"116.6,39.8,116.0,40.1": "bbox 的最小值必须小于最大值",
		"116.0,40.1,116.6,40.1": "bbox 的最小值必须小于最大值",
		"-181,0,0,1":            "bbox 超出经纬度范围",
	} {
		_, err := parseBBox(raw)
		if got := paramMessage(t, err); got != want {
			t.Errorf("parseBBox(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseCategories(t *testing.T) {
	if got := parseCategories(""); got != nil {
		t.Errorf("parseCategories(\"\") = %v, want nil", got)
	}
	if got := parseCategories(" park, university ,,"); !slices.Equal(got, []string{"park", "university"}) {
		t.Errorf("parseCategories = %v", got)
	}
}

func TestParseQuery(t *testing.T) {
	if q, err := parseQuery("  北京大学 "); err != nil || q != "北京大学" {
		t.Fatalf("parseQuery = %q, %v", q, err)
	}
	if _, err := parseQuery(strings.Repeat("北", 50)); err != nil {
		t.Fatalf("50 runes must be accepted: %v", err)
	}
	for raw, want := range map[string]string{
		"":                      "缺少参数 q",
		"   ":                   "缺少参数 q",
		strings.Repeat("北", 51): "q 不能超过 50 个字符",
		"a\x00b":                "q 包含非法字符",
		"\xff":                  "q 包含非法字符",
	} {
		_, err := parseQuery(raw)
		if got := paramMessage(t, err); got != want {
			t.Errorf("parseQuery(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseAdcode(t *testing.T) {
	if got, err := parseAdcode("110108"); err != nil || got != "110108" {
		t.Fatalf("parseAdcode = %q, %v", got, err)
	}
	for _, raw := range []string{"", "11010", "1101080", "11010a", "110108 "} {
		_, err := parseAdcode(raw)
		if got := paramMessage(t, err); got != "adcode 必须是 6 位数字" {
			t.Errorf("parseAdcode(%q) = %q", raw, got)
		}
	}
}

func TestParseID(t *testing.T) {
	if got, err := parseID("42"); err != nil || got != 42 {
		t.Fatalf("parseID = %v, %v", got, err)
	}
	for _, raw := range []string{"0", "-1", "abc", "9223372036854775808"} {
		_, err := parseID(raw)
		if got := paramMessage(t, err); got != "id 必须是正整数" {
			t.Errorf("parseID(%q) = %q", raw, got)
		}
	}
}
