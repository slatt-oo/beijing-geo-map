# 阶段 0：仓库骨架 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搭好前后端和部署骨架：`docker compose up --build` 启动 `db`、`backend`、`frontend` 三个服务后，`/api/v1/health` 能报告数据库连接正常，前端页面能显示后端状态，GitHub Actions 三个任务全部通过。

**Architecture:** 后端是 Go + Gin，分为 `config`（读取环境变量）和 `handler`（统一响应格式、中间件、路由）两个包，`cmd/server` 负责组装依赖和优雅关闭。前端是 React 19 + Vite 8，`src/api/client.ts` 封装请求和错误解析。nginx 托管前端构建产物，并把 `/api/` 反向代理到后端。

**Tech Stack:** Go 1.26、Gin v1.12.0、gin-contrib/gzip v1.2.8、pgx v5.11.0；React 19.3、Vite 8.3、TypeScript ~6.0.3、Vitest 5、ESLint 10、Tailwind CSS 4.3；Docker Compose；GitHub Actions。

**Spec:** `docs/01-implementation-plan.md`（第 5 节「阶段 0」）、`docs/02-technical-design.md`（第 2、3、5.1 到 5.3、8 节）

## Global Constraints

- 所有工作在 `phase-0-skeleton` 分支上进行，每个任务结束时提交一次。仓库的 git 提交身份已经配置为 `slatt-oo <177398036+slatt-oo@users.noreply.github.com>`，不要修改。
- Go 模块路径：`github.com/slatt-oo/beijing-geo-map/backend`；`go.mod` 声明 `go 1.26.0`。
- 本机 Go 是 1.21.0，`GOTOOLCHAIN=auto`。在 `backend/` 下执行 go 命令时，会自动下载并切换到 1.26 工具链，不要手动安装 Go。
- Go 依赖版本固定为：`github.com/gin-gonic/gin@v1.12.0`、`github.com/gin-contrib/gzip@v1.2.8`、`github.com/jackc/pgx/v5@v5.11.0`。后端测试只用标准库 `testing`，不引入 testify。
- 前端依赖版本以本计划中的 `package.json` 为准。TypeScript 必须是 `~6.0.3`，不能用 7.x，因为 typescript-eslint 8.71 的 peer 依赖要求 TypeScript `>=4.8.4 <6.1.0`。
- `frontend/.npmrc` 必须固定 `registry=https://registry.npmjs.org/`。本机全局 npm 源是一个私有仓库，`package-lock.json` 中不能出现 `packages.aliyun.com`。
- Docker 镜像：`postgis/postgis:16-3.4`（只有 amd64 版本，compose 中声明 `platform: linux/amd64`）、`golang:1.26-alpine`、`alpine:3.22`、`node:24-alpine`、`nginx:1.29-alpine`，以上镜像本机都已拉取。
- 接口约定（摘自技术方案 5.3 节）：所有接口挂在 `/api/v1` 下。非 GeoJSON 的成功响应为 `{"data": ...}`，错误响应为 `{"error": {"code": "...", "message": "..."}}`。错误码有四种：`INVALID_PARAM` 对应 400，`NOT_FOUND` 对应 404，`INTERNAL` 对应 500（message 固定为 `服务器内部错误`），`UNAVAILABLE` 对应 503。
- 中间件（摘自技术方案 5.2 节）：slog JSON 访问日志、panic 恢复、每个请求 5 秒超时、gzip。优雅关闭最长等待 10 秒。
- 环境变量默认值（摘自技术方案 3.2 节）：`HTTP_ADDR=:8000`、`LOG_LEVEL=info`。compose 中 `DATABASE_URL=postgres://geo:geo@db:5432/geo?sslmode=disable`，`FRONTEND_PORT` 默认 `8080`，`DB_PORT` 默认 `5432`。
- 本机 8080 端口已被其他进程占用。本地验证 compose 时使用 `FRONTEND_PORT=18080`，但默认值仍保持 `8080`。
- 代码注释只用来说明代码本身体现不出来的约束，不写「这一行做什么」。
- **沙箱说明：** Shell 默认在沙箱中运行。凡是需要访问网络（Go 工具链和模块下载、npm install）、操作 Docker，或者写入工作区以外目录的命令，都要以 `required_permissions: ["all"]` 运行。

---

### Task 1: 后端模块与配置加载

**Files:**
- Create: `backend/go.mod`
- Create: `backend/internal/config/config.go`
- Test: `backend/internal/config/config_test.go`

**Interfaces:**
- Consumes: 无
- Produces:
  - `type config.Config struct { DatabaseURL string; HTTPAddr string; LogLevel slog.Level }`
  - `func config.Load() (config.Config, error)`：从进程环境变量读取配置

- [ ] **Step 1: 初始化 Go 模块并确认工具链会自动切换**

```bash
mkdir -p backend && cd backend
go mod init github.com/slatt-oo/beijing-geo-map/backend
go mod edit -go=1.26.0
go version
```

Expected: `go version` 输出 `go version go1.26.x darwin/arm64`，说明已经自动切换到 1.26 工具链。`backend/go.mod` 的内容为：

```text
module github.com/slatt-oo/beijing-geo-map/backend

go 1.26.0
```

- [ ] **Step 2: 编写失败的测试**

创建 `backend/internal/config/config_test.go`：

```go
package config

import (
	"log/slog"
	"strings"
	"testing"
)

func envFrom(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{"DATABASE_URL": "postgres://u:p@h:5432/d"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://u:p@h:5432/d" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.HTTPAddr != ":8000" {
		t.Errorf("HTTPAddr = %q, want :8000", cfg.HTTPAddr)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want INFO", cfg.LogLevel)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{
		"DATABASE_URL": "postgres://x",
		"HTTP_ADDR":    ":9000",
		"LOG_LEVEL":    "DEBUG",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":9000" {
		t.Errorf("HTTPAddr = %q, want :9000", cfg.HTTPAddr)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want DEBUG", cfg.LogLevel)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	_, err := load(envFrom(map[string]string{}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v, want an error mentioning DATABASE_URL", err)
	}
}

func TestLoadRejectsUnknownLogLevel(t *testing.T) {
	_, err := load(envFrom(map[string]string{"DATABASE_URL": "postgres://x", "LOG_LEVEL": "verbose"}))
	if err == nil || !strings.Contains(err.Error(), "LOG_LEVEL") {
		t.Fatalf("err = %v, want an error mentioning LOG_LEVEL", err)
	}
}
```

