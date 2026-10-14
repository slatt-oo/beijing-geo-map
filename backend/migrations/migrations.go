// Package migrations 把数据库迁移 SQL 打包进二进制，并用 golang-migrate 执行。
package migrations

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var FS embed.FS

func Up(databaseURL string) error {
	target, err := pgx5URL(databaseURL)
	if err != nil {
		return err
	}
	src, err := iofs.New(FS, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, target)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// golang-migrate 的 pgx v5 驱动注册的协议名是 pgx5。
func pgx5URL(databaseURL string) (string, error) {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if rest, ok := strings.CutPrefix(databaseURL, prefix); ok {
			return "pgx5://" + rest, nil
		}
	}
	return "", errors.New("数据库连接串必须以 postgres:// 或 postgresql:// 开头")
}
