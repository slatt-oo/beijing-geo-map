package service

import "github.com/slatt-oo/beijing-geo-map/backend/internal/repository"

// ErrNotFound 与 repository.ErrNotFound 是同一个值，handler 只需要依赖 service 包。
var ErrNotFound = repository.ErrNotFound

type UnknownCategoryError struct {
	Code string
}

func (e UnknownCategoryError) Error() string {
	return "unknown category: " + e.Code
}
