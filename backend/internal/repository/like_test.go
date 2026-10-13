package repository

import "testing"

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"北京大学":   "北京大学",
		"100%":   `100\%`,
		"a_b":    `a\_b`,
		`C:\dir`: `C:\\dir`,
		`\%_`:    `\\\%\_`,
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}
