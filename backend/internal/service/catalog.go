package service

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

type CategoryLister interface {
	List(ctx context.Context) ([]model.Category, error)
}

// Catalog 在第一次使用时从数据库加载分类并缓存；加载失败时下次调用会重试，
// 因此数据库暂时不可用不会妨碍服务启动。
type Catalog struct {
	repo CategoryLister

	mu         sync.Mutex
	loaded     bool
	categories []Category
	ids        map[string]int16
}

func NewCatalog(repo CategoryLister) *Catalog {
	return &Catalog{repo: repo}
}

func (c *Catalog) List(ctx context.Context) ([]Category, error) {
	if err := c.load(ctx); err != nil {
		return nil, err
	}
	return slices.Clone(c.categories), nil
}

// IDs 把分类编码转换为 ID。codes 为空时返回 nil，表示不按分类过滤。
func (c *Catalog) IDs(ctx context.Context, codes []string) ([]int16, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	if err := c.load(ctx); err != nil {
		return nil, err
	}
	ids := make([]int16, 0, len(codes))
	for _, code := range codes {
		id, ok := c.ids[code]
		if !ok {
			return nil, UnknownCategoryError{Code: code}
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *Catalog) load(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		return nil
	}
	rows, err := c.repo.List(ctx)
	if err != nil {
		return fmt.Errorf("load categories: %w", err)
	}
	c.categories = make([]Category, 0, len(rows))
	c.ids = make(map[string]int16, len(rows))
	for _, r := range rows {
		c.categories = append(c.categories, Category{Code: r.Code, Name: r.Name, Color: r.Color})
		c.ids[r.Code] = r.ID
	}
	c.loaded = true
	return nil
}
