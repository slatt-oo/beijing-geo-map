package migrations_test

import (
	"strings"
	"testing"

	"github.com/slatt-oo/beijing-geo-map/backend/migrations"
)

func TestUpRejectsNonPostgresURL(t *testing.T) {
	err := migrations.Up("mysql://user@localhost/db")
	if err == nil || !strings.Contains(err.Error(), "postgres://") {
		t.Fatalf("err = %v, want an error mentioning postgres://", err)
	}
}