- [ ] **Step 3: 运行测试，确认失败**

Run: `cd backend && go test ./internal/config/`
Expected: 编译失败，报错包含 `undefined: load`。

- [ ] **Step 4: 编写最小实现**

创建 `backend/internal/config/config.go`：

```go
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	HTTPAddr    string
	LogLevel    slog.Level
}

func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL: getenv("DATABASE_URL"),
		HTTPAddr:    getenv("HTTP_ADDR"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL 未设置")
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8000"
	}
	level, err := parseLevel(getenv("LOG_LEVEL"))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = level
	return cfg, nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL 取值无效: %q（可选 debug/info/warn/error）", s)
	}
}
```

- [ ] **Step 5: 运行测试，确认通过**

Run: `cd backend && go test ./internal/config/ -v && go vet ./... && test -z "$(gofmt -l .)"`
Expected: 4 个测试全部 PASS，`go vet` 没有输出，`gofmt -l` 没有输出。

- [ ] **Step 6: 提交**

```bash
git add backend/go.mod backend/internal/config
git commit -m "feat(backend): add Go module and env-based config loading"
```

---

### Task 2: 后端 HTTP 层（统一响应、健康检查、中间件、路由）

**Files:**
- Modify: `backend/go.mod`、`backend/go.sum`（新增依赖）
- Create: `backend/internal/handler/response.go`
- Create: `backend/internal/handler/health.go`
- Create: `backend/internal/handler/middleware.go`
- Create: `backend/internal/handler/router.go`
- Test: `backend/internal/handler/router_test.go`

**Interfaces:**
- Consumes: 无
- Produces（后续阶段的 handler 都依赖这些）：
  - 错误码常量：`handler.CodeInvalidParam = "INVALID_PARAM"`、`handler.CodeNotFound = "NOT_FOUND"`、`handler.CodeInternal = "INTERNAL"`、`handler.CodeUnavailable = "UNAVAILABLE"`
  - 包内辅助函数：`writeData(c *gin.Context, data any)`，返回 200 和 `{"data": data}`；`writeError(c *gin.Context, status int, code, message string)`；`writeInternal(c *gin.Context)`
  - `type handler.Pinger interface { Ping(ctx context.Context) error }`，`*pgxpool.Pool` 天然满足这个接口
  - `type handler.Deps struct { DB Pinger }`，后续阶段会继续往里加 service 字段
  - `func handler.NewRouter(deps handler.Deps) *gin.Engine`

- [ ] **Step 1: 添加依赖**

```bash
cd backend
go get github.com/gin-gonic/gin@v1.12.0 github.com/gin-contrib/gzip@v1.2.8
```

Expected: `go.mod` 的 require 中出现这两个模块，并生成 `go.sum`。

- [ ] **Step 2: 编写失败的测试**

创建 `backend/internal/handler/router_test.go`：

```go
package handler

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func get(t *testing.T, h http.Handler, path string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for key, values := range header {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("response is not JSON: %v; body=%s", err, body)
	}
	return m
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, status, rec.Body.String())
	}
	errObj, ok := decodeJSON(t, rec.Body.Bytes())["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object; body=%s", rec.Body.String())
	}
	if errObj["code"] != code || errObj["message"] != message {
		t.Fatalf("error = %v, want code=%q message=%q", errObj, code, message)
	}
}

func TestHealthOK(t *testing.T) {
	rec := get(t, NewRouter(Deps{DB: fakePinger{}}), "/api/v1/health", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	data, ok := decodeJSON(t, rec.Body.Bytes())["data"].(map[string]any)
	if !ok || data["status"] != "ok" || data["db"] != "ok" {
		t.Fatalf("body = %s, want data.status=ok and data.db=ok", rec.Body.String())
	}
}

func TestHealthReportsUnavailableWhenDatabaseIsDown(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{err: errors.New("connection refused")}})

	rec := get(t, r, "/api/v1/health", nil)

	assertErrorResponse(t, rec, http.StatusServiceUnavailable, CodeUnavailable, "数据库不可用")
}

func TestUnknownRouteReturnsUnifiedNotFound(t *testing.T) {
	rec := get(t, NewRouter(Deps{DB: fakePinger{}}), "/api/v1/does-not-exist", nil)

	assertErrorResponse(t, rec, http.StatusNotFound, CodeNotFound, "接口不存在")
}

func TestPanicIsRecoveredAsUnifiedInternalError(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{}})
	r.GET("/api/v1/test-panic", func(*gin.Context) { panic("secret detail") })

	rec := get(t, r, "/api/v1/test-panic", nil)

	assertErrorResponse(t, rec, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	if strings.Contains(rec.Body.String(), "secret detail") {
		t.Fatalf("panic detail leaked to client: %s", rec.Body.String())
	}
}

func TestRequestContextCarriesFiveSecondTimeout(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{}})
	var remaining time.Duration
	var hasDeadline bool
	r.GET("/api/v1/test-deadline", func(c *gin.Context) {
		var deadline time.Time
		deadline, hasDeadline = c.Request.Context().Deadline()
		remaining = time.Until(deadline)
		c.Status(http.StatusNoContent)
	})

	get(t, r, "/api/v1/test-deadline", nil)

	if !hasDeadline {
		t.Fatal("request context has no deadline")
	}
	if remaining <= 4*time.Second || remaining > 5*time.Second {
		t.Fatalf("remaining = %v, want within (4s, 5s]", remaining)
	}
}

func TestLargeResponsesAreGzipped(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{}})
	payload := strings.Repeat("北京", 1000)
	r.GET("/api/v1/test-large", func(c *gin.Context) { writeData(c, payload) })

	rec := get(t, r, "/api/v1/test-large", http.Header{"Accept-Encoding": {"gzip"}})

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("body is not gzip: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if got := decodeJSON(t, body)["data"]; got != payload {
		t.Fatalf("decompressed data mismatch (got %d bytes)", len(body))
	}
}

func TestSmallResponsesAreNotGzipped(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{}})

	rec := get(t, r, "/api/v1/health", http.Header{"Accept-Encoding": {"gzip"}})

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want none for responses under 1 KB", got)
	}
}
```

