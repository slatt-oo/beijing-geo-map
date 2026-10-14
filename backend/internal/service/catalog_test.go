package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

type fakeCategoryLister struct {
	categories []model.Category
	err        error
	calls      int
}

func (f *fakeCategoryLister) List(context.Context) ([]model.Category, error) {
	f.calls++
	return f.categories, f.err
}

func seededLister() *fakeCategoryLister {
	return &fakeCategoryLister{categories: []model.Category{
		{ID: 1, Code: "subway_station", Name: "地铁站", Color: "#E53935"},
		{ID: 5, Code: "park", Name: "公园", Color: "#43A047"},
	}}
}

func TestCatalogListMapsCategoriesAndCachesThem(t *testing.T) {
	lister := seededLister()
	catalog := NewCatalog(lister)

	for range 2 {
		got, err := catalog.List(context.Background())
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		want := []Category{{Code: "subway_station", Name: "地铁站", Color: "#E53935"}, {Code: "park", Name: "公园", Color: "#43A047"}}
		if !slices.Equal(got, want) {
			t.Fatalf("List = %+v, want %+v", got, want)
		}
	}
	if lister.calls != 1 {
		t.Fatalf("repository called %d times, want 1 (cached)", lister.calls)
	}
}

func TestCatalogIDsTranslatesCodes(t *testing.T) {
	catalog := NewCatalog(seededLister())

	ids, err := catalog.IDs(context.Background(), []string{"park", "subway_station"})
	if err != nil {
		t.Fatalf("IDs: %v", err)
	}
	if !slices.Equal(ids, []int16{5, 1}) {
		t.Fatalf("IDs = %v, want [5 1]", ids)
	}
}

func TestCatalogIDsReturnsNilForNoCodesWithoutLoading(t *testing.T) {
	lister := seededLister()
	ids, err := NewCatalog(lister).IDs(context.Background(), nil)
	if err != nil || ids != nil {
		t.Fatalf("IDs(nil) = %v, %v; want nil, nil", ids, err)
	}
	if lister.calls != 0 {
		t.Fatalf("repository called %d times, want 0", lister.calls)
	}
}

func TestCatalogIDsRejectsUnknownCode(t *testing.T) {
	_, err := NewCatalog(seededLister()).IDs(context.Background(), []string{"park", "zoo"})
	var unknown UnknownCategoryError
	if !errors.As(err, &unknown) || unknown.Code != "zoo" {
		t.Fatalf("err = %v, want UnknownCategoryError{zoo}", err)
	}
}

func TestCatalogRetriesAfterLoadFailure(t *testing.T) {
	lister := seededLister()
	lister.err = errors.New("database down")
	catalog := NewCatalog(lister)

	if _, err := catalog.List(context.Background()); err == nil {
		t.Fatal("expected an error while the database is down")
	}
	lister.err = nil
	got, err := catalog.List(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("List after recovery = %v, %v; want 2 categories", got, err)
	}
}
