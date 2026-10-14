// Package testdb 为集成测试提供独立数据库：每个测试包使用自己的 geo_test_<name> 库，
// 不同的包可以并行运行，也不会碰到开发库。管理连接来自环境变量 TEST_DATABASE_URL。
package testdb

import (
	"context"
	"net/url"
	"os"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/migrations"
)

var validName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func URL(t testing.TB, name string) string {
	t.Helper()
	if !validName.MatchString(name) {
		t.Fatalf("testdb: invalid database name suffix %q", name)
	}
	admin := os.Getenv("TEST_DATABASE_URL")
	if admin == "" {
		t.Fatal("testdb: TEST_DATABASE_URL is not set (e.g. postgres://geo:geo@localhost:5432/geo?sslmode=disable)")
	}
	dbName := "geo_test_" + name

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("testdb: connect admin database: %v", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, dbName).Scan(&exists); err != nil {
		t.Fatalf("testdb: check database %s: %v", dbName, err)
	}
	if !exists {
		if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
			t.Fatalf("testdb: create database %s: %v", dbName, err)
		}
	}

	u, err := url.Parse(admin)
	if err != nil {
		t.Fatalf("testdb: parse TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + dbName
	// pgx 会用查询参数里的 dbname 或 database 覆盖路径中的库名。
	q := u.Query()
	q.Del("dbname")
	q.Del("database")
	u.RawQuery = q.Encode()
	dbURL := u.String()

	testConn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("testdb: connect %s: %v", dbName, err)
	}
	defer testConn.Close(ctx)
	var current string
	if err := testConn.QueryRow(ctx, `SELECT current_database()`).Scan(&current); err != nil {
		t.Fatalf("testdb: query current database of %s: %v", dbName, err)
	}
	if current != dbName {
		t.Fatalf("testdb: refusing to use %s: connected to database %q, want %q", u.Redacted(), current, dbName)
	}
	return dbURL
}

func New(t testing.TB, name string) *pgxpool.Pool {
	t.Helper()
	dbURL := URL(t, name)
	if err := migrations.Up(dbURL); err != nil {
		t.Fatalf("testdb: migrate %s: %v", name, err)
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("testdb: connect %s: %v", name, err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), `TRUNCATE poi, district RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("testdb: truncate %s: %v", name, err)
	}
	return pool
}