- [ ] **Step 3: 运行测试，确认失败**

Run: `cd backend && go test ./internal/handler/`
Expected: 编译失败，报错包含 `undefined: NewRouter`、`undefined: Deps` 等。

- [ ] **Step 4: 编写实现**

创建 `backend/internal/handler/response.go`：

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CodeInvalidParam = "INVALID_PARAM"
	CodeNotFound     = "NOT_FOUND"
	CodeInternal     = "INTERNAL"
	CodeUnavailable  = "UNAVAILABLE"
)

const internalErrorMessage = "服务器内部错误"

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeData(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": errorBody{Code: code, Message: message}})
}

func writeInternal(c *gin.Context) {
	writeError(c, http.StatusInternalServerError, CodeInternal, internalErrorMessage)
}
```

创建 `backend/internal/handler/health.go`：

```go
package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

func healthHandler(db Pinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := db.Ping(c.Request.Context()); err != nil {
			slog.ErrorContext(c.Request.Context(), "health check: database ping failed", "err", err)
			writeError(c, http.StatusServiceUnavailable, CodeUnavailable, "数据库不可用")
			return
		}
		writeData(c, gin.H{"status": "ok", "db": "ok"})
	}
}
```

创建 `backend/internal/handler/middleware.go`：

```go
package handler

import (
	"context"
	"io"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.InfoContext(c.Request.Context(), "request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query", c.Request.URL.RawQuery,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
		)
	}
}

func recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		slog.ErrorContext(c.Request.Context(), "panic recovered",
			"panic", recovered,
			"stack", string(debug.Stack()),
		)
		writeInternal(c)
	})
}

func requestTimeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
```

创建 `backend/internal/handler/router.go`：

```go
package handler

import (
	"net/http"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

const (
	requestTimeoutDuration = 5 * time.Second
	gzipMinLength          = 1024
)

type Deps struct {
	DB Pinger
}

func NewRouter(deps Deps) *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	// recovery 必须位于 gzip 内层：panic 后写出的 500 响应要经过仍然有效的 gzip writer。
	r.Use(
		requestLogger(),
		gzip.Gzip(gzip.DefaultCompression, gzip.WithMinLength(gzipMinLength)),
		recovery(),
		requestTimeout(requestTimeoutDuration),
	)
	r.NoRoute(func(c *gin.Context) {
		writeError(c, http.StatusNotFound, CodeNotFound, "接口不存在")
	})

	api := r.Group("/api/v1")
	api.GET("/health", healthHandler(deps.DB))
	return r
}
```

- [ ] **Step 5: 运行测试，确认通过**

Run: `cd backend && go mod tidy && go test ./... -v && go vet ./... && test -z "$(gofmt -l .)"`
Expected: `config` 包 4 个测试、`handler` 包 7 个测试全部 PASS，测试输出中没有日志噪音，`go vet` 和 `gofmt -l` 都没有输出。

- [ ] **Step 6: 提交**

```bash
git add backend/go.mod backend/go.sum backend/internal/handler
git commit -m "feat(backend): add router with unified errors, health check and middleware"
```

---

### Task 3: 后端服务入口与镜像

**Files:**
- Modify: `backend/go.mod`、`backend/go.sum`（新增 pgx）
- Create: `backend/cmd/server/main.go`
- Create: `backend/Dockerfile`
- Create: `backend/.dockerignore`

**Interfaces:**
- Consumes: `config.Load()`、`handler.NewRouter(handler.Deps{DB: ...})`
- Produces:
  - 镜像中的可执行文件 `/usr/local/bin/server`，默认 `CMD ["server"]`。Dockerfile 用 `go build -o /out/ ./cmd/...` 一次构建 `cmd/` 下的所有程序，阶段 1 新增的 `cmd/importer` 会自动被打包进去。
  - 构建参数 `GOPROXY`，默认 `https://proxy.golang.org,direct`

`main.go` 只负责组装依赖，没有可以单独测试的逻辑。这个任务的验证方式是编译、构建镜像，再做一次冒烟运行，用一个不可达的数据库地址验证 503 分支和优雅关闭。

- [ ] **Step 1: 添加 pgx 依赖**

```bash
cd backend && go get github.com/jackc/pgx/v5@v5.11.0
```

- [ ] **Step 2: 编写服务入口**

创建 `backend/cmd/server/main.go`：

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/config"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/handler"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))
	gin.SetMode(gin.ReleaseMode)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}
	defer pool.Close()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler.NewRouter(handler.Deps{DB: pool}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", cfg.HTTPAddr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	slog.Info("shutting down", "timeout", shutdownTimeout.String())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: 整理依赖并编译**

Run: `cd backend && go mod tidy && go build ./... && go vet ./... && test -z "$(gofmt -l .)" && go test ./...`
Expected: 全部成功，没有输出错误；`go.mod` 的 require 中出现 `github.com/jackc/pgx/v5 v5.11.0`。

- [ ] **Step 4: 编写 Dockerfile 和 .dockerignore**

创建 `backend/Dockerfile`：

```dockerfile
FROM golang:1.26-alpine AS build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY} CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/...

FROM alpine:3.22
RUN adduser -D -H -u 10001 app
COPY --from=build /out/ /usr/local/bin/
USER app
EXPOSE 8000
CMD ["server"]
```

创建 `backend/.dockerignore`：

```text
bin/
*.test
coverage.*
```

- [ ] **Step 5: 构建镜像并冒烟运行**

```bash
docker build --build-arg GOPROXY=https://goproxy.cn,direct -t bgm-backend:dev backend
docker run -d --name bgm-backend-smoke -p 18000:8000 \
  -e DATABASE_URL='postgres://geo:geo@127.0.0.1:1/geo?sslmode=disable' bgm-backend:dev
sleep 2
curl -s -w '\nHTTP %{http_code}\n' localhost:18000/api/v1/health
docker stop bgm-backend-smoke
docker logs bgm-backend-smoke 2>&1 | tail -5
docker rm bgm-backend-smoke
```

Expected:
- `curl` 输出 `{"error":{"code":"UNAVAILABLE","message":"数据库不可用"}}` 和 `HTTP 503`。
- `docker logs` 中依次出现 JSON 日志 `"msg":"http server listening"`、一条 `"msg":"request"`（`"status":503`）和 `"msg":"shutting down"`。
- `docker stop` 在 10 秒内返回。

- [ ] **Step 6: 提交**

```bash
git add backend/go.mod backend/go.sum backend/cmd backend/Dockerfile backend/.dockerignore
git commit -m "feat(backend): add server entrypoint with graceful shutdown and Dockerfile"
```

---

### Task 4: 前端骨架与接口客户端

**Files:**
- Create: `frontend/.npmrc`
- Create: `frontend/package.json`、`frontend/package-lock.json`（由 npm 生成）
- Create: `frontend/index.html`
- Create: `frontend/vite.config.ts`
- Create: `frontend/tsconfig.json`、`frontend/tsconfig.app.json`、`frontend/tsconfig.node.json`
- Create: `frontend/eslint.config.js`
- Create: `frontend/src/index.css`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/App.tsx`
- Create: `frontend/src/api/client.ts`
- Test: `frontend/src/api/client.test.ts`

**Interfaces:**
- Consumes: 后端 `GET /api/v1/health`，响应为 `{"data":{"status":"ok","db":"ok"}}`
- Produces（后续阶段的 API hooks 都依赖这些）：
  - `class ApiError extends Error { readonly status: number; readonly code: string }`。网络失败时 `status=0`、`code='NETWORK'`；响应体不是标准错误格式时 `code='HTTP_ERROR'`
  - `type QueryValue = string | number | readonly (string | number)[] | null | undefined`
  - `function buildUrl(path: string, params?: Record<string, QueryValue>): string`：加上 `/api/v1` 前缀；忽略 `null`、`undefined` 和空数组；数组用逗号拼接
  - `function apiGet<T>(path: string, params?: Record<string, QueryValue>, signal?: AbortSignal): Promise<T>`
  - npm scripts：`dev`、`build`、`preview`、`lint`、`typecheck`、`test`

- [ ] **Step 1: 创建 npm 源配置和 package.json，安装依赖**

创建 `frontend/.npmrc`：

```text
registry=https://registry.npmjs.org/
```

创建 `frontend/package.json`：

```json
{
  "name": "beijing-geo-map-frontend",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "engines": {
    "node": ">=24"
  },
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview",
    "lint": "eslint .",
    "typecheck": "tsc -b",
    "test": "vitest run"
  },
  "dependencies": {
    "react": "^19.3.0",
    "react-dom": "^19.3.0"
  },
  "devDependencies": {
    "@eslint/js": "^10.0.1",
    "@tailwindcss/vite": "^4.3.3",
    "@types/node": "^24.0.0",
    "@types/react": "^19.3.0",
    "@types/react-dom": "^19.3.0",
    "@vitejs/plugin-react": "^6.1.2",
    "eslint": "^10.12.0",
    "eslint-plugin-react-hooks": "^7.1.1",
    "globals": "^17.13.0",
    "tailwindcss": "^4.3.3",
    "typescript": "~6.0.3",
    "typescript-eslint": "^8.71.1",
    "vite": "^8.3.4",
    "vitest": "^5.0.3"
  }
}
```

然后执行：

```bash
cd frontend && npm install
grep -c 'packages.aliyun.com' package-lock.json || true
grep -c 'registry.npmjs.org' package-lock.json
```

Expected: `npm install` 成功，没有 `ERESOLVE` 报错（`Unknown env config "devdir"` 这条警告来自本机全局配置，可以忽略）；第一个 grep 输出 `0`，第二个 grep 输出大于 0 的数字。

- [ ] **Step 2: 创建构建、类型检查和 lint 配置**

创建 `frontend/index.html`：

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>北京地图 Demo</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

创建 `frontend/vite.config.ts`：

```ts
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api': 'http://localhost:8000',
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
```

创建 `frontend/tsconfig.json`：

```json
{
  "files": [],
  "references": [{ "path": "./tsconfig.app.json" }, { "path": "./tsconfig.node.json" }]
}
```

创建 `frontend/tsconfig.app.json`：

```json
{
  "compilerOptions": {
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.app.tsbuildinfo",
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "types": ["vite/client"],
    "strict": true,
    "noEmit": true,
    "skipLibCheck": true,
    "isolatedModules": true,
    "verbatimModuleSyntax": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src"]
}
```

创建 `frontend/tsconfig.node.json`：

```json
{
  "compilerOptions": {
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.node.tsbuildinfo",
    "target": "ES2023",
    "lib": ["ES2023"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "types": ["node"],
    "strict": true,
    "noEmit": true,
    "skipLibCheck": true,
    "isolatedModules": true,
    "verbatimModuleSyntax": true
  },
  "include": ["vite.config.ts"]
}
```

创建 `frontend/eslint.config.js`：

```js
import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [js.configs.recommended, tseslint.configs.recommended, reactHooks.configs.flat.recommended],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
    },
  },
])
```

创建 `frontend/src/index.css`：

```css
@import "tailwindcss";

html,
body,
#root {
  height: 100%;
  margin: 0;
}
```

创建 `frontend/src/main.tsx`：

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

- [ ] **Step 3: 编写失败的测试**

创建 `frontend/src/api/client.test.ts`：

```ts
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiGet, buildUrl } from './client'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('buildUrl', () => {
  it('returns the bare path under /api/v1 when there are no params', () => {
    expect(buildUrl('/health')).toBe('/api/v1/health')
  })

  it('encodes params and skips null, undefined and empty arrays', () => {
    const url = buildUrl('/pois/search', { q: '北京', limit: 10, categories: [], radius: undefined, lng: null })
    expect(url).toBe('/api/v1/pois/search?q=%E5%8C%97%E4%BA%AC&limit=10')
  })

  it('joins array params with commas', () => {
    expect(buildUrl('/pois', { categories: ['park', 'university'] })).toBe(
      '/api/v1/pois?categories=park%2Cuniversity',
    )
  })
})

describe('apiGet', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('requests the built URL and returns the parsed body', async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse(200, { data: { status: 'ok', db: 'ok' } }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(apiGet('/health', { verbose: 1 })).resolves.toEqual({ data: { status: 'ok', db: 'ok' } })
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/health?verbose=1', expect.anything())
  })

  it('throws ApiError carrying the server error code and message', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(400, { error: { code: 'INVALID_PARAM', message: 'lat 必须在 -90 到 90 之间' } })),
    )

    const err = await apiGet('/districts/locate').catch((e: unknown) => e)

    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 400, code: 'INVALID_PARAM', message: 'lat 必须在 -90 到 90 之间' })
  })

  it('falls back to HTTP_ERROR when the error body is not the standard format', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>Bad Gateway</html>', { status: 502 })))

    await expect(apiGet('/health')).rejects.toMatchObject({ status: 502, code: 'HTTP_ERROR' })
  })

  it('maps network failures to a NETWORK ApiError', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('fetch failed')
      }),
    )

    await expect(apiGet('/health')).rejects.toMatchObject({ status: 0, code: 'NETWORK' })
  })

  it('rethrows aborts unchanged so callers can ignore them', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new DOMException('The operation was aborted.', 'AbortError')
      }),
    )

    await expect(apiGet('/health')).rejects.toMatchObject({ name: 'AbortError' })
  })
})
```

- [ ] **Step 4: 运行测试，确认失败**

Run: `cd frontend && npm test`
Expected: FAIL，报错为无法解析 `./client` 模块（`Failed to resolve import "./client"` 或类似信息）。

- [ ] **Step 5: 编写实现**

创建 `frontend/src/api/client.ts`：

```ts
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export type QueryValue = string | number | readonly (string | number)[] | null | undefined

const API_BASE = '/api/v1'

export function buildUrl(path: string, params: Record<string, QueryValue> = {}): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === null || value === undefined) continue
    if (Array.isArray(value)) {
      if (value.length > 0) search.set(key, value.join(','))
      continue
    }
    search.set(key, String(value))
  }
  const qs = search.toString()
  return qs ? `${API_BASE}${path}?${qs}` : `${API_BASE}${path}`
}

export async function apiGet<T>(path: string, params?: Record<string, QueryValue>, signal?: AbortSignal): Promise<T> {
  let res: Response
  try {
    res = await fetch(buildUrl(path, params), { signal, headers: { Accept: 'application/json' } })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, 'NETWORK', '网络错误，请确认后端服务已启动')
  }
  if (!res.ok) throw await toApiError(res)
  return (await res.json()) as T
}

async function toApiError(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as { error?: { code?: unknown; message?: unknown } }
    const { code, message } = body.error ?? {}
    if (typeof code === 'string' && typeof message === 'string') {
      return new ApiError(res.status, code, message)
    }
  } catch {
    // 非 JSON 响应体（例如 nginx 的 502 页面）走下面的通用错误
  }
  return new ApiError(res.status, 'HTTP_ERROR', `请求失败（HTTP ${res.status}）`)
}
```

- [ ] **Step 6: 运行测试，确认通过**

Run: `cd frontend && npm test`
Expected: 8 个测试全部 PASS。

- [ ] **Step 7: 编写显示后端状态的首页**

创建 `frontend/src/App.tsx`：

```tsx
import { useEffect, useState } from 'react'
import { ApiError, apiGet } from './api/client'

type HealthResponse = { data: { status: string; db: string } }

type HealthState = { kind: 'loading' } | { kind: 'ok' } | { kind: 'error'; message: string }

export default function App() {
  const [health, setHealth] = useState<HealthState>({ kind: 'loading' })

  useEffect(() => {
    const controller = new AbortController()
    apiGet<HealthResponse>('/health', undefined, controller.signal)
      .then(() => setHealth({ kind: 'ok' }))
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setHealth({ kind: 'error', message: err instanceof ApiError ? err.message : String(err) })
      })
    return () => controller.abort()
  }, [])

  return (
    <main className="flex h-full flex-col items-center justify-center gap-3 bg-slate-50 text-slate-800">
      <h1 className="text-2xl font-semibold">北京地图 Demo</h1>
      <p className="text-sm">
        后端状态：
        {health.kind === 'loading' && <span className="text-slate-500">检查中…</span>}
        {health.kind === 'ok' && <span className="text-emerald-600">正常</span>}
        {health.kind === 'error' && <span className="text-red-600">{health.message}</span>}
      </p>
    </main>
  )
}
```

- [ ] **Step 8: 运行全部前端检查**

Run: `cd frontend && npm run lint && npm run typecheck && npm test && npm run build`
Expected: lint 和 typecheck 没有错误；8 个测试 PASS；`vite build` 成功，生成 `dist/index.html` 和 `dist/assets/*`。

- [ ] **Step 9: 提交**

```bash
git add frontend
git commit -m "feat(frontend): add Vite + React skeleton with API client and health status page"
```

（`node_modules/` 和 `dist/` 已被根目录 `.gitignore` 忽略。提交前用 `git status --short` 确认没有带上这两个目录。）

---

### Task 5: 前端镜像（nginx 托管 + 反向代理）

**Files:**
- Create: `frontend/Dockerfile`
- Create: `frontend/nginx.conf`
- Create: `frontend/.dockerignore`

**Interfaces:**
- Consumes: npm scripts `build`；`frontend/.npmrc`
- Produces:
  - 镜像监听 80 端口，托管 `dist/`，未知路径回落到 `index.html`；`/api/` 反向代理到 `http://backend:8000`，原始 URI 保持不变
  - 构建参数 `NPM_REGISTRY`，默认 `https://registry.npmjs.org/`

- [ ] **Step 1: 编写 nginx 配置**

创建 `frontend/nginx.conf`：

```nginx
server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    gzip on;
    gzip_types text/css application/javascript application/json image/svg+xml;

    # 通过 Docker 内置 DNS 动态解析 backend：nginx 不会因为 backend 尚未启动而启动失败，backend 容器重建后 IP 变化也不会导致 502。
    resolver 127.0.0.11 valid=10s ipv6=off;
    set $backend_upstream http://backend:8000;

    location /api/ {
        proxy_pass $backend_upstream;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

- [ ] **Step 2: 编写 Dockerfile 和 .dockerignore**

创建 `frontend/Dockerfile`：

```dockerfile
FROM node:24-alpine AS build
ARG NPM_REGISTRY=https://registry.npmjs.org/
WORKDIR /app
COPY package.json package-lock.json .npmrc ./
RUN npm ci --registry=${NPM_REGISTRY}
COPY . .
RUN npm run build

FROM nginx:1.29-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
```

创建 `frontend/.dockerignore`：

```text
node_modules/
dist/
```

- [ ] **Step 3: 构建镜像并验证**

```bash
docker build -t bgm-frontend:dev frontend
docker run --rm bgm-frontend:dev nginx -t
docker run -d --name bgm-frontend-smoke -p 18081:80 bgm-frontend:dev
sleep 1
curl -s localhost:18081/ | grep -o '<title>.*</title>'
curl -s localhost:18081/some/spa/route | grep -o '<title>.*</title>'
docker rm -f bgm-frontend-smoke
```

Expected:
- `nginx -t` 输出 `syntax is ok` 和 `test is successful`。
- 两次 `curl` 都输出 `<title>北京地图 Demo</title>`，说明 SPA 路由回落生效。

- [ ] **Step 4: 提交**

```bash
git add frontend/Dockerfile frontend/nginx.conf frontend/.dockerignore
git commit -m "feat(frontend): add nginx image serving the SPA and proxying /api"
```

---

### Task 6: Compose 编排与一键启动

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`
- Create: `Makefile`

**Interfaces:**
- Consumes: `backend/Dockerfile`（构建参数 `GOPROXY`，镜像中有 `server`）、`frontend/Dockerfile`（构建参数 `NPM_REGISTRY`）
- Produces:
  - compose 服务 `db`、`backend`、`frontend`；命名卷 `pgdata`。阶段 1 会加入 `seed` 服务
  - Make 目标：`up`、`down`、`logs`、`dev-db`、`run-backend`、`run-frontend`、`test`、`test-backend`、`test-frontend`、`test-integration`、`lint`

- [ ] **Step 1: 编写 docker-compose.yml**

创建 `docker-compose.yml`：

```yaml
name: beijing-geo-map

services:
  db:
    image: postgis/postgis:16-3.4
    platform: linux/amd64
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-geo}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-geo}
      POSTGRES_DB: ${POSTGRES_DB:-geo}
    ports:
      - "${DB_PORT:-5432}:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      # 必须走 TCP：初始化阶段的临时实例只监听 unix socket，这样可以避免初始化尚未完成就被判定为健康。
      test: ["CMD-SHELL", "pg_isready -h 127.0.0.1 -U $${POSTGRES_USER} -d $${POSTGRES_DB}"]
      interval: 5s
      timeout: 5s
      retries: 30

  backend:
    build:
      context: ./backend
      args:
        GOPROXY: ${GOPROXY:-https://goproxy.cn,direct}
    environment:
      DATABASE_URL: postgres://${POSTGRES_USER:-geo}:${POSTGRES_PASSWORD:-geo}@db:5432/${POSTGRES_DB:-geo}?sslmode=disable
      HTTP_ADDR: ":8000"
      LOG_LEVEL: ${LOG_LEVEL:-info}
    depends_on:
      db:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:8000/api/v1/health >/dev/null || exit 1"]
      interval: 5s
      timeout: 3s
      retries: 20

  frontend:
    build:
      context: ./frontend
      args:
        NPM_REGISTRY: ${NPM_REGISTRY:-https://registry.npmjs.org/}
    ports:
      - "${FRONTEND_PORT:-8080}:80"
    depends_on:
      backend:
        condition: service_healthy

volumes:
  pgdata:
```

- [ ] **Step 2: 编写 .env.example**

创建 `.env.example`：

```text
# 复制为 .env 后按需修改；没有 .env 时 docker compose 使用下面这些默认值。
POSTGRES_USER=geo
POSTGRES_PASSWORD=geo
POSTGRES_DB=geo
DB_PORT=5432
FRONTEND_PORT=8080
LOG_LEVEL=info

# 构建镜像时使用的依赖源。国内网络 Go 模块代理可以保持 goproxy.cn，
# npm 可以改成 https://registry.npmmirror.com/
GOPROXY=https://goproxy.cn,direct
NPM_REGISTRY=https://registry.npmjs.org/
```

- [ ] **Step 3: 编写 Makefile**

创建 `Makefile`（命令行必须以 Tab 缩进）：

```makefile
.PHONY: up down logs dev-db run-backend run-frontend test test-backend test-frontend test-integration lint

DEV_DATABASE_URL ?= postgres://geo:geo@localhost:5432/geo?sslmode=disable

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

dev-db:
	docker compose up -d db

run-backend:
	cd backend && DATABASE_URL="$(DEV_DATABASE_URL)" go run ./cmd/server

run-frontend:
	cd frontend && npm run dev

test: test-backend test-frontend

test-backend:
	cd backend && go test ./...

test-frontend:
	cd frontend && npm test

test-integration:
	cd backend && DATABASE_URL="$(DEV_DATABASE_URL)" go test -tags=integration ./...

lint:
	cd backend && test -z "$$(gofmt -l .)" && go vet ./...
	cd frontend && npm run lint && npm run typecheck
```

- [ ] **Step 4: 校验 compose 文件并一键启动**

```bash
docker compose config --quiet && echo "compose config ok"
FRONTEND_PORT=18080 docker compose up --build -d
for i in $(seq 1 60); do curl -fsS localhost:18080/api/v1/health && break; sleep 5; done; echo
docker compose ps
```

Expected:
- 先输出 `compose config ok`。
- 循环最终打印 `{"data":{"db":"ok","status":"ok"}}`。
- `docker compose ps` 中 `db` 和 `backend` 的状态为 `healthy`，`frontend` 为 `running`。

- [ ] **Step 5: 验证代理、前端页面和统一 404**

```bash
curl -s localhost:18080/ | grep -o '<title>.*</title>'
curl -s -w '\nHTTP %{http_code}\n' localhost:18080/api/v1/does-not-exist
docker compose logs backend 2>&1 | grep '"msg":"request"' | tail -2
```

Expected:
- 输出 `<title>北京地图 Demo</title>`。
- 输出 `{"error":{"code":"NOT_FOUND","message":"接口不存在"}}` 和 `HTTP 404`。
- 后端日志中有 JSON 格式的 `request` 记录，包含 `"path":"/api/v1/does-not-exist"` 和 `"status":404`。

- [ ] **Step 6: 验证 Make 目标，然后关闭服务**

```bash
make test
make lint
docker compose down
```

Expected: `make test` 中后端和前端的测试全部通过；`make lint` 没有错误；`docker compose down` 成功停止并删除容器（保留 `pgdata` 卷）。

- [ ] **Step 7: 提交**

```bash
git add docker-compose.yml .env.example Makefile
git commit -m "feat: add docker compose stack, env example and Makefile"
```

---

### Task 7: CI 与文档更新

**Files:**
- Create: `.github/workflows/ci.yml`
- Modify: `README.md`
- Modify: `docs/01-implementation-plan.md`
- Modify: `docs/02-technical-design.md`

**Interfaces:**
- Consumes: Make 目标与 npm scripts（`lint`、`typecheck`、`test`、`build`）、`backend/go.mod`、`frontend/package-lock.json`
- Produces: GitHub Actions 工作流 `CI`，包含 `backend`、`frontend`、`docker` 三个任务；README 中的 CI 徽章和快速开始说明

- [ ] **Step 1: 编写 CI 工作流**

创建 `.github/workflows/ci.yml`：

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  backend:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: backend
    services:
      db:
        image: postgis/postgis:16-3.4
        env:
          POSTGRES_USER: geo
          POSTGRES_PASSWORD: geo
          POSTGRES_DB: geo
        ports:
          - 5432:5432
        options: >-
          --health-cmd "pg_isready -h 127.0.0.1 -U geo -d geo"
          --health-interval 5s
          --health-timeout 5s
          --health-retries 30
    env:
      DATABASE_URL: postgres://geo:geo@localhost:5432/geo?sslmode=disable
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: backend/go.mod
          cache-dependency-path: backend/go.sum
      - name: gofmt
        run: |
          unformatted="$(gofmt -l .)"
          if [ -n "$unformatted" ]; then echo "$unformatted"; exit 1; fi
      - name: go vet
        run: go vet ./...
      - name: unit tests
        run: go test -race ./...
      - name: integration tests
        run: go test -race -tags=integration ./...

  frontend:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-node@v7
        with:
          node-version: 24
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - run: npm ci
      - run: npm run lint
      - run: npm run typecheck
      - run: npm test
      - run: npm run build

  docker:
    runs-on: ubuntu-latest
    env:
      GOPROXY: https://proxy.golang.org,direct
    steps:
      - uses: actions/checkout@v7
      - run: docker compose build
```

- [ ] **Step 2: 用 actionlint 检查工作流**

Run: `docker run --rm -v "$PWD":/repo -w /repo rhysd/actionlint:latest -color`
Expected: 没有输出，退出码为 0。

- [ ] **Step 3: 在本地按 CI 的步骤完整执行一遍**

```bash
(cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test -race ./...)
(cd frontend && npm ci && npm run lint && npm run typecheck && npm test && npm run build)
GOPROXY=https://goproxy.cn,direct docker compose build
```

Expected: 全部成功。集成测试步骤要在 PR 上的 CI 中验证：本阶段还没有带 `integration` 构建标签的测试，那一步实际执行的是单元测试。

- [ ] **Step 4: 更新 README**

把 `README.md` 第一行标题 `# beijing-geo-map` 替换为：

```markdown
# beijing-geo-map

[![CI](https://github.com/slatt-oo/beijing-geo-map/actions/workflows/ci.yml/badge.svg)](https://github.com/slatt-oo/beijing-geo-map/actions/workflows/ci.yml)
```

把状态行：

```markdown
> **当前状态：设计阶段。** 设计文档已经完成，代码按阶段推进中，进度见[实现文档](docs/01-implementation-plan.md#5-阶段计划)。
```

替换为：

```markdown
> **当前状态：开发中。** 项目骨架已经可以一键启动，地图功能按阶段推进中，进度见[实现文档](docs/01-implementation-plan.md#5-阶段计划)。
```

把技术栈表格中的两行：

```markdown
| 后端 | Go、Gin、pgx v5、golang-migrate |
| 前端 | React 18、TypeScript、Vite、MapLibre GL JS、TanStack Query、Zustand、Tailwind CSS |
```

替换为：

```markdown
| 后端 | Go 1.26、Gin、pgx v5、golang-migrate |
| 前端 | React 19、TypeScript 6、Vite 8、MapLibre GL JS、TanStack Query、Zustand、Tailwind CSS 4 |
```

在 `## 文档` 这一行之前插入：

````markdown
## 快速开始

只需要安装 Docker（包含 Docker Compose v2）：

```bash
git clone https://github.com/slatt-oo/beijing-geo-map.git
cd beijing-geo-map
docker compose up --build
```

启动后访问 <http://localhost:8080>。端口、数据库账号和构建镜像时的依赖源都可以在 `.env` 中修改，参见 `.env.example`。例如 8080 端口被占用时，可以用 `FRONTEND_PORT=8090 docker compose up --build`。

本地开发时不需要构建镜像：

```bash
make dev-db         # 只启动数据库
make run-backend    # 启动后端，监听 localhost:8000
make run-frontend   # 启动前端开发服务器，/api 会被代理到 localhost:8000
make test           # 运行前后端单元测试
```

````

- [ ] **Step 5: 更新实现文档中的版本信息**

在 `docs/01-implementation-plan.md` 的技术选型表中，把：

```markdown
| 后端 | Go + Gin + pgx v5 + golang-migrate | 性能好，部署是单个二进制；直接用 pgx 手写 SQL，空间查询完全可控 |
| 前端 | React 18 + TypeScript + Vite | 主流组合，类型安全，开发体验好 |
```

替换为：

```markdown
| 后端 | Go 1.26 + Gin + pgx v5 + golang-migrate | 性能好，部署是单个二进制；直接用 pgx 手写 SQL，空间查询完全可控 |
| 前端 | React 19 + TypeScript 6 + Vite 8 | 主流组合，类型安全，开发体验好。TypeScript 暂时固定在 6.0，因为 typescript-eslint 8 只支持到 6.0 |
```

- [ ] **Step 6: 更新技术方案中的版本、构建参数和端口说明**

在 `docs/02-technical-design.md` 的环境变量表中，在 `VITE_TILE_CRS` 这一行之后插入三行：

```markdown
| `DB_PORT` / `FRONTEND_PORT` | compose | `5432` / `8080` | 映射到宿主机的端口 |
| `GOPROXY` | backend 镜像构建 | `https://goproxy.cn,direct` | Go 模块代理；CI 中设为 `https://proxy.golang.org,direct` |
| `NPM_REGISTRY` | frontend 镜像构建 | `https://registry.npmjs.org/` | npm 源；国内可以改为 `https://registry.npmmirror.com/` |
```

把 3.4 节整节：

```markdown
### 3.4 Go 版本

本机安装的是 Go 1.21。`go.mod` 会声明项目实际需要的更高版本，Go 1.21 及以上会根据 `go.mod` 自动下载对应的工具链（`GOTOOLCHAIN=auto`），所以不需要手动升级。Docker 构建和 CI 直接使用对应版本的官方镜像。
```

替换为：

```markdown
### 3.4 版本与依赖源

- **Go 1.26**：`go.mod` 声明 `go 1.26.0`。Gin v1.12 和 pgx v5.11 要求 Go 1.25 以上，gin-contrib/gzip v1.2.8 要求 Go 1.26 以上。本机的 Go 1.21 会根据 `go.mod` 自动下载 1.26 工具链（`GOTOOLCHAIN=auto`），不需要手动升级。Docker 构建使用 `golang:1.26-alpine`，CI 中的 `setup-go` 从 `go.mod` 读取版本。
- **前端**：React 19、Vite 8、Vitest 5、ESLint 10、Tailwind CSS 4，Node 24。TypeScript 固定为 `~6.0.3`：typescript-eslint 8 只支持到 TypeScript 6.0，暂时不能升级到 7.0。
- **npm 源**：`frontend/.npmrc` 固定使用 `https://registry.npmjs.org/`，保证 `package-lock.json` 中只有公共源地址，GitHub CI 和其他人 clone 后都能正常安装。
- **镜像构建时的依赖源**：通过构建参数 `GOPROXY` 和 `NPM_REGISTRY` 配置，默认值见上方环境变量表。
- **PostGIS 镜像**：`postgis/postgis:16-3.4` 只提供 amd64 版本。compose 中显式声明 `platform: linux/amd64`，在 Apple Silicon 上通过模拟运行，功能不受影响，只是启动稍慢。
- **数据库健康检查**：`pg_isready` 必须加 `-h 127.0.0.1` 走 TCP。官方镜像初始化阶段的临时实例只监听 unix socket，不加这个参数会在初始化完成之前就被判定为健康。
```

- [ ] **Step 7: 提交**

```bash
git add .github/workflows/ci.yml README.md docs/01-implementation-plan.md docs/02-technical-design.md
git commit -m "ci: add GitHub Actions workflow; docs: record pinned versions and quick start"
```

---

## 阶段完成后（由协调者执行，不属于任何任务）

1. 推送分支：`git push -u origin phase-0-skeleton`。
2. 创建 PR：`gh pr create --base main --head phase-0-skeleton`，标题为 `阶段 0：项目骨架、Compose 一键启动与 CI`，正文写明本阶段的验收标准和验证证据。
3. 等待 CI：`gh pr checks --watch`，确认 `backend`、`frontend`、`docker` 三个任务全部通过。
4. 由用户审查并合并 PR。用户合并之前，不开始阶段 1。
