# 阶段 1：数据层 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成数据层：数据库迁移、坐标转换、行政区与 POI 数据快照、幂等导入器（`seed` 服务）、repository 空间查询层及其集成测试。`docker compose up --build` 启动后数据库里有 16 个区和约 4100 个 POI，`backend` 在导入成功后才启动。

**Architecture:** 迁移 SQL 通过 `embed.FS` 打包，由 golang-migrate（pgx v5 驱动）执行。`internal/geo` 实现 WGS-84 与 GCJ-02 互转，前后端共用仓库根目录的测试样例。`internal/importer` 负责解析 DataV 边界（导入时转成 WGS-84）和 Overpass 快照（分类、取名、地铁站去重），然后在一个事务里完成 upsert、同步删除和 `district_id` 重算。`internal/repository` 用 pgx 手写 SQL 实现数据库设计文档第 6 节的全部查询。集成测试通过 `internal/testdb` 为每个测试包创建独立的 `geo_test_<name>` 数据库，不会碰到开发库。

**Tech Stack:** Go 1.26、pgx v5.11.0、golang-migrate v4.20.1、PostgreSQL 16 + PostGIS 3.4、pg_trgm；Docker Compose；GitHub Actions。

**Spec:** `docs/01-implementation-plan.md`（第 5 节「阶段 1」）、`docs/02-technical-design.md`（第 3、4、5.5、7 节）、`docs/03-database-design.md`（全文）

## Global Constraints

- 所有工作在 `phase-1-data-layer` 分支上进行，每个任务结束时提交一次。仓库的 git 提交身份已经配置为 `slatt-oo <177398036+slatt-oo@users.noreply.github.com>`，不要修改。环境会在提交信息末尾自动加一行 `Co-authored-by: Cursor`，属于正常现象。
- Go 模块路径：`github.com/slatt-oo/beijing-geo-map/backend`，`go.mod` 声明 `go 1.26.0`。本机 Go 是 1.21.0，`GOTOOLCHAIN=auto` 会自动切换到 1.26 工具链。
- Go 依赖版本固定为：`github.com/gin-gonic/gin@v1.12.0`、`github.com/gin-contrib/gzip@v1.2.8`、`github.com/jackc/pgx/v5@v5.11.0`、`github.com/golang-migrate/migrate/v4@v4.20.1`（已实测这些版本可以一起编译，不会把 pgx 升级）。后端测试只用标准库 `testing`，不引入 testify。
- 坐标约定：数据库和 repository 返回的坐标一律是 WGS-84 的 `[经度, 纬度]`，SRID 4326。只有 DataV 边界是 GCJ-02，在导入时转换。
- 数据库对象的名称以 `docs/03-database-design.md` 第 3 节为准（表 `district`、`poi_category`、`poi`；索引 `district_geom_gist`、`poi_geom_gist`、`poi_geog_gist`、`poi_name_trgm`、`poi_category_id_idx`、`poi_district_id_idx`）。
- 数据文件固定为 `data/raw/beijing_districts_gcj02.json`（DataV）和 `data/raw/beijing_pois_osm.json`（Overpass 原样输出）。
- 导入规则（技术方案 5.5 节）：分类按 地铁站 → 高校 → 医院 → 景点/博物馆 → 公园 的顺序匹配，命中即停；名称优先 `name:zh`，其次 `name`，都为空则丢弃；同名且相距 300 米以内的地铁站只保留一条，节点优先，其次 `osm_id` 较小者；`cmd/importer` 要求快照中至少有 1000 个 POI，否则拒绝导入。
- 集成测试：文件名为 `*_integration_test.go`，首行 `//go:build integration`；通过 `testdb.New(t, "<包名>")` 或 `testdb.URL(t, "<包名>")` 获取数据库。管理连接来自环境变量 `TEST_DATABASE_URL`，本地取 `postgres://geo:geo@localhost:5432/geo?sslmode=disable`。测试只能使用 `geo_test_*` 数据库，不能写开发库 `geo`。
- 本地数据库：用阶段 0 的 compose 启动，命令为 `docker compose up -d --wait db`，端口只绑定 `127.0.0.1:5432`。`postgis/postgis:16-3.4` 只有 amd64 版本，在本机通过模拟运行，首次启动需要几十秒。
- 本机 Docker 是 Colima，没有 buildx 插件，`docker compose build` 会自动退回经典构建器，这是正常的。本机 8080 端口被其他进程占用，端到端验证时使用 `FRONTEND_PORT=18080`。
- 代码注释只用来说明代码本身体现不出来的约束，不写「这一行做什么」。
- 计划中的 Go 代码如果和 gofmt 的格式（例如对齐用的空格）有出入，以 `gofmt -w` 的结果为准；提交前 `gofmt -l` 必须没有输出。
- **沙箱说明：** 凡是需要访问网络（Go 模块下载、curl）、操作 Docker、或者连接本机数据库的命令，都要以 Shell `required_permissions: ["all"]` 运行。沙箱里写入 `/tmp` 的文件，沙箱外的 Docker 看不到，临时文件请放在仓库内被 git 忽略的 `.superpowers/` 目录下。

---

### Task 1: 配置补充与健康检查日志降级（阶段 0 遗留）

**Files:**
- Modify: `backend/internal/config/config.go`（整体替换）
- Modify: `backend/internal/config/config_test.go`（追加测试）
- Modify: `backend/internal/handler/middleware.go`（`requestLogger`）
- Modify: `backend/internal/handler/router.go`（健康检查路由）
- Modify: `backend/internal/handler/router_test.go`（追加测试）

**Interfaces:**
- Consumes: 现有 `config.load(getenv)`、`handler.NewRouter`、测试辅助函数 `get`、`fakePinger`
- Produces:
  - `config.Config` 新增字段 `DataDir string`，来自 `DATA_DIR`，默认 `/data`；`DATABASE_URL`、`HTTP_ADDR`、`DATA_DIR` 都会去掉首尾空白，只有空白的 `DATABASE_URL` 视为未设置
  - `handler` 包常量 `healthPath = "/api/v1/health"`；成功（状态码小于 400）的健康检查请求日志级别为 Debug，其他请求仍为 Info

- [ ] **Step 1: 编写失败的配置测试**

在 `backend/internal/config/config_test.go` 末尾追加：

```go
func TestLoadDefaultsDataDir(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DataDir != "/data" {
		t.Errorf("DataDir = %q, want /data", cfg.DataDir)
	}
}

func TestLoadTrimsWhitespace(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{
		"DATABASE_URL": "  postgres://x  ",
		"HTTP_ADDR":    " :9000 ",
		"DATA_DIR":     " ../data/raw ",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://x" || cfg.HTTPAddr != ":9000" || cfg.DataDir != "../data/raw" {
		t.Errorf("cfg = %+v, want trimmed values", cfg)
	}
}

func TestLoadRejectsBlankDatabaseURL(t *testing.T) {
	_, err := load(envFrom(map[string]string{"DATABASE_URL": "   "}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v, want an error mentioning DATABASE_URL", err)
	}
}
```

- [ ] **Step 2: 运行配置测试，确认失败**

Run: `cd backend && go test ./internal/config/`
Expected: 编译失败，报错 `cfg.DataDir undefined`。

- [ ] **Step 3: 替换 config.go**

把 `backend/internal/config/config.go` 整体替换为：

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
	DataDir     string
	LogLevel    slog.Level
}

func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL: strings.TrimSpace(getenv("DATABASE_URL")),
		HTTPAddr:    strings.TrimSpace(getenv("HTTP_ADDR")),
		DataDir:     strings.TrimSpace(getenv("DATA_DIR")),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL 未设置")
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8000"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
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

- [ ] **Step 4: 运行配置测试，确认通过**

Run: `cd backend && go test ./internal/config/ -v`
Expected: 7 个测试全部 PASS。

- [ ] **Step 5: 编写失败的日志测试**

在 `backend/internal/handler/router_test.go` 的 import 中加入 `"bytes"`，并在文件末尾追加：

```go
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestSuccessfulHealthChecksAreNotLoggedAtInfo(t *testing.T) {
	logs := captureLogs(t)
	r := NewRouter(Deps{DB: fakePinger{}})

	get(t, r, "/api/v1/health", nil)
	get(t, r, "/api/v1/does-not-exist", nil)

	if strings.Contains(logs.String(), `"path":"/api/v1/health"`) {
		t.Fatalf("successful health check was logged at info level:\n%s", logs)
	}
	if !strings.Contains(logs.String(), `"path":"/api/v1/does-not-exist"`) {
		t.Fatalf("other requests must still be logged at info level:\n%s", logs)
	}
}

func TestFailedHealthChecksAreLoggedAtInfo(t *testing.T) {
	logs := captureLogs(t)
	r := NewRouter(Deps{DB: fakePinger{err: errors.New("connection refused")}})

	get(t, r, "/api/v1/health", nil)

	if !strings.Contains(logs.String(), `"path":"/api/v1/health"`) || !strings.Contains(logs.String(), `"status":503`) {
		t.Fatalf("failed health check must be logged at info level with status 503:\n%s", logs)
	}
}
```

- [ ] **Step 6: 运行日志测试，确认失败**

Run: `cd backend && go test ./internal/handler/ -run 'HealthChecks' -v`
Expected: `TestSuccessfulHealthChecksAreNotLoggedAtInfo` FAIL（成功的健康检查目前按 Info 记录），`TestFailedHealthChecksAreLoggedAtInfo` PASS。

- [ ] **Step 7: 修改路由和日志中间件**

在 `backend/internal/handler/router.go` 中：在 `const (` 块（包含 `requestTimeoutDuration` 和 `gzipMinLength`）里加一行 `healthPath = "/api/v1/health"`；把 `NewRouter` 末尾的这两行：

```go
	api := r.Group("/api/v1")
	api.GET("/health", healthHandler(deps.DB))
```

替换为：

```go
	r.GET(healthPath, healthHandler(deps.DB))
```

在 `backend/internal/handler/middleware.go` 中，import 加入 `"net/http"`，并把 `requestLogger` 整个函数替换为：

```go
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		level := slog.LevelInfo
		// compose 每 5 秒探测一次健康检查，成功的探测记为 Debug，避免淹没访问日志。
		if c.Request.URL.Path == healthPath && c.Writer.Status() < http.StatusBadRequest {
			level = slog.LevelDebug
		}
		slog.Log(c.Request.Context(), level, "request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query", c.Request.URL.RawQuery,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
		)
	}
}
```

- [ ] **Step 8: 运行全部后端检查，确认通过**

Run: `cd backend && go test ./... && go vet ./... && test -z "$("$(go env GOROOT)/bin/gofmt" -l .)"`
Expected: `config` 包 7 个测试、`handler` 包 10 个测试全部 PASS，`go vet` 和 `gofmt` 都没有输出。

- [ ] **Step 9: 提交**

```bash
git add backend/internal/config backend/internal/handler
git commit -m "feat(backend): add DATA_DIR config, trim env values, log successful health checks at debug"
```

---

### Task 2: 坐标转换（WGS-84 与 GCJ-02）与共享测试样例

**Files:**
- Create: `testdata/coord_cases.json`（仓库根目录）
- Create: `backend/internal/geo/gcj02.go`
- Test: `backend/internal/geo/gcj02_test.go`

**Interfaces:**
- Consumes: 无
- Produces:
  - `func geo.WGS84ToGCJ02(lng, lat float64) (float64, float64)`
  - `func geo.GCJ02ToWGS84(lng, lat float64) (float64, float64)`：迭代反算，收敛阈值 `1e-7` 度，最多 10 轮
  - `func geo.OutOfChina(lng, lat float64) bool`：境外点两个转换函数都原样返回
  - `testdata/coord_cases.json`：阶段 3 的前端 `coord.ts` 也会读取这份样例，格式为 `{"source": string, "cases": [{"name", "wgs84": [lng, lat], "gcj02": [lng, lat]}]}`

- [ ] **Step 1: 创建共享测试样例**

创建 `testdata/coord_cases.json`（数值已由 gcoord 1.0.7 生成，原样写入）：

```json
{
  "source": "gcoord@1.0.7 transform(WGS84 -> GCJ02)，生成于 2026-10-13；gcj02 保留 9 位小数",
  "cases": [
    { "name": "天安门", "wgs84": [116.391275, 39.906217], "gcj02": [116.397516005, 39.907618208] },
    { "name": "鸟巢", "wgs84": [116.390297, 39.991853], "gcj02": [116.396548401, 39.993257194] },
    { "name": "北京大学", "wgs84": [116.304826, 39.986913], "gcj02": [116.310919103, 39.988184685] },
    { "name": "首都机场", "wgs84": [116.587115, 40.07715], "gcj02": [116.592921528, 40.078198902] },
    { "name": "东京（境外）", "wgs84": [139.767125, 35.681236], "gcj02": [139.767125, 35.681236] }
  ]
}
```

- [ ] **Step 2: 编写失败的测试**

创建 `backend/internal/geo/gcj02_test.go`：

```go
package geo

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

const tolerance = 1e-6

type coordCase struct {
	Name  string     `json:"name"`
	WGS84 [2]float64 `json:"wgs84"`
	GCJ02 [2]float64 `json:"gcj02"`
}

func loadCases(t *testing.T) []coordCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "coord_cases.json"))
	if err != nil {
		t.Fatalf("read shared fixture: %v", err)
	}
	var fixture struct {
		Cases []coordCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode shared fixture: %v", err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("shared fixture has no cases")
	}
	return fixture.Cases
}

func near(a, b [2]float64) bool {
	return math.Abs(a[0]-b[0]) <= tolerance && math.Abs(a[1]-b[1]) <= tolerance
}

func TestWGS84ToGCJ02MatchesReference(t *testing.T) {
	for _, c := range loadCases(t) {
		lng, lat := WGS84ToGCJ02(c.WGS84[0], c.WGS84[1])
		if got := [2]float64{lng, lat}; !near(got, c.GCJ02) {
			t.Errorf("%s: WGS84ToGCJ02 = %.9f, want %.9f", c.Name, got, c.GCJ02)
		}
	}
}

func TestGCJ02ToWGS84InvertsForwardTransform(t *testing.T) {
	for _, c := range loadCases(t) {
		wLng, wLat := GCJ02ToWGS84(c.GCJ02[0], c.GCJ02[1])
		gLng, gLat := WGS84ToGCJ02(wLng, wLat)
		if got := [2]float64{gLng, gLat}; !near(got, c.GCJ02) {
			t.Errorf("%s: forward(inverse(g)) = %.9f, want %.9f", c.Name, got, c.GCJ02)
		}
		if got := [2]float64{wLng, wLat}; !near(got, c.WGS84) {
			t.Errorf("%s: GCJ02ToWGS84 = %.9f, want %.9f", c.Name, got, c.WGS84)
		}
	}
}

func TestOutOfChinaPointsAreUnchanged(t *testing.T) {
	const lng, lat = 139.767125, 35.681236
	if !OutOfChina(lng, lat) {
		t.Fatal("Tokyo should be out of China")
	}
	if OutOfChina(116.391275, 39.906217) {
		t.Fatal("Tiananmen should be inside China")
	}
	if gLng, gLat := WGS84ToGCJ02(lng, lat); gLng != lng || gLat != lat {
		t.Errorf("WGS84ToGCJ02 changed an out-of-China point: (%v, %v)", gLng, gLat)
	}
	if wLng, wLat := GCJ02ToWGS84(lng, lat); wLng != lng || wLat != lat {
		t.Errorf("GCJ02ToWGS84 changed an out-of-China point: (%v, %v)", wLng, wLat)
	}
}
```

- [ ] **Step 3: 运行测试，确认失败**

Run: `cd backend && go test ./internal/geo/`
Expected: 编译失败，报错 `undefined: WGS84ToGCJ02` 等。

- [ ] **Step 4: 编写实现**

创建 `backend/internal/geo/gcj02.go`：

```go
// Package geo 实现 WGS-84 与 GCJ-02 坐标的互相转换（公开的标准算法，克拉索夫斯基椭球参数）。
package geo

import "math"

const (
	krasovskyA  = 6378245.0
	krasovskyEE = 0.00669342162296594323

	inverseTolerance     = 1e-7
	maxInverseIterations = 10
)

func OutOfChina(lng, lat float64) bool {
	return lng < 72.004 || lng > 137.8347 || lat < 0.8293 || lat > 55.8271
}

func WGS84ToGCJ02(lng, lat float64) (float64, float64) {
	if OutOfChina(lng, lat) {
		return lng, lat
	}
	dLng, dLat := offset(lng, lat)
	return lng + dLng, lat + dLat
}

// GCJ02ToWGS84 没有解析解，用迭代法反算：每轮按正向转换的残差修正估计值。
func GCJ02ToWGS84(lng, lat float64) (float64, float64) {
	if OutOfChina(lng, lat) {
		return lng, lat
	}
	wLng, wLat := lng, lat
	for range maxInverseIterations {
		gLng, gLat := WGS84ToGCJ02(wLng, wLat)
		dLng, dLat := gLng-lng, gLat-lat
		wLng, wLat = wLng-dLng, wLat-dLat
		if math.Abs(dLng) < inverseTolerance && math.Abs(dLat) < inverseTolerance {
			break
		}
	}
	return wLng, wLat
}

func offset(lng, lat float64) (float64, float64) {
	x, y := lng-105.0, lat-35.0
	dLat := transformLat(x, y)
	dLng := transformLng(x, y)
	radLat := lat / 180.0 * math.Pi
	magic := 1 - krasovskyEE*math.Sin(radLat)*math.Sin(radLat)
	sqrtMagic := math.Sqrt(magic)
	dLat = (dLat * 180.0) / ((krasovskyA * (1 - krasovskyEE)) / (magic * sqrtMagic) * math.Pi)
	dLng = (dLng * 180.0) / (krasovskyA / sqrtMagic * math.Cos(radLat) * math.Pi)
	return dLng, dLat
}

func transformLat(x, y float64) float64 {
	r := -100.0 + 2.0*x + 3.0*y + 0.2*y*y + 0.1*x*y + 0.2*math.Sqrt(math.Abs(x))
	r += (20.0*math.Sin(6.0*x*math.Pi) + 20.0*math.Sin(2.0*x*math.Pi)) * 2.0 / 3.0
	r += (20.0*math.Sin(y*math.Pi) + 40.0*math.Sin(y/3.0*math.Pi)) * 2.0 / 3.0
	r += (160.0*math.Sin(y/12.0*math.Pi) + 320.0*math.Sin(y*math.Pi/30.0)) * 2.0 / 3.0
	return r
}

func transformLng(x, y float64) float64 {
	r := 300.0 + x + 2.0*y + 0.1*x*x + 0.1*x*y + 0.1*math.Sqrt(math.Abs(x))
	r += (20.0*math.Sin(6.0*x*math.Pi) + 20.0*math.Sin(2.0*x*math.Pi)) * 2.0 / 3.0
	r += (20.0*math.Sin(x*math.Pi) + 40.0*math.Sin(x/3.0*math.Pi)) * 2.0 / 3.0
	r += (150.0*math.Sin(x/12.0*math.Pi) + 300.0*math.Sin(x/30.0*math.Pi)) * 2.0 / 3.0
	return r
}
```

- [ ] **Step 5: 运行测试，确认通过**

Run: `cd backend && go test ./internal/geo/ -v && go vet ./... && test -z "$("$(go env GOROOT)/bin/gofmt" -l .)"`
Expected: 3 个测试全部 PASS，`go vet` 和 `gofmt` 都没有输出。

- [ ] **Step 6: 提交**

```bash
git add testdata/coord_cases.json backend/internal/geo
git commit -m "feat(backend): add WGS-84/GCJ-02 coordinate conversion with shared fixture"
```

---

### Task 3: 数据库迁移、集成测试数据库与 CI 调整

**Files:**
- Modify: `backend/go.mod`、`backend/go.sum`（新增 golang-migrate）
- Create: `backend/migrations/000001_init.up.sql`、`backend/migrations/000001_init.down.sql`
- Create: `backend/migrations/000002_seed_poi_categories.up.sql`、`backend/migrations/000002_seed_poi_categories.down.sql`
- Create: `backend/migrations/migrations.go`
- Test: `backend/migrations/migrations_test.go`、`backend/migrations/migrations_integration_test.go`
- Create: `backend/internal/testdb/testdb.go`
- Modify: `.github/workflows/ci.yml`（backend 任务的环境变量和 vet 步骤）
- Modify: `Makefile`（`test-integration` 一行）

**Interfaces:**
- Consumes: 无
- Produces:
  - `var migrations.FS embed.FS`：全部 `*.sql` 迁移文件
  - `func migrations.Up(databaseURL string) error`：接受 `postgres://` 或 `postgresql://` 连接串，升级到最新版本；已是最新时返回 nil
  - `func testdb.URL(t testing.TB, name string) string`：确保 `geo_test_<name>` 存在并返回连接串，不执行迁移
  - `func testdb.New(t testing.TB, name string) *pgxpool.Pool`：执行迁移、清空 `poi` 和 `district`（保留分类种子数据）后返回连接池，测试结束时自动关闭
  - CI 和 Makefile 用环境变量 `TEST_DATABASE_URL` 运行集成测试

- [ ] **Step 1: 添加依赖并启动本地数据库**

```bash
cd backend && go get github.com/golang-migrate/migrate/v4@v4.20.1 && cd ..
docker compose up -d --wait db
```

Expected: `go.mod` 的 require 中出现 `github.com/golang-migrate/migrate/v4 v4.20.1`，并且 `github.com/jackc/pgx/v5` 仍是 `v5.11.0`；`db` 容器状态为 healthy。

- [ ] **Step 2: 编写迁移 SQL**

创建 `backend/migrations/000001_init.up.sql`：

```sql
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE district (
    id          serial PRIMARY KEY,
    adcode      char(6)        NOT NULL,
    name        varchar(32)    NOT NULL,
    center      geometry(Point, 4326)        NOT NULL,
    geom        geometry(MultiPolygon, 4326) NOT NULL,
    area_km2    numeric(10, 2) NOT NULL CHECK (area_km2 > 0),
    created_at  timestamptz    NOT NULL DEFAULT now(),
    updated_at  timestamptz    NOT NULL DEFAULT now(),
    CONSTRAINT district_adcode_uk   UNIQUE (adcode),
    CONSTRAINT district_adcode_fmt  CHECK (adcode ~ '^[0-9]{6}$'),
    CONSTRAINT district_geom_valid  CHECK (ST_IsValid(geom))
);

CREATE INDEX district_geom_gist ON district USING gist (geom);

CREATE TABLE poi_category (
    id          smallserial PRIMARY KEY,
    code        varchar(32) NOT NULL,
    name        varchar(32) NOT NULL,
    color       char(7)     NOT NULL,
    sort_order  smallint    NOT NULL DEFAULT 0,
    CONSTRAINT poi_category_code_uk    UNIQUE (code),
    CONSTRAINT poi_category_color_fmt  CHECK (color ~ '^#[0-9A-Fa-f]{6}$')
);

CREATE TABLE poi (
    id           bigserial    PRIMARY KEY,
    osm_type     char(1)      NOT NULL,
    osm_id       bigint       NOT NULL,
    name         varchar(128) NOT NULL,
    name_en      varchar(128),
    category_id  smallint     NOT NULL REFERENCES poi_category (id),
    district_id  integer      REFERENCES district (id) ON DELETE SET NULL,
    geom         geometry(Point, 4326) NOT NULL,
    address      text,
    tags         jsonb        NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT poi_osm_uk         UNIQUE (osm_type, osm_id),
    CONSTRAINT poi_osm_type_chk   CHECK (osm_type IN ('n', 'w', 'r')),
    CONSTRAINT poi_name_not_blank CHECK (btrim(name) <> '')
);

CREATE INDEX poi_geom_gist        ON poi USING gist (geom);
CREATE INDEX poi_geog_gist        ON poi USING gist ((geom::geography));
CREATE INDEX poi_name_trgm        ON poi USING gin (name gin_trgm_ops);
CREATE INDEX poi_category_id_idx  ON poi (category_id);
CREATE INDEX poi_district_id_idx  ON poi (district_id);
```

创建 `backend/migrations/000001_init.down.sql`：

```sql
DROP TABLE IF EXISTS poi;
DROP TABLE IF EXISTS poi_category;
DROP TABLE IF EXISTS district;
```

创建 `backend/migrations/000002_seed_poi_categories.up.sql`：

```sql
INSERT INTO poi_category (code, name, color, sort_order) VALUES
    ('subway_station', '地铁站', '#E53935', 1),
    ('university', '高校', '#1E88E5', 2),
    ('attraction', '景点/博物馆', '#FB8C00', 3),
    ('hospital', '医院', '#8E24AA', 4),
    ('park', '公园', '#43A047', 5);
```

创建 `backend/migrations/000002_seed_poi_categories.down.sql`：

```sql
DELETE FROM poi_category
WHERE code IN ('subway_station', 'university', 'attraction', 'hospital', 'park');
```

- [ ] **Step 3: 编写失败的单元测试**

创建 `backend/migrations/migrations_test.go`：

```go
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
```

Run: `cd backend && go test ./migrations/`
Expected: 编译失败，报错 `undefined: migrations.Up`（或该包中没有非测试 Go 文件）。

- [ ] **Step 4: 编写 migrations.go**

创建 `backend/migrations/migrations.go`：

```go
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
```

Run: `cd backend && go mod tidy && go test ./migrations/ -v`
Expected: `TestUpRejectsNonPostgresURL` PASS。

- [ ] **Step 5: 编写集成测试数据库辅助包**

创建 `backend/internal/testdb/testdb.go`：

```go
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
	return u.String()
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
```

- [ ] **Step 6: 编写迁移的集成测试**

创建 `backend/migrations/migrations_integration_test.go`：

```go
//go:build integration

package migrations_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/testdb"
	"github.com/slatt-oo/beijing-geo-map/backend/migrations"
)

func connect(t *testing.T, dbURL string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

func freshDatabase(t *testing.T) string {
	t.Helper()
	dbURL := testdb.URL(t, "migrations")
	conn := connect(t, dbURL)
	if _, err := conn.Exec(context.Background(), `DROP TABLE IF EXISTS poi, poi_category, district, schema_migrations CASCADE`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	return dbURL
}

func queryStrings(t *testing.T, conn *pgx.Conn, sql string) []string {
	t.Helper()
	rows, err := conn.Query(context.Background(), sql)
	if err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	values, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect %q: %v", sql, err)
	}
	return values
}

const appTablesSQL = `SELECT tablename FROM pg_tables
WHERE schemaname = 'public' AND tablename IN ('district', 'poi_category', 'poi')
ORDER BY tablename`

func TestUpCreatesSchemaAndIsIdempotent(t *testing.T) {
	dbURL := freshDatabase(t)

	if err := migrations.Up(dbURL); err != nil {
		t.Fatalf("first Up: %v", err)
	}
	if err := migrations.Up(dbURL); err != nil {
		t.Fatalf("second Up must be a no-op: %v", err)
	}

	conn := connect(t, dbURL)
	if got := queryStrings(t, conn, appTablesSQL); !slices.Equal(got, []string{"district", "poi", "poi_category"}) {
		t.Fatalf("tables = %v", got)
	}
	indexes := queryStrings(t, conn, `SELECT indexname FROM pg_indexes WHERE schemaname = 'public'`)
	for _, want := range []string{
		"district_geom_gist", "poi_geom_gist", "poi_geog_gist", "poi_name_trgm",
		"poi_category_id_idx", "poi_district_id_idx",
		"district_adcode_uk", "poi_osm_uk", "poi_category_code_uk",
	} {
		if !slices.Contains(indexes, want) {
			t.Errorf("missing index %s", want)
		}
	}
	codes := queryStrings(t, conn, `SELECT code FROM poi_category ORDER BY sort_order`)
	if want := []string{"subway_station", "university", "attraction", "hospital", "park"}; !slices.Equal(codes, want) {
		t.Fatalf("category codes = %v, want %v", codes, want)
	}
}

func TestDownRemovesSchemaAndUpRestoresIt(t *testing.T) {
	dbURL := freshDatabase(t)
	if err := migrations.Up(dbURL); err != nil {
		t.Fatalf("Up: %v", err)
	}

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("iofs: %v", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, strings.Replace(dbURL, "postgres://", "pgx5://", 1))
	if err != nil {
		t.Fatalf("init migrate: %v", err)
	}
	if err := m.Down(); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
		t.Fatalf("close migrate: %v, %v", srcErr, dbErr)
	}

	conn := connect(t, dbURL)
	if got := queryStrings(t, conn, appTablesSQL); len(got) != 0 {
		t.Fatalf("tables left after Down: %v", got)
	}

	if err := migrations.Up(dbURL); err != nil {
		t.Fatalf("Up after Down: %v", err)
	}
	var categories int
	if err := conn.QueryRow(context.Background(), `SELECT count(*) FROM poi_category`).Scan(&categories); err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if categories != 5 {
		t.Fatalf("categories after Up = %d, want 5", categories)
	}
}
```

- [ ] **Step 7: 运行集成测试，确认通过**

Run:

```bash
cd backend && TEST_DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable' \
  go test -count=1 -tags=integration ./migrations/ -v
```

Expected: `TestUpRejectsNonPostgresURL`、`TestUpCreatesSchemaAndIsIdempotent`、`TestDownRemovesSchemaAndUpRestoresIt` 全部 PASS。开发库 `geo` 里没有出现任何业务表（用 `docker compose exec db psql -U geo -d geo -c '\dt'` 确认输出为 `Did not find any relations.`）。

- [ ] **Step 8: 调整 CI 和 Makefile**

在 `.github/workflows/ci.yml` 的 backend 任务中，把环境变量这一行：

```yaml
      DATABASE_URL: postgres://geo:geo@localhost:5432/geo?sslmode=disable
```

替换为：

```yaml
      TEST_DATABASE_URL: postgres://geo:geo@localhost:5432/geo?sslmode=disable
```

并把 `go vet` 步骤的 `run: go vet ./...` 替换为 `run: go vet -tags=integration ./...`。

在 `Makefile` 中，把 `test-integration` 目标的命令行：

```makefile
	cd backend && DATABASE_URL="$(DEV_DATABASE_URL)" go test -tags=integration ./...
```

替换为：

```makefile
	cd backend && TEST_DATABASE_URL="$(DEV_DATABASE_URL)" go test -count=1 -tags=integration ./...
```

- [ ] **Step 9: 运行全部后端检查，确认通过**

Run:

```bash
cd backend && go test ./... && go vet -tags=integration ./... && test -z "$("$(go env GOROOT)/bin/gofmt" -l .)"
cd .. && make test-integration
docker run --rm -v "$PWD":/repo -w /repo rhysd/actionlint:latest -color
```

Expected: 单元测试全部 PASS；`go vet` 和 `gofmt` 没有输出；`make test-integration` 中迁移的集成测试 PASS；actionlint 没有输出。

- [ ] **Step 10: 提交**

```bash
git add backend/go.mod backend/go.sum backend/migrations backend/internal/testdb .github/workflows/ci.yml Makefile
git commit -m "feat(backend): add embedded migrations, per-package test databases and CI integration env"
```

---

### Task 4: 导入器的解析逻辑（DataV 边界与 Overpass 快照）

**Files:**
- Create: `backend/internal/importer/datav.go`
- Create: `backend/internal/importer/osm.go`
- Test: `backend/internal/importer/datav_test.go`、`backend/internal/importer/osm_test.go`

**Interfaces:**
- Consumes: `geo.GCJ02ToWGS84`（Task 2）
- Produces:
  - `type importer.District struct { Adcode string; Name string; Geometry json.RawMessage }`：`Geometry` 是已转换为 WGS-84 的 GeoJSON MultiPolygon
  - `func importer.ParseDistricts(r io.Reader) ([]District, error)`
  - `type importer.POI struct { OSMType string; OSMID int64; Name string; NameEn *string; Category string; Lng, Lat float64; Address *string; Tags map[string]string }`：`OSMType` 取值为 `"n"`、`"w"`、`"r"`，`Category` 是分类编码
  - `func importer.ParsePOIs(r io.Reader) ([]POI, error)`：完成分类、取名、取坐标和地铁站去重

这个任务不需要数据库。

- [ ] **Step 1: 编写失败的 DataV 解析测试**

创建 `backend/internal/importer/datav_test.go`：

```go
package importer

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/geo"
)

const datavSample = `{"type":"FeatureCollection","features":[
  {"type":"Feature","properties":{"adcode":110101,"name":"东城区"},
   "geometry":{"type":"MultiPolygon","coordinates":[[[[116.40,39.90],[116.41,39.90],[116.41,39.91],[116.40,39.90]]]]}}
]}`

func TestParseDistrictsConvertsCoordinatesToWGS84(t *testing.T) {
	districts, err := ParseDistricts(strings.NewReader(datavSample))
	if err != nil {
		t.Fatalf("ParseDistricts: %v", err)
	}
	if len(districts) != 1 || districts[0].Adcode != "110101" || districts[0].Name != "东城区" {
		t.Fatalf("districts = %+v", districts)
	}
	var geometry struct {
		Type        string           `json:"type"`
		Coordinates [][][][2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(districts[0].Geometry, &geometry); err != nil {
		t.Fatalf("geometry is not valid JSON: %v", err)
	}
	if geometry.Type != "MultiPolygon" {
		t.Fatalf("geometry type = %q, want MultiPolygon", geometry.Type)
	}
	ring := geometry.Coordinates[0][0]
	wantLng, wantLat := geo.GCJ02ToWGS84(116.40, 39.90)
	if math.Abs(ring[0][0]-wantLng) > 1e-12 || math.Abs(ring[0][1]-wantLat) > 1e-12 {
		t.Fatalf("first vertex = %v, want (%v, %v)", ring[0], wantLng, wantLat)
	}
	if ring[0] != ring[len(ring)-1] {
		t.Fatalf("ring is no longer closed: first %v, last %v", ring[0], ring[len(ring)-1])
	}
}

func TestParseDistrictsRejectsUnsupportedGeometry(t *testing.T) {
	input := strings.Replace(datavSample, `"type":"MultiPolygon"`, `"type":"Polygon"`, 1)
	_, err := ParseDistricts(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "unsupported geometry type") {
		t.Fatalf("err = %v, want unsupported geometry type", err)
	}
}

func TestParseDistrictsRejectsInvalidAdcode(t *testing.T) {
	input := strings.Replace(datavSample, `"adcode":110101`, `"adcode":1101`, 1)
	_, err := ParseDistricts(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "adcode") {
		t.Fatalf("err = %v, want an invalid adcode error", err)
	}
}

func TestParseDistrictsRejectsEmptyCollection(t *testing.T) {
	_, err := ParseDistricts(strings.NewReader(`{"type":"FeatureCollection","features":[]}`))
	if err == nil {
		t.Fatal("expected an error for an empty collection")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd backend && go test ./internal/importer/`
Expected: 编译失败，报错 `undefined: ParseDistricts`。

- [ ] **Step 3: 编写 DataV 解析实现**

创建 `backend/internal/importer/datav.go`：

```go
package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/geo"
)

type District struct {
	Adcode   string
	Name     string
	Geometry json.RawMessage
}

type datavFeature struct {
	Properties struct {
		Adcode json.Number `json:"adcode"`
		Name   string      `json:"name"`
	} `json:"properties"`
	Geometry struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
}

type multiPolygon struct {
	Type        string           `json:"type"`
	Coordinates [][][][2]float64 `json:"coordinates"`
}

// ParseDistricts 解析 DataV.GeoAtlas 的 FeatureCollection。输入坐标是 GCJ-02，返回的几何已转换为 WGS-84。
func ParseDistricts(r io.Reader) ([]District, error) {
	var collection struct {
		Features []datavFeature `json:"features"`
	}
	if err := json.NewDecoder(r).Decode(&collection); err != nil {
		return nil, fmt.Errorf("decode districts: %w", err)
	}
	if len(collection.Features) == 0 {
		return nil, errors.New("districts snapshot has no features")
	}
	districts := make([]District, 0, len(collection.Features))
	for i, f := range collection.Features {
		adcode := f.Properties.Adcode.String()
		if len(adcode) != 6 {
			return nil, fmt.Errorf("feature %d: invalid adcode %q", i, adcode)
		}
		if f.Geometry.Type != "MultiPolygon" {
			return nil, fmt.Errorf("feature %d (%s): unsupported geometry type %q", i, adcode, f.Geometry.Type)
		}
		var polygons [][][][2]float64
		if err := json.Unmarshal(f.Geometry.Coordinates, &polygons); err != nil {
			return nil, fmt.Errorf("feature %d (%s): decode coordinates: %w", i, adcode, err)
		}
		toWGS84(polygons)
		geometry, err := json.Marshal(multiPolygon{Type: "MultiPolygon", Coordinates: polygons})
		if err != nil {
			return nil, fmt.Errorf("feature %d (%s): encode geometry: %w", i, adcode, err)
		}
		districts = append(districts, District{Adcode: adcode, Name: f.Properties.Name, Geometry: geometry})
	}
	return districts, nil
}

func toWGS84(polygons [][][][2]float64) {
	for _, polygon := range polygons {
		for _, ring := range polygon {
			for i, pt := range ring {
				ring[i][0], ring[i][1] = geo.GCJ02ToWGS84(pt[0], pt[1])
			}
		}
	}
}
```

Run: `cd backend && go test ./internal/importer/ -run ParseDistricts -v`
Expected: 4 个 `TestParseDistricts*` 测试 PASS。

- [ ] **Step 4: 编写失败的 Overpass 解析测试**

创建 `backend/internal/importer/osm_test.go`：

```go
package importer

import (
	"fmt"
	"strings"
	"testing"
)

func TestClassifyFollowsPriorityOrder(t *testing.T) {
	cases := []struct {
		name string
		tags map[string]string
		want string
		ok   bool
	}{
		{"subway via railway tag", map[string]string{"railway": "station", "station": "subway"}, "subway_station", true},
		{"subway via public_transport tag", map[string]string{"public_transport": "station", "subway": "yes"}, "subway_station", true},
		{"university", map[string]string{"amenity": "university"}, "university", true},
		{"hospital", map[string]string{"amenity": "hospital"}, "hospital", true},
		{"museum", map[string]string{"tourism": "museum"}, "attraction", true},
		{"park that is also an attraction", map[string]string{"leisure": "park", "tourism": "attraction"}, "attraction", true},
		{"park", map[string]string{"leisure": "park"}, "park", true},
		{"railway station that is not a subway", map[string]string{"railway": "station"}, "", false},
		{"unrelated", map[string]string{"shop": "supermarket"}, "", false},
	}
	for _, c := range cases {
		got, ok := classify(c.tags)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: classify = (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestPickNamePrefersChineseName(t *testing.T) {
	if got := pickName(map[string]string{"name": "Peking University", "name:zh": "北京大学"}); got != "北京大学" {
		t.Errorf("pickName = %q, want 北京大学", got)
	}
	if got := pickName(map[string]string{"name": " 颐和园 "}); got != "颐和园" {
		t.Errorf("pickName = %q, want 颐和园", got)
	}
	if got := pickName(map[string]string{"name:en": "Only English"}); got != "" {
		t.Errorf("pickName = %q, want empty", got)
	}
}

func TestComposeAddress(t *testing.T) {
	if got := composeAddress(map[string]string{"addr:full": "颐和园路5号", "addr:street": "忽略"}); got != "颐和园路5号" {
		t.Errorf("composeAddress = %q, want 颐和园路5号", got)
	}
	parts := map[string]string{"addr:city": "北京市", "addr:district": "海淀区", "addr:street": "颐和园路", "addr:housenumber": "5号"}
	if got := composeAddress(parts); got != "北京市海淀区颐和园路5号" {
		t.Errorf("composeAddress = %q, want 北京市海淀区颐和园路5号", got)
	}
	if got := composeAddress(map[string]string{}); got != "" {
		t.Errorf("composeAddress = %q, want empty", got)
	}
}

func TestParsePOIsBuildsPOIsFromOverpassElements(t *testing.T) {
	input := `{"elements":[
	  {"type":"node","id":1,"lat":39.99,"lon":116.31,"tags":{"amenity":"university","name":"北京大学","name:en":"Peking University","addr:full":"颐和园路5号"}},
	  {"type":"way","id":2,"center":{"lat":39.999,"lon":116.27},"tags":{"leisure":"park","tourism":"attraction","name":"颐和园"}},
	  {"type":"relation","id":3,"tags":{"leisure":"park","name":"没有中心点的公园"}},
	  {"type":"node","id":4,"lat":39.9,"lon":116.4,"tags":{"amenity":"hospital"}},
	  {"type":"node","id":5,"lat":39.9,"lon":116.4,"tags":{"shop":"supermarket","name":"超市"}}
	]}`

	pois, err := ParsePOIs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParsePOIs: %v", err)
	}
	if len(pois) != 2 {
		t.Fatalf("got %d POIs, want 2: %+v", len(pois), pois)
	}

	pku := pois[0]
	if pku.OSMType != "n" || pku.OSMID != 1 || pku.Name != "北京大学" || pku.Category != "university" ||
		pku.Lng != 116.31 || pku.Lat != 39.99 || pku.Tags["amenity"] != "university" {
		t.Errorf("first POI = %+v", pku)
	}
	if pku.NameEn == nil || *pku.NameEn != "Peking University" {
		t.Errorf("NameEn = %v, want Peking University", pku.NameEn)
	}
	if pku.Address == nil || *pku.Address != "颐和园路5号" {
		t.Errorf("Address = %v, want 颐和园路5号", pku.Address)
	}

	summerPalace := pois[1]
	if summerPalace.OSMType != "w" || summerPalace.OSMID != 2 || summerPalace.Category != "attraction" ||
		summerPalace.Lng != 116.27 || summerPalace.Lat != 39.999 {
		t.Errorf("second POI = %+v", summerPalace)
	}
	if summerPalace.NameEn != nil || summerPalace.Address != nil {
		t.Errorf("second POI should have no English name or address: %+v", summerPalace)
	}
}

func TestDedupeSubwayStationsKeepsOneStationPerNearbyName(t *testing.T) {
	pois := []POI{
		{OSMType: "w", OSMID: 10, Name: "西直门", Category: "subway_station", Lng: 116.3535, Lat: 39.9405},
		{OSMType: "n", OSMID: 30, Name: "西直门", Category: "subway_station", Lng: 116.3550, Lat: 39.9410},
		{OSMType: "n", OSMID: 20, Name: "西直门", Category: "subway_station", Lng: 116.3540, Lat: 39.9400},
		{OSMType: "n", OSMID: 40, Name: "草桥", Category: "subway_station", Lng: 116.3500, Lat: 39.8460},
		{OSMType: "n", OSMID: 50, Name: "草桥", Category: "subway_station", Lng: 116.3500, Lat: 39.8500},
		{OSMType: "n", OSMID: 60, Name: "动物园", Category: "attraction", Lng: 116.3360, Lat: 39.9390},
	}

	got := dedupeSubwayStations(pois)

	var keys []string
	for _, p := range got {
		keys = append(keys, fmt.Sprintf("%s%d", p.OSMType, p.OSMID))
	}
	want := []string{"n60", "n20", "n40", "n50"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("kept = %v, want %v", keys, want)
	}
}

func TestHaversineMeters(t *testing.T) {
	got := haversineMeters(116.05, 40.05, 116.05, 40.059)
	if got < 1000.7 || got > 1000.8 {
		t.Fatalf("haversineMeters = %v, want about 1000.76 (0.009° of latitude on the mean-radius sphere)", got)
	}
}
```

去重测试的预期说明：同名站按「节点优先、`osm_id` 较小优先」排序后逐个检查。n20 保留；n30 与 n20 相距约 140 米，丢弃；n40 保留；n50 与 n40 相距约 445 米，保留；w10 与 n20 相距约 70 米，丢弃。非地铁站 n60 保持在最前面。

Run: `cd backend && go test ./internal/importer/`
Expected: 编译失败，报错 `undefined: classify`、`undefined: ParsePOIs` 等。

- [ ] **Step 5: 编写 Overpass 解析实现**

创建 `backend/internal/importer/osm.go`：

```go
package importer

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
)

type POI struct {
	OSMType  string
	OSMID    int64
	Name     string
	NameEn   *string
	Category string
	Lng      float64
	Lat      float64
	Address  *string
	Tags     map[string]string
}

type overpassElement struct {
	Type   string   `json:"type"`
	ID     int64    `json:"id"`
	Lat    *float64 `json:"lat"`
	Lon    *float64 `json:"lon"`
	Center *struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"center"`
	Tags map[string]string `json:"tags"`
}

const subwayDedupeMeters = 300

var osmTypeCodes = map[string]string{"node": "n", "way": "w", "relation": "r"}

// ParsePOIs 解析 Overpass API 的 JSON 输出（out center tags）。无法分类、没有名称或没有坐标的要素会被丢弃。
func ParsePOIs(r io.Reader) ([]POI, error) {
	var resp struct {
		Elements []overpassElement `json:"elements"`
	}
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode overpass response: %w", err)
	}
	pois := make([]POI, 0, len(resp.Elements))
	for _, el := range resp.Elements {
		if poi, ok := toPOI(el); ok {
			pois = append(pois, poi)
		}
	}
	return dedupeSubwayStations(pois), nil
}

func toPOI(el overpassElement) (POI, bool) {
	osmType, ok := osmTypeCodes[el.Type]
	if !ok {
		return POI{}, false
	}
	category, ok := classify(el.Tags)
	if !ok {
		return POI{}, false
	}
	name := pickName(el.Tags)
	if name == "" {
		return POI{}, false
	}
	lng, lat, ok := location(el)
	if !ok {
		return POI{}, false
	}
	return POI{
		OSMType:  osmType,
		OSMID:    el.ID,
		Name:     name,
		NameEn:   optional(el.Tags["name:en"]),
		Category: category,
		Lng:      lng,
		Lat:      lat,
		Address:  optional(composeAddress(el.Tags)),
		Tags:     el.Tags,
	}, true
}

// classify 按优先级匹配，命中第一条即返回：同时带有景点和公园标签的要素（如颐和园）归为景点。
func classify(tags map[string]string) (string, bool) {
	switch {
	case tags["railway"] == "station" && tags["station"] == "subway",
		tags["public_transport"] == "station" && tags["subway"] == "yes":
		return "subway_station", true
	case tags["amenity"] == "university":
		return "university", true
	case tags["amenity"] == "hospital":
		return "hospital", true
	case tags["tourism"] == "attraction", tags["tourism"] == "museum":
		return "attraction", true
	case tags["leisure"] == "park":
		return "park", true
	default:
		return "", false
	}
}

func pickName(tags map[string]string) string {
	if name := strings.TrimSpace(tags["name:zh"]); name != "" {
		return name
	}
	return strings.TrimSpace(tags["name"])
}

var addressParts = []string{"addr:province", "addr:city", "addr:district", "addr:street", "addr:housenumber"}

func composeAddress(tags map[string]string) string {
	if full := strings.TrimSpace(tags["addr:full"]); full != "" {
		return full
	}
	var b strings.Builder
	for _, key := range addressParts {
		b.WriteString(strings.TrimSpace(tags[key]))
	}
	return b.String()
}

func location(el overpassElement) (float64, float64, bool) {
	if el.Lat != nil && el.Lon != nil {
		return *el.Lon, *el.Lat, true
	}
	if el.Center != nil {
		return el.Center.Lon, el.Center.Lat, true
	}
	return 0, 0, false
}

func optional(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

var osmTypeRank = map[string]int{"n": 0, "w": 1, "r": 2}

// dedupeSubwayStations 处理换乘站在 OSM 中每条线路各有一条记录的情况：同名且相距 300 米以内只保留一条，
// 节点优先，其次 osm_id 较小者。非地铁站保持原有顺序，保留下来的地铁站排在它们之后。
func dedupeSubwayStations(pois []POI) []POI {
	result := make([]POI, 0, len(pois))
	var stations []POI
	for _, p := range pois {
		if p.Category == "subway_station" {
			stations = append(stations, p)
		} else {
			result = append(result, p)
		}
	}
	slices.SortFunc(stations, func(a, b POI) int {
		if c := cmp.Compare(osmTypeRank[a.OSMType], osmTypeRank[b.OSMType]); c != 0 {
			return c
		}
		return cmp.Compare(a.OSMID, b.OSMID)
	})
	var kept []POI
	for _, s := range stations {
		duplicate := slices.ContainsFunc(kept, func(k POI) bool {
			return k.Name == s.Name && haversineMeters(k.Lng, k.Lat, s.Lng, s.Lat) <= subwayDedupeMeters
		})
		if !duplicate {
			kept = append(kept, s)
		}
	}
	return append(result, kept...)
}

const earthRadiusMeters = 6371008.8

func haversineMeters(lng1, lat1, lng2, lat2 float64) float64 {
	phi1, phi2 := lat1*math.Pi/180, lat2*math.Pi/180
	dPhi := phi2 - phi1
	dLambda := (lng2 - lng1) * math.Pi / 180
	h := math.Sin(dPhi/2)*math.Sin(dPhi/2) + math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	return 2 * earthRadiusMeters * math.Asin(math.Sqrt(h))
}
```

- [ ] **Step 6: 运行测试，确认通过**

Run: `cd backend && go test ./internal/importer/ -v && go vet ./... && test -z "$("$(go env GOROOT)/bin/gofmt" -l .)"`
Expected: 10 个测试全部 PASS（DataV 4 个、Overpass 6 个），`go vet` 和 `gofmt` 都没有输出。

- [ ] **Step 7: 提交**

```bash
git add backend/internal/importer
git commit -m "feat(importer): parse DataV districts and Overpass POIs with classification and subway dedupe"
```

---

### Task 5: 数据快照、拉取脚本与数据说明

**Files:**
- Create: `backend/internal/importer/files.go`
- Test: `backend/internal/importer/snapshot_test.go`
- Create: `data/raw/beijing_districts_gcj02.json`（下载）
- Create: `data/raw/beijing_pois_osm.json`（由脚本生成）
- Create: `scripts/fetch_osm_pois.sh`（可执行）
- Create: `data/README.md`

**Interfaces:**
- Consumes: `importer.ParseDistricts`、`importer.ParsePOIs`（Task 4）
- Produces:
  - 常量 `importer.DistrictsFile = "beijing_districts_gcj02.json"`、`importer.POIsFile = "beijing_pois_osm.json"`：导入器在 `DataDir` 下按这两个文件名读取
  - 提交进仓库的两份快照；`scripts/fetch_osm_pois.sh [输出路径]`，环境变量 `OVERPASS_URL` 可以覆盖接口地址

- [ ] **Step 1: 定义文件名常量并编写失败的快照测试**

创建 `backend/internal/importer/files.go`：

```go
package importer

const (
	DistrictsFile = "beijing_districts_gcj02.json"
	POIsFile      = "beijing_pois_osm.json"
)
```

创建 `backend/internal/importer/snapshot_test.go`：

```go
package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var snapshotDir = filepath.Join("..", "..", "..", "data", "raw")

func TestRealDistrictSnapshotParses(t *testing.T) {
	f, err := os.Open(filepath.Join(snapshotDir, DistrictsFile))
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer f.Close()

	districts, err := ParseDistricts(f)
	if err != nil {
		t.Fatalf("ParseDistricts: %v", err)
	}
	if len(districts) != 16 {
		t.Fatalf("got %d districts, want 16", len(districts))
	}
	for _, d := range districts {
		if !strings.HasPrefix(d.Adcode, "1101") {
			t.Errorf("district %s %s is not a Beijing district", d.Adcode, d.Name)
		}
	}
}

func TestRealPOISnapshotParses(t *testing.T) {
	f, err := os.Open(filepath.Join(snapshotDir, POIsFile))
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer f.Close()

	pois, err := ParsePOIs(f)
	if err != nil {
		t.Fatalf("ParsePOIs: %v", err)
	}
	if len(pois) < 4000 || len(pois) > 4300 {
		t.Fatalf("got %d POIs, want between 4000 and 4300", len(pois))
	}
	counts := map[string]int{}
	for _, p := range pois {
		counts[p.Category]++
	}
	for _, code := range []string{"subway_station", "university", "attraction", "hospital", "park"} {
		if counts[code] == 0 {
			t.Errorf("no POIs in category %s", code)
		}
	}
}
```

Run: `cd backend && go test ./internal/importer/ -run RealSnapshot`
Expected: 2 个测试 FAIL，报错 `open snapshot: ... no such file or directory`。

- [ ] **Step 2: 下载 DataV 行政区边界**

```bash
mkdir -p data/raw
curl -sSf -o data/raw/beijing_districts_gcj02.json https://geo.datav.aliyun.com/areas_v3/bound/110000_full.json
python3 -c "import json; d=json.load(open('data/raw/beijing_districts_gcj02.json')); print(len(d['features']), sorted(f['properties']['adcode'] for f in d['features'])[:3])"
```

Expected: 输出 `16 [110101, 110102, 110105]`，文件约 100 KB。

- [ ] **Step 3: 编写 OSM 拉取脚本**

创建 `scripts/fetch_osm_pois.sh`：

```bash
#!/usr/bin/env bash
# 重新拉取北京 POI 快照（OpenStreetMap，经 Overpass API）。只在需要更新快照时手动运行，CI 不会运行它。
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
out="${1:-$repo_root/data/raw/beijing_pois_osm.json}"
endpoint="${OVERPASS_URL:-https://overpass-api.de/api/interpreter}"
# Overpass 会拒绝没有 User-Agent 的请求（HTTP 406）。
user_agent="beijing-geo-map (https://github.com/slatt-oo/beijing-geo-map)"
min_elements=1000

query='[out:json][timeout:180];
area["ISO3166-2"="CN-BJ"]["admin_level"="4"]->.bj;
(
  nwr["railway"="station"]["station"="subway"](area.bj);
  nwr["public_transport"="station"]["subway"="yes"](area.bj);
  nwr["amenity"~"^(university|hospital)$"]["name"](area.bj);
  nwr["tourism"~"^(attraction|museum)$"]["name"](area.bj);
  nwr["leisure"="park"]["name"](area.bj);
);
out center tags;'

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

count_elements() {
  python3 -c 'import json, sys; print(len(json.load(open(sys.argv[1]))["elements"]))' "$1" 2>/dev/null || echo 0
}

for delay in 30 60 120 0; do
  code="$(curl -sS -m 240 -A "$user_agent" --data-urlencode "data=$query" -o "$tmp" -w '%{http_code}' "$endpoint" || true)"
  if [ "$code" = 200 ]; then
    count="$(count_elements "$tmp")"
    if [ "$count" -ge "$min_elements" ]; then
      mv "$tmp" "$out"
      chmod 644 "$out"
      trap - EXIT
      echo "saved $count elements to $out"
      exit 0
    fi
    echo "Overpass returned only $count elements (need at least $min_elements)" >&2
  else
    echo "Overpass returned HTTP $code" >&2
  fi
  if [ "$delay" -eq 0 ]; then
    break
  fi
  echo "retrying in ${delay}s..." >&2
  sleep "$delay"
done

echo "failed to fetch POIs from Overpass; the existing snapshot was left unchanged" >&2
exit 1
```

```bash
chmod +x scripts/fetch_osm_pois.sh
```

- [ ] **Step 4: 运行脚本生成 POI 快照**

Run: `./scripts/fetch_osm_pois.sh`（Shell 的 `block_until_ms` 设为 600000，最坏情况下包括三次重试约需 9 分钟）
Expected: 输出 `saved <N> elements to .../data/raw/beijing_pois_osm.json`，N 约为 4180，文件约 1.1 MB。

如果三次重试后仍然失败，改用协调者在 2026-10-13 用同一条查询拉取的快照：`cp .superpowers/sdd/spike/bj_pois.json data/raw/beijing_pois_osm.json && chmod 644 data/raw/beijing_pois_osm.json`，并在报告中写明使用了备用快照。

- [ ] **Step 5: 编写数据说明**

创建 `data/README.md`：

````markdown
# 数据快照

| 文件 | 内容 | 来源 | 坐标系 | 许可 |
|---|---|---|---|---|
| `raw/beijing_districts_gcj02.json` | 北京市 16 个区的边界 | [阿里云 DataV.GeoAtlas](https://datav.aliyun.com/portal/school/atlas/area_selector)：`https://geo.datav.aliyun.com/areas_v3/bound/110000_full.json` | GCJ-02 | 数据来自高德开放平台，仅用于学习和非商业演示 |
| `raw/beijing_pois_osm.json` | 地铁站、高校、景点/博物馆、医院、公园 | [OpenStreetMap](https://www.openstreetmap.org/)，经 Overpass API 导出，内容为接口原样输出 | WGS-84 | [ODbL](https://opendatacommons.org/licenses/odbl/)，© OpenStreetMap contributors |

两份快照都拉取于 2026-10-13，POI 快照的确切数据时间见文件中的 `osm3s.timestamp_osm_base` 字段。

导入时，行政区边界会从 GCJ-02 转换为 WGS-84，POI 本来就是 WGS-84，不需要转换。分类、取名和地铁站去重规则见[技术方案](../docs/02-technical-design.md)第 5.5 节。

## 更新快照

- **POI**：运行 `make fetch-osm`（即 `scripts/fetch_osm_pois.sh`）。脚本会带上 User-Agent，失败时间隔 30、60、120 秒重试，只有返回的要素不少于 1000 个时才会覆盖现有文件。
- **行政区边界**：

  ```bash
  curl -sSf -o data/raw/beijing_districts_gcj02.json https://geo.datav.aliyun.com/areas_v3/bound/110000_full.json
  ```

更新后运行 `make seed` 重新导入。导入是幂等的：已有记录会被更新，快照里已经没有的 POI 会被删除。
````

- [ ] **Step 6: 运行快照测试，确认通过**

Run: `cd backend && go test ./internal/importer/ -v && go vet ./...`
Expected: 12 个测试全部 PASS，其中 `TestRealPOISnapshotParses` 解析出约 4110 个 POI。

- [ ] **Step 7: 提交**

```bash
git add backend/internal/importer/files.go backend/internal/importer/snapshot_test.go data scripts/fetch_osm_pois.sh
git commit -m "feat(data): add DataV and OSM snapshots, Overpass fetch script and data README"
```

用 `git ls-files -s scripts/fetch_osm_pois.sh` 确认文件模式为 `100755`。

---

### Task 6: 导入流程（事务内 upsert、同步删除、重算所属区）与导入命令

**Files:**
- Create: `backend/internal/importer/importer.go`
- Create: `backend/cmd/importer/main.go`
- Create: `backend/internal/importer/testdata/beijing_districts_gcj02.json`
- Create: `backend/internal/importer/testdata/beijing_pois_osm.json`
- Test: `backend/internal/importer/importer_integration_test.go`

**Interfaces:**
- Consumes: `importer.ParseDistricts`、`importer.ParsePOIs`、`importer.DistrictsFile`、`importer.POIsFile`、`migrations.Up`、`config.Load`（`DataDir`）、`testdb.New`
- Produces:
  - `type importer.Options struct { DataDir string; MinPOIs int }`
  - `type importer.Stats struct { Districts int; POIs int; POIsWithDistrict int; POIsByCategory map[string]int; DeletedPOIs int64 }`：`POIsByCategory` 只包含数量大于 0 的分类
  - `func importer.Run(ctx context.Context, pool *pgxpool.Pool, opts Options) (Stats, error)`：快照中的 POI 少于 `MinPOIs` 时直接返回错误，不碰数据库；其余步骤在同一个事务中完成
  - 镜像中的可执行文件 `importer`（由 Dockerfile 的 `./cmd/...` 自动构建）：先执行迁移，再导入 `DATA_DIR` 下的快照

- [ ] **Step 1: 创建测试快照**

创建 `backend/internal/importer/testdata/beijing_districts_gcj02.json`（两个相邻的正方形区，坐标为 GCJ-02）：

```json
{"type":"FeatureCollection","features":[
  {"type":"Feature","properties":{"adcode":990001,"name":"测试甲区"},"geometry":{"type":"MultiPolygon","coordinates":[[[[116.0,40.0],[116.1,40.0],[116.1,40.1],[116.0,40.1],[116.0,40.0]]]]}},
  {"type":"Feature","properties":{"adcode":990002,"name":"测试乙区"},"geometry":{"type":"MultiPolygon","coordinates":[[[[116.1,40.0],[116.2,40.0],[116.2,40.1],[116.1,40.1],[116.1,40.0]]]]}}
]}
```

创建 `backend/internal/importer/testdata/beijing_pois_osm.json`（坐标为 WGS-84）：

```json
{"version":0.6,"generator":"test fixture","elements":[
  {"type":"node","id":1001,"lat":40.05,"lon":116.05,"tags":{"amenity":"university","name":"测试大学","name:en":"Test University","addr:full":"测试路1号","website":"https://example.edu"}},
  {"type":"way","id":2001,"center":{"lat":40.06,"lon":116.15},"tags":{"leisure":"park","tourism":"attraction","name":"测试园林"}},
  {"type":"way","id":2002,"center":{"lat":40.04,"lon":116.16},"tags":{"leisure":"park","name":"乙区公园"}},
  {"type":"relation","id":3001,"center":{"lat":40.03,"lon":116.07},"tags":{"amenity":"hospital","name:zh":"测试医院","name":"Test Hospital"}},
  {"type":"node","id":1002,"lat":40.02,"lon":116.02,"tags":{"railway":"station","station":"subway","name":"测试站"}},
  {"type":"node","id":1003,"lat":40.0205,"lon":116.0205,"tags":{"public_transport":"station","subway":"yes","name":"测试站"}},
  {"type":"node","id":1004,"lat":41.0,"lon":117.0,"tags":{"tourism":"museum","name":"区外博物馆"}},
  {"type":"node","id":1005,"lat":40.05,"lon":116.12,"tags":{"amenity":"hospital"}},
  {"type":"node","id":1006,"lat":40.05,"lon":116.13,"tags":{"shop":"supermarket","name":"不相关超市"}}
]}
```

解析后应保留 6 个 POI：n1001、w2001（公园兼景点，归为景点）、w2002、r3001（取 `name:zh`）、n1002、n1004（位于两个区之外）。n1003 与 n1002 同名、相距约 70 米，被去重；n1005 没有名称，n1006 无法分类，都被丢弃。转换为 WGS-84 后，甲区的经度范围约为 115.994 到 116.094，乙区约为 116.094 到 116.194，所以 n1001、r3001、n1002 落在甲区，w2001、w2002 落在乙区。

- [ ] **Step 2: 编写失败的集成测试**

创建 `backend/internal/importer/importer_integration_test.go`：

```go
//go:build integration

package importer_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/geo"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/importer"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/testdb"
)

const fixtureDir = "testdata"

func runImport(t *testing.T, pool *pgxpool.Pool, dir string, minPOIs int) importer.Stats {
	t.Helper()
	stats, err := importer.Run(context.Background(), pool, importer.Options{DataDir: dir, MinPOIs: minPOIs})
	if err != nil {
		t.Fatalf("import %s: %v", dir, err)
	}
	return stats
}

type storedPOI struct {
	ID       int64
	Name     string
	Category string
	Adcode   *string
}

func storedPOIs(t *testing.T, pool *pgxpool.Pool) map[string]storedPOI {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT p.osm_type || p.osm_id, p.id, p.name, c.code, d.adcode
		FROM poi p
		JOIN poi_category c ON c.id = p.category_id
		LEFT JOIN district d ON d.id = p.district_id`)
	if err != nil {
		t.Fatalf("query pois: %v", err)
	}
	type keyed struct {
		key string
		poi storedPOI
	}
	collected, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (keyed, error) {
		var k keyed
		err := row.Scan(&k.key, &k.poi.ID, &k.poi.Name, &k.poi.Category, &k.poi.Adcode)
		return k, err
	})
	if err != nil {
		t.Fatalf("scan pois: %v", err)
	}
	result := make(map[string]storedPOI, len(collected))
	for _, k := range collected {
		result[k.key] = k.poi
	}
	return result
}

func countPOIs(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM poi`).Scan(&n); err != nil {
		t.Fatalf("count pois: %v", err)
	}
	return n
}

func TestRunLoadsFixtureSnapshot(t *testing.T) {
	pool := testdb.New(t, "importer")

	stats := runImport(t, pool, fixtureDir, 1)

	want := importer.Stats{
		Districts:        2,
		POIs:             6,
		POIsWithDistrict: 5,
		POIsByCategory:   map[string]int{"subway_station": 1, "university": 1, "attraction": 2, "hospital": 1, "park": 1},
	}
	if !reflect.DeepEqual(stats, want) {
		t.Fatalf("stats = %+v, want %+v", stats, want)
	}

	pois := storedPOIs(t, pool)
	wantPOIs := map[string]struct{ name, category, adcode string }{
		"n1001": {"测试大学", "university", "990001"},
		"w2001": {"测试园林", "attraction", "990002"},
		"w2002": {"乙区公园", "park", "990002"},
		"r3001": {"测试医院", "hospital", "990001"},
		"n1002": {"测试站", "subway_station", "990001"},
		"n1004": {"区外博物馆", "attraction", ""},
	}
	if len(pois) != len(wantPOIs) {
		t.Fatalf("stored POIs = %v", pois)
	}
	for key, w := range wantPOIs {
		got, ok := pois[key]
		if !ok {
			t.Errorf("POI %s is missing", key)
			continue
		}
		adcode := ""
		if got.Adcode != nil {
			adcode = *got.Adcode
		}
		if got.Name != w.name || got.Category != w.category || adcode != w.adcode {
			t.Errorf("POI %s = {%s %s %q}, want {%s %s %q}", key, got.Name, got.Category, adcode, w.name, w.category, w.adcode)
		}
	}

	var nameEn, address, website string
	err := pool.QueryRow(context.Background(),
		`SELECT name_en, address, tags->>'website' FROM poi WHERE osm_type = 'n' AND osm_id = 1001`).
		Scan(&nameEn, &address, &website)
	if err != nil {
		t.Fatalf("query n1001: %v", err)
	}
	if nameEn != "Test University" || address != "测试路1号" || website != "https://example.edu" {
		t.Errorf("n1001 = (%q, %q, %q)", nameEn, address, website)
	}

	wantLng, wantLat := geo.GCJ02ToWGS84(116.05, 40.05)
	var gotLng, gotLat float64
	err = pool.QueryRow(context.Background(),
		`SELECT ST_X(ST_Centroid(geom)), ST_Y(ST_Centroid(geom)) FROM district WHERE adcode = '990001'`).
		Scan(&gotLng, &gotLat)
	if err != nil {
		t.Fatalf("query district centroid: %v", err)
	}
	if math.Abs(gotLng-wantLng) > 1e-5 || math.Abs(gotLat-wantLat) > 1e-5 {
		t.Fatalf("district centroid = (%v, %v), want about (%v, %v): boundary was not converted to WGS-84",
			gotLng, gotLat, wantLng, wantLat)
	}
}

func TestRunIsIdempotent(t *testing.T) {
	pool := testdb.New(t, "importer")
	runImport(t, pool, fixtureDir, 1)
	before := storedPOIs(t, pool)

	stats := runImport(t, pool, fixtureDir, 1)

	after := storedPOIs(t, pool)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("second import changed POIs:\nbefore %v\nafter  %v", before, after)
	}
	if stats.Districts != 2 || stats.POIs != 6 || stats.DeletedPOIs != 0 {
		t.Fatalf("second import stats = %+v", stats)
	}
}

func TestRunDeletesPOIsMissingFromSnapshot(t *testing.T) {
	pool := testdb.New(t, "importer")
	runImport(t, pool, fixtureDir, 1)

	dir := t.TempDir()
	copyFile(t, filepath.Join(fixtureDir, importer.DistrictsFile), filepath.Join(dir, importer.DistrictsFile))
	writeSnapshotWithout(t, filepath.Join(dir, importer.POIsFile), 2002)

	stats := runImport(t, pool, dir, 1)

	if stats.POIs != 5 || stats.DeletedPOIs != 1 {
		t.Fatalf("stats = %+v, want 5 POIs and 1 deleted", stats)
	}
	if _, ok := storedPOIs(t, pool)["w2002"]; ok {
		t.Fatal("w2002 should have been deleted")
	}
}

func TestRunRejectsSnapshotBelowMinimumWithoutTouchingData(t *testing.T) {
	pool := testdb.New(t, "importer")
	runImport(t, pool, fixtureDir, 1)

	_, err := importer.Run(context.Background(), pool, importer.Options{DataDir: fixtureDir, MinPOIs: 7})

	if err == nil || !strings.Contains(err.Error(), "minimum") {
		t.Fatalf("err = %v, want a minimum-size error", err)
	}
	if n := countPOIs(t, pool); n != 6 {
		t.Fatalf("POIs after rejected import = %d, want 6", n)
	}
}

func TestRunImportsRealSnapshot(t *testing.T) {
	pool := testdb.New(t, "importer")

	stats := runImport(t, pool, filepath.Join("..", "..", "..", "data", "raw"), 1000)

	if stats.Districts != 16 {
		t.Errorf("districts = %d, want 16", stats.Districts)
	}
	if stats.POIs < 4000 || stats.POIs > 4300 {
		t.Errorf("POIs = %d, want between 4000 and 4300", stats.POIs)
	}
	if ratio := float64(stats.POIsWithDistrict) / float64(stats.POIs); ratio < 0.95 {
		t.Errorf("district fill rate = %.4f, want at least 0.95", ratio)
	}
	for _, code := range []string{"subway_station", "university", "attraction", "hospital", "park"} {
		if stats.POIsByCategory[code] == 0 {
			t.Errorf("no POIs imported for category %s", code)
		}
	}
	var totalArea float64
	if err := pool.QueryRow(context.Background(), `SELECT sum(area_km2)::float8 FROM district`).Scan(&totalArea); err != nil {
		t.Fatalf("sum area: %v", err)
	}
	if totalArea < 16300 || totalArea > 16500 {
		t.Errorf("total area = %.2f km², want about 16410 km²", totalArea)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

func writeSnapshotWithout(t *testing.T, dst string, osmID int64) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir, importer.POIsFile))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var doc struct {
		Elements []map[string]any `json:"elements"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	kept := doc.Elements[:0]
	for _, el := range doc.Elements {
		if el["id"] != float64(osmID) {
			kept = append(kept, el)
		}
	}
	out, err := json.Marshal(map[string]any{"elements": kept})
	if err != nil {
		t.Fatalf("encode snapshot: %v", err)
	}
	if err := os.WriteFile(dst, out, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
}
```

Run:

```bash
cd backend && TEST_DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable' \
  go test -count=1 -tags=integration ./internal/importer/
```

Expected: 编译失败，报错 `undefined: importer.Run`、`undefined: importer.Options`、`undefined: importer.Stats`。

- [ ] **Step 3: 编写导入流程**

创建 `backend/internal/importer/importer.go`：

```go
package importer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	DataDir string
	// MinPOIs 是快照中 POI 数量的下限。快照不足这个数量时拒绝导入，避免损坏的快照把 poi 表同步删空。
	MinPOIs int
}

type Stats struct {
	Districts        int
	POIs             int
	POIsWithDistrict int
	POIsByCategory   map[string]int
	DeletedPOIs      int64
}

// Run 导入 DataDir 下的两份快照。除了快照校验，其余步骤都在同一个事务里完成，任何一步失败都会整体回滚。
func Run(ctx context.Context, pool *pgxpool.Pool, opts Options) (Stats, error) {
	districts, err := readDistricts(filepath.Join(opts.DataDir, DistrictsFile))
	if err != nil {
		return Stats{}, err
	}
	pois, err := readPOIs(filepath.Join(opts.DataDir, POIsFile))
	if err != nil {
		return Stats{}, err
	}
	if len(pois) < opts.MinPOIs {
		return Stats{}, fmt.Errorf("snapshot has %d POIs, fewer than the minimum %d; refusing to import", len(pois), opts.MinPOIs)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := upsertDistricts(ctx, tx, districts); err != nil {
		return Stats{}, err
	}
	categoryIDs, err := loadCategoryIDs(ctx, tx)
	if err != nil {
		return Stats{}, err
	}
	if err := upsertPOIs(ctx, tx, pois, categoryIDs); err != nil {
		return Stats{}, err
	}
	deleted, err := deleteMissingPOIs(ctx, tx, pois)
	if err != nil {
		return Stats{}, err
	}
	if _, err := tx.Exec(ctx, assignDistrictsSQL); err != nil {
		return Stats{}, fmt.Errorf("assign districts: %w", err)
	}
	stats, err := collectStats(ctx, tx)
	if err != nil {
		return Stats{}, err
	}
	stats.DeletedPOIs = deleted
	if err := tx.Commit(ctx); err != nil {
		return Stats{}, fmt.Errorf("commit: %w", err)
	}
	return stats, nil
}

func readDistricts(path string) ([]District, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open districts snapshot: %w", err)
	}
	defer f.Close()
	districts, err := ParseDistricts(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return districts, nil
}

func readPOIs(path string) ([]POI, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open POI snapshot: %w", err)
	}
	defer f.Close()
	pois, err := ParsePOIs(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return pois, nil
}

const upsertDistrictSQL = `
WITH src AS (
    SELECT ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON($3::text), 4326)), 3)) AS g
)
INSERT INTO district (adcode, name, center, geom, area_km2)
SELECT $1::text, $2::text, ST_PointOnSurface(g), g, round((ST_Area(g::geography) / 1e6)::numeric, 2)
FROM src
ON CONFLICT (adcode) DO UPDATE
SET name       = EXCLUDED.name,
    center     = EXCLUDED.center,
    geom       = EXCLUDED.geom,
    area_km2   = EXCLUDED.area_km2,
    updated_at = now()`

func upsertDistricts(ctx context.Context, tx pgx.Tx, districts []District) error {
	batch := &pgx.Batch{}
	for _, d := range districts {
		batch.Queue(upsertDistrictSQL, d.Adcode, d.Name, string(d.Geometry))
	}
	results := tx.SendBatch(ctx, batch)
	for _, d := range districts {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("upsert district %s: %w", d.Adcode, err)
		}
	}
	return results.Close()
}

func loadCategoryIDs(ctx context.Context, tx pgx.Tx) (map[string]int16, error) {
	rows, err := tx.Query(ctx, `SELECT code, id FROM poi_category`)
	if err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	ids := map[string]int16{}
	var code string
	var id int16
	if _, err := pgx.ForEachRow(rows, []any{&code, &id}, func() error {
		ids[code] = id
		return nil
	}); err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	return ids, nil
}

const upsertPOISQL = `
INSERT INTO poi (osm_type, osm_id, name, name_en, category_id, geom, address, tags)
VALUES ($1, $2, $3, $4, $5, ST_SetSRID(ST_MakePoint($6, $7), 4326), $8, $9)
ON CONFLICT (osm_type, osm_id) DO UPDATE
SET name        = EXCLUDED.name,
    name_en     = EXCLUDED.name_en,
    category_id = EXCLUDED.category_id,
    geom        = EXCLUDED.geom,
    address     = EXCLUDED.address,
    tags        = EXCLUDED.tags,
    updated_at  = now()`

func upsertPOIs(ctx context.Context, tx pgx.Tx, pois []POI, categoryIDs map[string]int16) error {
	batch := &pgx.Batch{}
	for _, p := range pois {
		categoryID, ok := categoryIDs[p.Category]
		if !ok {
			return fmt.Errorf("poi %s%d: unknown category %q", p.OSMType, p.OSMID, p.Category)
		}
		batch.Queue(upsertPOISQL, p.OSMType, p.OSMID, p.Name, p.NameEn, categoryID, p.Lng, p.Lat, p.Address, p.Tags)
	}
	results := tx.SendBatch(ctx, batch)
	for _, p := range pois {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("upsert poi %s%d: %w", p.OSMType, p.OSMID, err)
		}
	}
	return results.Close()
}

func deleteMissingPOIs(ctx context.Context, tx pgx.Tx, pois []POI) (int64, error) {
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE snapshot_keys (osm_type char(1), osm_id bigint) ON COMMIT DROP`); err != nil {
		return 0, fmt.Errorf("create snapshot_keys: %w", err)
	}
	keys := make([][]any, len(pois))
	for i, p := range pois {
		keys[i] = []any{p.OSMType, p.OSMID}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"snapshot_keys"}, []string{"osm_type", "osm_id"}, pgx.CopyFromRows(keys)); err != nil {
		return 0, fmt.Errorf("copy snapshot keys: %w", err)
	}
	tag, err := tx.Exec(ctx, `
DELETE FROM poi p
WHERE NOT EXISTS (
    SELECT 1 FROM snapshot_keys k
    WHERE k.osm_type = p.osm_type AND k.osm_id = p.osm_id
)`)
	if err != nil {
		return 0, fmt.Errorf("delete missing pois: %w", err)
	}
	return tag.RowsAffected(), nil
}

// 一条语句同时完成分配和清空：不在任何区内的 POI 得到 NULL。多个区都包含同一个点时取 adcode 最小的区，
// 保证重复导入的结果稳定。
const assignDistrictsSQL = `
UPDATE poi p
SET district_id = m.district_id, updated_at = now()
FROM (
    SELECT p2.id AS poi_id,
           (SELECT d.id FROM district d WHERE ST_Contains(d.geom, p2.geom) ORDER BY d.adcode LIMIT 1) AS district_id
    FROM poi p2
) m
WHERE m.poi_id = p.id
  AND p.district_id IS DISTINCT FROM m.district_id`

func collectStats(ctx context.Context, tx pgx.Tx) (Stats, error) {
	var s Stats
	err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM district), (SELECT count(*) FROM poi), (SELECT count(district_id) FROM poi)`).
		Scan(&s.Districts, &s.POIs, &s.POIsWithDistrict)
	if err != nil {
		return Stats{}, fmt.Errorf("count rows: %w", err)
	}
	rows, err := tx.Query(ctx, `SELECT c.code, count(*) FROM poi p JOIN poi_category c ON c.id = p.category_id GROUP BY c.code`)
	if err != nil {
		return Stats{}, fmt.Errorf("count pois by category: %w", err)
	}
	s.POIsByCategory = map[string]int{}
	var code string
	var n int
	if _, err := pgx.ForEachRow(rows, []any{&code, &n}, func() error {
		s.POIsByCategory[code] = n
		return nil
	}); err != nil {
		return Stats{}, fmt.Errorf("count pois by category: %w", err)
	}
	return s, nil
}
```

- [ ] **Step 4: 运行集成测试，确认通过**

Run:

```bash
cd backend && TEST_DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable' \
  go test -count=1 -tags=integration ./internal/importer/ -v
```

Expected: 5 个集成测试和 12 个单元测试全部 PASS。`TestRunImportsRealSnapshot` 导入 16 个区、约 4110 个 POI，所属区匹配率约 99.6%。

- [ ] **Step 5: 编写导入命令**

创建 `backend/cmd/importer/main.go`：

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/config"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/importer"
	"github.com/slatt-oo/beijing-geo-map/backend/migrations"
)

// 正常快照约有 4100 个 POI；少于这个数量说明快照损坏，继续导入会把 poi 表同步删空。
const minSnapshotPOIs = 1000

func main() {
	if err := run(); err != nil {
		slog.Error("import failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := migrations.Up(cfg.DatabaseURL); err != nil {
		return err
	}
	slog.Info("migrations applied")

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}
	defer pool.Close()

	start := time.Now()
	stats, err := importer.Run(ctx, pool, importer.Options{DataDir: cfg.DataDir, MinPOIs: minSnapshotPOIs})
	if err != nil {
		return err
	}
	slog.Info("import finished",
		"districts", stats.Districts,
		"pois", stats.POIs,
		"pois_with_district", stats.POIsWithDistrict,
		"deleted_pois", stats.DeletedPOIs,
		"pois_by_category", stats.POIsByCategory,
		"elapsed_ms", time.Since(start).Milliseconds(),
	)
	return nil
}
```

- [ ] **Step 6: 对开发库运行两次导入命令**

```bash
cd backend
DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable' DATA_DIR=../data/raw go run ./cmd/importer
DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable' DATA_DIR=../data/raw go run ./cmd/importer
```

Expected:
- 第一次依次输出 JSON 日志 `"msg":"migrations applied"` 和 `"msg":"import finished"`，其中 `"districts":16`，`"pois"` 约为 4110，`"deleted_pois":0`。
- 第二次的 `"pois"` 和 `"districts"` 与第一次相同，`"deleted_pois":0`。

- [ ] **Step 7: 运行全部后端检查并提交**

Run: `cd backend && go test ./... && go vet -tags=integration ./... && test -z "$("$(go env GOROOT)/bin/gofmt" -l .)" && cd .. && make test-integration`
Expected: 全部 PASS，没有输出错误。

```bash
git add backend/internal/importer backend/cmd/importer
git commit -m "feat(importer): transactional import with sync delete, district assignment and importer command"
```

---

### Task 7: repository 数据访问层与数据模型

**Files:**
- Create: `backend/internal/model/model.go`
- Create: `backend/internal/repository/errors.go`
- Create: `backend/internal/repository/like.go`
- Create: `backend/internal/repository/district.go`
- Create: `backend/internal/repository/category.go`
- Create: `backend/internal/repository/poi.go`
- Test: `backend/internal/repository/like_test.go`、`backend/internal/repository/repository_integration_test.go`

**Interfaces:**
- Consumes: 迁移后的表结构（Task 3）、`testdb.New`
- Produces（阶段 2 的 service 层直接依赖这些）：
  - `model` 包：`Category{ID int16; Code, Name, Color string}`、`DistrictRef{Adcode, Name string}`、`DistrictFeature{Adcode, Name string; AreaKm2 float64; POICount int; Center [2]float64; Geometry json.RawMessage}`、`CategoryCount{Code, Name string; Count int}`、`DistrictDetail{Adcode, Name string; AreaKm2 float64; Center [2]float64; POICounts []CategoryCount}`、`POISummary{ID int64; Name, Category string; Location [2]float64}`、`NearbyPOI{POISummary; DistanceM int}`、`SearchHit{POISummary; DistrictName *string}`、`POIDetail{ID int64; Name string; NameEn *string; OSMType string; OSMID int64; Category Category; District *DistrictRef; Location [2]float64; Address *string; Tags map[string]string}`、`BBox{MinLng, MinLat, MaxLng, MaxLat float64}`
  - `var repository.ErrNotFound`
  - `NewDistrictRepo(*pgxpool.Pool) *DistrictRepo`，方法：`List(ctx) ([]model.DistrictFeature, error)`、`Get(ctx, adcode string) (model.DistrictDetail, error)`（不存在时返回 `ErrNotFound`）、`Locate(ctx, lng, lat float64) (*model.DistrictRef, error)`（点不在任何区内时返回 `nil, nil`）
  - `NewCategoryRepo(*pgxpool.Pool) *CategoryRepo`，方法：`List(ctx) ([]model.Category, error)`，按 `sort_order` 排序
  - `NewPOIRepo(*pgxpool.Pool) *POIRepo`，方法：`InBBox(ctx, box model.BBox, categoryIDs []int16, limit int) ([]model.POISummary, bool, error)`、`Get(ctx, id int64) (model.POIDetail, error)`、`Search(ctx, q string, limit int) ([]model.SearchHit, error)`、`Nearby(ctx, lng, lat, radiusM float64, categoryIDs []int16, limit int) ([]model.NearbyPOI, error)`。`categoryIDs` 为 nil 表示不按分类过滤
  - 参数校验（经纬度范围、`adcode` 是否为 6 位数字、`limit` 上限等）由阶段 2 的 handler 负责，repository 假定输入合法

- [ ] **Step 1: 编写失败的转义单元测试**

创建 `backend/internal/repository/like_test.go`：

```go
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
```

Run: `cd backend && go test ./internal/repository/`
Expected: 编译失败，报错 `undefined: escapeLike`（或该目录中没有非测试 Go 文件）。

- [ ] **Step 2: 编写数据模型、错误和转义函数**

创建 `backend/internal/model/model.go`：

```go
// Package model 定义 repository 返回的数据结构。坐标一律为 WGS-84 的 [经度, 纬度]。
package model

import "encoding/json"

type Category struct {
	ID    int16
	Code  string
	Name  string
	Color string
}

type DistrictRef struct {
	Adcode string
	Name   string
}

type DistrictFeature struct {
	Adcode   string
	Name     string
	AreaKm2  float64
	POICount int
	Center   [2]float64
	// Geometry 是按约 10 米容差简化后的 GeoJSON MultiPolygon，坐标保留 6 位小数。
	Geometry json.RawMessage
}

type CategoryCount struct {
	Code  string
	Name  string
	Count int
}

type DistrictDetail struct {
	Adcode    string
	Name      string
	AreaKm2   float64
	Center    [2]float64
	POICounts []CategoryCount
}

type POISummary struct {
	ID       int64
	Name     string
	Category string
	Location [2]float64
}

type NearbyPOI struct {
	POISummary
	DistanceM int
}

type SearchHit struct {
	POISummary
	DistrictName *string
}

type POIDetail struct {
	ID       int64
	Name     string
	NameEn   *string
	OSMType  string
	OSMID    int64
	Category Category
	District *DistrictRef
	Location [2]float64
	Address  *string
	Tags     map[string]string
}

type BBox struct {
	MinLng, MinLat, MaxLng, MaxLat float64
}
```

创建 `backend/internal/repository/errors.go`：

```go
package repository

import "errors"

var ErrNotFound = errors.New("repository: not found")
```

创建 `backend/internal/repository/like.go`：

```go
package repository

import "strings"

// PostgreSQL 中 LIKE/ILIKE 的默认转义字符是反斜杠。
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string {
	return likeEscaper.Replace(s)
}
```

Run: `cd backend && go test ./internal/repository/ -v`
Expected: `TestEscapeLike` PASS。

- [ ] **Step 3: 编写失败的集成测试**

创建 `backend/internal/repository/repository_integration_test.go`：

```go
//go:build integration

package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/repository"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/testdb"
)

// 测试数据：两个相邻的正方形区（甲区经度 116.0~116.1，乙区 116.1~116.2，纬度都是 40.0~40.1）和 8 个 POI。
// 北京大学医学部、清华大学分别位于北京大学正北 499.66 米、999.32 米处（WGS-84 椭球上的子午线弧长）。
var boxA = model.BBox{MinLng: 116.00, MinLat: 40.00, MaxLng: 116.10, MaxLat: 40.10}

type fixture struct {
	pool   *pgxpool.Pool
	poiIDs map[string]int64
}

func ptr[T any](v T) *T { return &v }

func setup(t *testing.T) fixture {
	t.Helper()
	pool := testdb.New(t, "repository")
	districtA := insertDistrict(t, pool, "990001", "测试甲区", 116.00, 40.00, 116.10, 40.10)
	districtB := insertDistrict(t, pool, "990002", "测试乙区", 116.10, 40.00, 116.20, 40.10)

	f := fixture{pool: pool, poiIDs: map[string]int64{}}
	for i, p := range []struct {
		name     string
		nameEn   *string
		category string
		district *int32
		lng, lat float64
		tags     map[string]string
	}{
		{"北京大学", ptr("Peking University"), "university", &districtA, 116.050, 40.050, map[string]string{"website": "https://www.pku.edu.cn"}},
		{"北京大学医学部", nil, "university", &districtA, 116.050, 40.0545, map[string]string{}},
		{"清华大学", nil, "university", &districtA, 116.050, 40.059, map[string]string{}},
		{"测试地铁站", nil, "subway_station", &districtA, 116.020, 40.020, map[string]string{}},
		{"测试公园", nil, "park", &districtB, 116.150, 40.050, map[string]string{}},
		{"100%纯净公园", nil, "park", &districtB, 116.160, 40.050, map[string]string{}},
		{"a_b 博物馆", nil, "attraction", &districtB, 116.170, 40.050, map[string]string{}},
		{"境外医院", nil, "hospital", nil, 117.000, 41.000, map[string]string{}},
	} {
		var id int64
		err := pool.QueryRow(context.Background(), `
			INSERT INTO poi (osm_type, osm_id, name, name_en, category_id, district_id, geom, tags)
			VALUES ('n', $1, $2, $3, (SELECT id FROM poi_category WHERE code = $4), $5,
			        ST_SetSRID(ST_MakePoint($6, $7), 4326), $8)
			RETURNING id`,
			int64(i+1), p.name, p.nameEn, p.category, p.district, p.lng, p.lat, p.tags).Scan(&id)
		if err != nil {
			t.Fatalf("insert poi %s: %v", p.name, err)
		}
		f.poiIDs[p.name] = id
	}
	return f
}

func insertDistrict(t *testing.T, pool *pgxpool.Pool, adcode, name string, minLng, minLat, maxLng, maxLat float64) int32 {
	t.Helper()
	var id int32
	err := pool.QueryRow(context.Background(), `
		INSERT INTO district (adcode, name, center, geom, area_km2)
		SELECT $1::text, $2::text, ST_PointOnSurface(g), g, round((ST_Area(g::geography) / 1e6)::numeric, 2)
		FROM (SELECT ST_Multi(ST_MakeEnvelope($3, $4, $5, $6, 4326)) AS g) s
		RETURNING id`, adcode, name, minLng, minLat, maxLng, maxLat).Scan(&id)
	if err != nil {
		t.Fatalf("insert district %s: %v", adcode, err)
	}
	return id
}

func categoryID(t *testing.T, pool *pgxpool.Pool, code string) int16 {
	t.Helper()
	var id int16
	if err := pool.QueryRow(context.Background(), `SELECT id FROM poi_category WHERE code = $1`, code).Scan(&id); err != nil {
		t.Fatalf("category %s: %v", code, err)
	}
	return id
}

func summaryNames(pois []model.POISummary) []string {
	names := make([]string, len(pois))
	for i, p := range pois {
		names[i] = p.Name
	}
	return names
}

func nearbyNames(pois []model.NearbyPOI) []string {
	names := make([]string, len(pois))
	for i, p := range pois {
		names[i] = p.Name
	}
	return names
}

func hitNames(hits []model.SearchHit) []string {
	names := make([]string, len(hits))
	for i, h := range hits {
		names[i] = h.Name
	}
	return names
}

func TestLocateReturnsContainingDistrict(t *testing.T) {
	f := setup(t)
	repo := repository.NewDistrictRepo(f.pool)
	for _, tc := range []struct {
		lng, lat float64
		adcode   string
	}{{116.05, 40.05, "990001"}, {116.15, 40.05, "990002"}} {
		ref, err := repo.Locate(context.Background(), tc.lng, tc.lat)
		if err != nil {
			t.Fatalf("Locate: %v", err)
		}
		if ref == nil || ref.Adcode != tc.adcode {
			t.Fatalf("Locate(%v, %v) = %+v, want %s", tc.lng, tc.lat, ref, tc.adcode)
		}
	}
}

func TestLocateReturnsNilOutsideAllDistricts(t *testing.T) {
	f := setup(t)
	ref, err := repository.NewDistrictRepo(f.pool).Locate(context.Background(), 117.0, 41.0)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if ref != nil {
		t.Fatalf("Locate outside = %+v, want nil", ref)
	}
}

func TestListDistrictsReturnsCountsCentersAndGeometry(t *testing.T) {
	f := setup(t)
	districts, err := repository.NewDistrictRepo(f.pool).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(districts) != 2 || districts[0].Adcode != "990001" || districts[1].Adcode != "990002" {
		t.Fatalf("districts = %+v", districts)
	}
	a, b := districts[0], districts[1]
	if a.Name != "测试甲区" || a.POICount != 4 || b.POICount != 3 {
		t.Errorf("A = {%s %d}, B count = %d; want {测试甲区 4} and 3", a.Name, a.POICount, b.POICount)
	}
	if a.AreaKm2 < 90 || a.AreaKm2 > 100 {
		t.Errorf("A area = %v km², want about 94.6 km² (0.1° × 0.1° at 40°N)", a.AreaKm2)
	}
	if a.Center[0] <= 116.00 || a.Center[0] >= 116.10 || a.Center[1] <= 40.00 || a.Center[1] >= 40.10 {
		t.Errorf("A center %v is not inside the district", a.Center)
	}
	var geometry struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(a.Geometry, &geometry); err != nil || geometry.Type != "MultiPolygon" {
		t.Errorf("A geometry = %s (err %v), want a GeoJSON MultiPolygon", a.Geometry, err)
	}
}

func TestGetDistrictReturnsCountsForEveryCategory(t *testing.T) {
	f := setup(t)
	d, err := repository.NewDistrictRepo(f.pool).Get(context.Background(), "990001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if d.Adcode != "990001" || d.Name != "测试甲区" {
		t.Fatalf("district = %+v", d)
	}
	want := []model.CategoryCount{
		{Code: "subway_station", Name: "地铁站", Count: 1},
		{Code: "university", Name: "高校", Count: 3},
		{Code: "attraction", Name: "景点/博物馆", Count: 0},
		{Code: "hospital", Name: "医院", Count: 0},
		{Code: "park", Name: "公园", Count: 0},
	}
	if !slices.Equal(d.POICounts, want) {
		t.Fatalf("POICounts = %+v, want %+v", d.POICounts, want)
	}
}

func TestGetDistrictReturnsNotFoundForUnknownAdcode(t *testing.T) {
	f := setup(t)
	_, err := repository.NewDistrictRepo(f.pool).Get(context.Background(), "999999")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListCategoriesInDisplayOrder(t *testing.T) {
	f := setup(t)
	categories, err := repository.NewCategoryRepo(f.pool).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var codes []string
	for _, c := range categories {
		codes = append(codes, c.Code)
		if c.ID == 0 || len(c.Color) != 7 || !strings.HasPrefix(c.Color, "#") {
			t.Errorf("category %+v has an invalid id or color", c)
		}
	}
	if want := []string{"subway_station", "university", "attraction", "hospital", "park"}; !slices.Equal(codes, want) {
		t.Fatalf("codes = %v, want %v", codes, want)
	}
}

func TestInBBoxFiltersByEnvelopeAndCategory(t *testing.T) {
	f := setup(t)
	repo := repository.NewPOIRepo(f.pool)

	pois, truncated, err := repo.InBBox(context.Background(), boxA, nil, 100)
	if err != nil {
		t.Fatalf("InBBox: %v", err)
	}
	if want := []string{"北京大学", "北京大学医学部", "清华大学", "测试地铁站"}; !slices.Equal(summaryNames(pois), want) || truncated {
		t.Fatalf("InBBox = %v (truncated %v), want %v", summaryNames(pois), truncated, want)
	}
	if pois[0].Category != "university" || pois[0].Location != [2]float64{116.05, 40.05} {
		t.Errorf("first POI = %+v", pois[0])
	}

	pois, _, err = repo.InBBox(context.Background(), boxA, []int16{categoryID(t, f.pool, "subway_station")}, 100)
	if err != nil {
		t.Fatalf("InBBox with category: %v", err)
	}
	if want := []string{"测试地铁站"}; !slices.Equal(summaryNames(pois), want) {
		t.Fatalf("InBBox subway = %v, want %v", summaryNames(pois), want)
	}
}

func TestInBBoxReportsTruncation(t *testing.T) {
	f := setup(t)
	repo := repository.NewPOIRepo(f.pool)

	pois, truncated, err := repo.InBBox(context.Background(), boxA, nil, 2)
	if err != nil {
		t.Fatalf("InBBox: %v", err)
	}
	if len(pois) != 2 || !truncated {
		t.Fatalf("limit 2: got %d POIs, truncated %v; want 2, true", len(pois), truncated)
	}

	pois, truncated, err = repo.InBBox(context.Background(), boxA, nil, 4)
	if err != nil {
		t.Fatalf("InBBox: %v", err)
	}
	if len(pois) != 4 || truncated {
		t.Fatalf("limit 4: got %d POIs, truncated %v; want 4, false", len(pois), truncated)
	}
}

func TestGetPOIReturnsDetail(t *testing.T) {
	f := setup(t)
	p, err := repository.NewPOIRepo(f.pool).Get(context.Background(), f.poiIDs["北京大学"])
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.Name != "北京大学" || p.NameEn == nil || *p.NameEn != "Peking University" || p.OSMType != "n" || p.OSMID != 1 {
		t.Errorf("identity fields = %+v", p)
	}
	if p.Category.Code != "university" || p.Category.Name != "高校" || p.Category.Color != "#1E88E5" {
		t.Errorf("category = %+v", p.Category)
	}
	if p.District == nil || *p.District != (model.DistrictRef{Adcode: "990001", Name: "测试甲区"}) {
		t.Errorf("district = %+v", p.District)
	}
	if p.Location != [2]float64{116.05, 40.05} || p.Address != nil || p.Tags["website"] != "https://www.pku.edu.cn" {
		t.Errorf("location/address/tags = %v / %v / %v", p.Location, p.Address, p.Tags)
	}
}

func TestGetPOIWithoutDistrict(t *testing.T) {
	f := setup(t)
	p, err := repository.NewPOIRepo(f.pool).Get(context.Background(), f.poiIDs["境外医院"])
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.District != nil {
		t.Fatalf("district = %+v, want nil", p.District)
	}
}

func TestGetPOIReturnsNotFound(t *testing.T) {
	f := setup(t)
	_, err := repository.NewPOIRepo(f.pool).Get(context.Background(), 999999)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSearchRanksExactMatchFirst(t *testing.T) {
	f := setup(t)
	hits, err := repository.NewPOIRepo(f.pool).Search(context.Background(), "北京大学", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if want := []string{"北京大学", "北京大学医学部"}; !slices.Equal(hitNames(hits), want) {
		t.Fatalf("Search = %v, want %v", hitNames(hits), want)
	}
	if hits[0].DistrictName == nil || *hits[0].DistrictName != "测试甲区" || hits[0].Category != "university" {
		t.Errorf("first hit = %+v", hits[0])
	}
}

func TestSearchTreatsWildcardsLiterally(t *testing.T) {
	f := setup(t)
	repo := repository.NewPOIRepo(f.pool)
	for q, want := range map[string][]string{"%": {"100%纯净公园"}, "_": {"a_b 博物馆"}} {
		hits, err := repo.Search(context.Background(), q, 10)
		if err != nil {
			t.Fatalf("Search(%q): %v", q, err)
		}
		if !slices.Equal(hitNames(hits), want) {
			t.Errorf("Search(%q) = %v, want %v", q, hitNames(hits), want)
		}
	}
}

func TestSearchMatchesTwoCharacterChinese(t *testing.T) {
	f := setup(t)
	hits, err := repository.NewPOIRepo(f.pool).Search(context.Background(), "清华", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if want := []string{"清华大学"}; !slices.Equal(hitNames(hits), want) {
		t.Fatalf("Search = %v, want %v", hitNames(hits), want)
	}
}

func TestChineseNamesProduceTrigrams(t *testing.T) {
	f := setup(t)
	var trigrams []string
	if err := f.pool.QueryRow(context.Background(), `SELECT show_trgm('北京大学')`).Scan(&trigrams); err != nil {
		t.Fatalf("show_trgm: %v", err)
	}
	if len(trigrams) == 0 {
		t.Fatal("show_trgm('北京大学') is empty: the database locale does not treat CJK characters as word characters")
	}
}

func TestNearbyOrdersByDistanceWithinRadius(t *testing.T) {
	f := setup(t)
	repo := repository.NewPOIRepo(f.pool)

	near, err := repo.Nearby(context.Background(), 116.05, 40.05, 600, nil, 50)
	if err != nil {
		t.Fatalf("Nearby 600: %v", err)
	}
	if want := []string{"北京大学", "北京大学医学部"}; !slices.Equal(nearbyNames(near), want) {
		t.Fatalf("Nearby 600 = %v, want %v", nearbyNames(near), want)
	}
	if near[0].DistanceM != 0 || abs(near[1].DistanceM-500) > 1 {
		t.Errorf("distances = %d, %d; want 0 and about 500", near[0].DistanceM, near[1].DistanceM)
	}

	near, err = repo.Nearby(context.Background(), 116.05, 40.05, 1000, nil, 50)
	if err != nil {
		t.Fatalf("Nearby 1000: %v", err)
	}
	if want := []string{"北京大学", "北京大学医学部", "清华大学"}; !slices.Equal(nearbyNames(near), want) {
		t.Fatalf("Nearby 1000 = %v, want %v", nearbyNames(near), want)
	}
	if abs(near[2].DistanceM-999) > 1 {
		t.Errorf("distance to 清华大学 = %d, want about 999", near[2].DistanceM)
	}
}

func TestNearbyFiltersByCategoryAndLimit(t *testing.T) {
	f := setup(t)
	repo := repository.NewPOIRepo(f.pool)

	near, err := repo.Nearby(context.Background(), 116.15, 40.05, 3000, []int16{categoryID(t, f.pool, "park")}, 50)
	if err != nil {
		t.Fatalf("Nearby park: %v", err)
	}
	if want := []string{"测试公园", "100%纯净公园"}; !slices.Equal(nearbyNames(near), want) {
		t.Fatalf("Nearby park = %v, want %v", nearbyNames(near), want)
	}

	near, err = repo.Nearby(context.Background(), 116.15, 40.05, 3000, nil, 1)
	if err != nil {
		t.Fatalf("Nearby limit 1: %v", err)
	}
	if want := []string{"测试公园"}; !slices.Equal(nearbyNames(near), want) {
		t.Fatalf("Nearby limit 1 = %v, want %v", nearbyNames(near), want)
	}
}

func TestQueriesCanUseTheirIndexes(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// 测试数据只有几行，规划器默认会选全表扫描；关掉顺序扫描后才能确认索引确实可用。
	if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
		t.Fatalf("disable seqscan: %v", err)
	}
	for name, tc := range map[string]struct{ query, index string }{
		"nearby": {`SELECT id FROM poi WHERE ST_DWithin(geom::geography, ST_SetSRID(ST_MakePoint(116.05, 40.05), 4326)::geography, 1000)`, "poi_geog_gist"},
		"bbox":   {`SELECT id FROM poi WHERE geom && ST_MakeEnvelope(116.0, 40.0, 116.1, 40.1, 4326)`, "poi_geom_gist"},
		"locate": {`SELECT adcode FROM district WHERE ST_Contains(geom, ST_SetSRID(ST_MakePoint(116.05, 40.05), 4326))`, "district_geom_gist"},
		"search": {`SELECT id FROM poi WHERE name ILIKE '%北京大学%'`, "poi_name_trgm"},
	} {
		rows, err := tx.Query(ctx, "EXPLAIN "+tc.query)
		if err != nil {
			t.Fatalf("explain %s: %v", name, err)
		}
		lines, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatalf("collect plan %s: %v", name, err)
		}
		if plan := strings.Join(lines, "\n"); !strings.Contains(plan, tc.index) {
			t.Errorf("%s query does not use %s:\n%s", name, tc.index, plan)
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
```

Run:

```bash
cd backend && TEST_DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable' \
  go test -count=1 -tags=integration ./internal/repository/
```

Expected: 编译失败，报错 `undefined: repository.NewDistrictRepo` 等。

- [ ] **Step 4: 编写 repository 实现**

创建 `backend/internal/repository/district.go`：

```go
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

type DistrictRepo struct {
	db *pgxpool.Pool
}

func NewDistrictRepo(db *pgxpool.Pool) *DistrictRepo {
	return &DistrictRepo{db: db}
}

const listDistrictsSQL = `
SELECT d.adcode, d.name, d.area_km2::float8,
       round(ST_X(d.center)::numeric, 6)::float8, round(ST_Y(d.center)::numeric, 6)::float8,
       ST_AsGeoJSON(ST_SimplifyPreserveTopology(d.geom, 0.0001), 6),
       count(p.id)
FROM district d
LEFT JOIN poi p ON p.district_id = d.id
GROUP BY d.id
ORDER BY d.adcode`

func (r *DistrictRepo) List(ctx context.Context) ([]model.DistrictFeature, error) {
	rows, err := r.db.Query(ctx, listDistrictsSQL)
	if err != nil {
		return nil, fmt.Errorf("list districts: %w", err)
	}
	districts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.DistrictFeature, error) {
		var d model.DistrictFeature
		var geometry string
		err := row.Scan(&d.Adcode, &d.Name, &d.AreaKm2, &d.Center[0], &d.Center[1], &geometry, &d.POICount)
		d.Geometry = json.RawMessage(geometry)
		return d, err
	})
	if err != nil {
		return nil, fmt.Errorf("list districts: %w", err)
	}
	return districts, nil
}

const getDistrictSQL = `
SELECT id, adcode, name, area_km2::float8,
       round(ST_X(center)::numeric, 6)::float8, round(ST_Y(center)::numeric, 6)::float8
FROM district
WHERE adcode = $1`

const districtCategoryCountsSQL = `
SELECT c.code, c.name, count(p.id)
FROM poi_category c
LEFT JOIN poi p ON p.category_id = c.id AND p.district_id = $1
GROUP BY c.id
ORDER BY c.sort_order`

func (r *DistrictRepo) Get(ctx context.Context, adcode string) (model.DistrictDetail, error) {
	var d model.DistrictDetail
	var id int32
	err := r.db.QueryRow(ctx, getDistrictSQL, adcode).Scan(&id, &d.Adcode, &d.Name, &d.AreaKm2, &d.Center[0], &d.Center[1])
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DistrictDetail{}, ErrNotFound
	}
	if err != nil {
		return model.DistrictDetail{}, fmt.Errorf("get district %s: %w", adcode, err)
	}
	rows, err := r.db.Query(ctx, districtCategoryCountsSQL, id)
	if err != nil {
		return model.DistrictDetail{}, fmt.Errorf("count pois of district %s: %w", adcode, err)
	}
	d.POICounts, err = pgx.CollectRows(rows, pgx.RowToStructByPos[model.CategoryCount])
	if err != nil {
		return model.DistrictDetail{}, fmt.Errorf("count pois of district %s: %w", adcode, err)
	}
	return d, nil
}

const locateDistrictSQL = `
SELECT adcode, name
FROM district
WHERE ST_Contains(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326))
ORDER BY adcode
LIMIT 1`

// Locate 返回包含该点的区；点不在任何区内（包括恰好落在边界线上）时返回 nil, nil。
func (r *DistrictRepo) Locate(ctx context.Context, lng, lat float64) (*model.DistrictRef, error) {
	var ref model.DistrictRef
	err := r.db.QueryRow(ctx, locateDistrictSQL, lng, lat).Scan(&ref.Adcode, &ref.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("locate district: %w", err)
	}
	return &ref, nil
}
```

创建 `backend/internal/repository/category.go`：

```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

type CategoryRepo struct {
	db *pgxpool.Pool
}

func NewCategoryRepo(db *pgxpool.Pool) *CategoryRepo {
	return &CategoryRepo{db: db}
}

func (r *CategoryRepo) List(ctx context.Context) ([]model.Category, error) {
	rows, err := r.db.Query(ctx, `SELECT id, code, name, color FROM poi_category ORDER BY sort_order`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	categories, err := pgx.CollectRows(rows, pgx.RowToStructByPos[model.Category])
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return categories, nil
}
```

创建 `backend/internal/repository/poi.go`：

```go
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/model"
)

type POIRepo struct {
	db *pgxpool.Pool
}

func NewPOIRepo(db *pgxpool.Pool) *POIRepo {
	return &POIRepo{db: db}
}

const poiLocationColumns = `round(ST_X(p.geom)::numeric, 6)::float8, round(ST_Y(p.geom)::numeric, 6)::float8`

const poisInBBoxSQL = `
SELECT p.id, p.name, c.code, ` + poiLocationColumns + `
FROM poi p
JOIN poi_category c ON c.id = p.category_id
WHERE p.geom && ST_MakeEnvelope($1, $2, $3, $4, 4326)
  AND ($5::smallint[] IS NULL OR p.category_id = ANY($5))
ORDER BY p.id
LIMIT $6`

// InBBox 返回落在 box 内的 POI，按 id 排序。categoryIDs 为 nil 表示不按分类过滤。
// 多查 1 条来判断是否截断，比额外执行 count(*) 更省。
func (r *POIRepo) InBBox(ctx context.Context, box model.BBox, categoryIDs []int16, limit int) ([]model.POISummary, bool, error) {
	rows, err := r.db.Query(ctx, poisInBBoxSQL, box.MinLng, box.MinLat, box.MaxLng, box.MaxLat, categoryIDs, limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("pois in bbox: %w", err)
	}
	pois, err := pgx.CollectRows(rows, scanPOISummary)
	if err != nil {
		return nil, false, fmt.Errorf("pois in bbox: %w", err)
	}
	if len(pois) > limit {
		return pois[:limit], true, nil
	}
	return pois, false, nil
}

func scanPOISummary(row pgx.CollectableRow) (model.POISummary, error) {
	var p model.POISummary
	err := row.Scan(&p.ID, &p.Name, &p.Category, &p.Location[0], &p.Location[1])
	return p, err
}

const getPOISQL = `
SELECT p.id, p.name, p.name_en, p.osm_type, p.osm_id, p.address, p.tags,
       c.id, c.code, c.name, c.color,
       d.adcode, d.name,
       ` + poiLocationColumns + `
FROM poi p
JOIN poi_category c ON c.id = p.category_id
LEFT JOIN district d ON d.id = p.district_id
WHERE p.id = $1`

func (r *POIRepo) Get(ctx context.Context, id int64) (model.POIDetail, error) {
	var p model.POIDetail
	var adcode, districtName *string
	err := r.db.QueryRow(ctx, getPOISQL, id).Scan(
		&p.ID, &p.Name, &p.NameEn, &p.OSMType, &p.OSMID, &p.Address, &p.Tags,
		&p.Category.ID, &p.Category.Code, &p.Category.Name, &p.Category.Color,
		&adcode, &districtName,
		&p.Location[0], &p.Location[1],
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.POIDetail{}, ErrNotFound
	}
	if err != nil {
		return model.POIDetail{}, fmt.Errorf("get poi %d: %w", id, err)
	}
	if adcode != nil && districtName != nil {
		p.District = &model.DistrictRef{Adcode: *adcode, Name: *districtName}
	}
	return p, nil
}

const searchPOIsSQL = `
SELECT p.id, p.name, c.code, ` + poiLocationColumns + `, d.name
FROM poi p
JOIN poi_category c ON c.id = p.category_id
LEFT JOIN district d ON d.id = p.district_id
WHERE p.name ILIKE $1::text
ORDER BY (p.name = $2::text) DESC, similarity(p.name, $2::text) DESC, length(p.name), p.id
LIMIT $3`

// Search 按名称模糊搜索，q 中的 % 和 _ 按字面匹配。排序规则：完全匹配优先，然后按相似度从高到低，再按名称从短到长。
func (r *POIRepo) Search(ctx context.Context, q string, limit int) ([]model.SearchHit, error) {
	rows, err := r.db.Query(ctx, searchPOIsSQL, "%"+escapeLike(q)+"%", q, limit)
	if err != nil {
		return nil, fmt.Errorf("search pois: %w", err)
	}
	hits, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.SearchHit, error) {
		var h model.SearchHit
		err := row.Scan(&h.ID, &h.Name, &h.Category, &h.Location[0], &h.Location[1], &h.DistrictName)
		return h, err
	})
	if err != nil {
		return nil, fmt.Errorf("search pois: %w", err)
	}
	return hits, nil
}

const nearbyPOIsSQL = `
WITH origin AS (
    SELECT ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography AS g
)
SELECT p.id, p.name, c.code, ` + poiLocationColumns + `,
       round(ST_Distance(p.geom::geography, o.g))::int AS distance_m
FROM poi p
JOIN poi_category c ON c.id = p.category_id
CROSS JOIN origin o
WHERE ST_DWithin(p.geom::geography, o.g, $3)
  AND ($4::smallint[] IS NULL OR p.category_id = ANY($4))
ORDER BY distance_m, p.id
LIMIT $5`

// Nearby 返回距 (lng, lat) radiusM 米以内的 POI，按距离从近到远排序。
// 过滤条件必须写成 p.geom::geography，与表达式索引 poi_geog_gist 一致，否则用不上索引。
func (r *POIRepo) Nearby(ctx context.Context, lng, lat, radiusM float64, categoryIDs []int16, limit int) ([]model.NearbyPOI, error) {
	rows, err := r.db.Query(ctx, nearbyPOIsSQL, lng, lat, radiusM, categoryIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("nearby pois: %w", err)
	}
	pois, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.NearbyPOI, error) {
		var p model.NearbyPOI
		err := row.Scan(&p.ID, &p.Name, &p.Category, &p.Location[0], &p.Location[1], &p.DistanceM)
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("nearby pois: %w", err)
	}
	return pois, nil
}
```

- [ ] **Step 5: 运行全部测试，确认通过**

Run:

```bash
cd backend && go test ./... && go vet -tags=integration ./... && test -z "$("$(go env GOROOT)/bin/gofmt" -l .)"
TEST_DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable' go test -count=1 -tags=integration ./internal/repository/ -v
```

Expected: 单元测试全部 PASS；repository 的 18 个集成测试和 `TestEscapeLike` 全部 PASS。

- [ ] **Step 6: 提交**

```bash
git add backend/internal/model backend/internal/repository
git commit -m "feat(backend): add repository layer for districts, categories and POI spatial queries"
```

---

### Task 8: Compose 中的 seed 服务、Makefile、环境变量说明与文档

**Files:**
- Modify: `docker-compose.yml`（整体替换）
- Modify: `Makefile`（整体替换）
- Modify: `.env.example`（整体替换）
- Modify: `README.md`
- Modify: `docs/02-technical-design.md`
- Modify: `docs/03-database-design.md`

**Interfaces:**
- Consumes: 镜像中的 `importer` 可执行文件（Task 6）、`scripts/fetch_osm_pois.sh`（Task 5）、`TEST_DATABASE_URL` 约定（Task 3）
- Produces:
  - compose 服务 `seed`：与 `backend` 使用同一份构建配置，执行 `importer`，`./data/raw` 以只读方式挂载到 `/data`；`backend` 在 `seed` 成功结束后才启动
  - Make 目标：`up`、`down`、`logs`、`dev-db`、`seed`、`run-backend`、`run-frontend`、`test`、`test-backend`、`test-frontend`、`test-integration`、`lint`、`fetch-osm`；`Makefile` 会读取 `.env`

- [ ] **Step 1: 替换 docker-compose.yml**

把 `docker-compose.yml` 整体替换为：

```yaml
name: beijing-geo-map

x-backend-build: &backend-build
  context: ./backend
  args:
    GOPROXY: ${GOPROXY:-https://goproxy.cn,direct}

x-database-url: &database-url "postgres://${POSTGRES_USER:-geo}:${POSTGRES_PASSWORD:-geo}@db:5432/${POSTGRES_DB:-geo}?sslmode=disable"

services:
  db:
    image: postgis/postgis:16-3.4
    platform: linux/amd64
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-geo}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-geo}
      POSTGRES_DB: ${POSTGRES_DB:-geo}
    ports:
      - "127.0.0.1:${DB_PORT:-5432}:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      # 必须走 TCP：初始化阶段的临时实例只监听 unix socket，这样可以避免初始化尚未完成就被判定为健康。
      test: ["CMD-SHELL", "pg_isready -h 127.0.0.1 -U $${POSTGRES_USER} -d $${POSTGRES_DB}"]
      interval: 5s
      timeout: 5s
      retries: 30

  seed:
    build: *backend-build
    command: ["importer"]
    environment:
      DATABASE_URL: *database-url
      DATA_DIR: /data
      LOG_LEVEL: ${LOG_LEVEL:-info}
    volumes:
      - ./data/raw:/data:ro
    depends_on:
      db:
        condition: service_healthy

  backend:
    build: *backend-build
    environment:
      DATABASE_URL: *database-url
      HTTP_ADDR: ":8000"
      LOG_LEVEL: ${LOG_LEVEL:-info}
    depends_on:
      db:
        condition: service_healthy
      seed:
        condition: service_completed_successfully
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

Run: `docker compose config --quiet && docker compose config | grep -E 'DATABASE_URL|DATA_DIR|/data'`
Expected: 配置校验通过；`seed` 和 `backend` 的 `DATABASE_URL` 都是 `postgres://geo:geo@db:5432/geo?sslmode=disable`，`seed` 有 `DATA_DIR: /data` 以及指向 `data/raw` 的只读挂载。

- [ ] **Step 2: 替换 Makefile**

把 `Makefile` 整体替换为（命令行必须以 Tab 缩进）：

```makefile
.PHONY: up down logs dev-db seed run-backend run-frontend test test-backend test-frontend test-integration lint fetch-osm

-include .env

POSTGRES_USER ?= geo
POSTGRES_PASSWORD ?= geo
POSTGRES_DB ?= geo
DB_PORT ?= 5432
DEV_DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(DB_PORT)/$(POSTGRES_DB)?sslmode=disable

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

dev-db:
	docker compose up -d --wait db
	docker compose run --rm --build seed

seed:
	docker compose run --rm --build seed

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
	cd backend && TEST_DATABASE_URL="$(DEV_DATABASE_URL)" go test -count=1 -tags=integration ./...

lint:
	cd backend && unformatted="$$("$$(go env GOROOT)/bin/gofmt" -l .)" && if [ -n "$$unformatted" ]; then echo "以下文件需要 gofmt：" >&2; echo "$$unformatted" >&2; exit 1; fi && go vet -tags=integration ./...
	cd frontend && npm run lint && npm run typecheck

fetch-osm:
	./scripts/fetch_osm_pois.sh
```

Run: `grep -nP '^ +\S' Makefile || echo "tabs ok"; make -n dev-db test-integration lint`
Expected: 输出 `tabs ok`；`make -n` 打印出的命令中，`TEST_DATABASE_URL` 为 `postgres://geo:geo@localhost:5432/geo?sslmode=disable`。

- [ ] **Step 3: 替换 .env.example**

把 `.env.example` 整体替换为：

```text
# 复制为 .env 后按需修改；没有 .env 时 docker compose 和 Makefile 都使用下面这些默认值。
#
# 注意：
# 1. POSTGRES_* 只在数据卷第一次初始化时生效。数据库已经初始化后再修改，需要先执行
#    docker compose down -v 删除数据卷（会清空已导入的数据），再重新启动，seed 服务会重新导入。
# 2. 密码会原样拼进数据库连接串，不要使用 @ : / ? # % 等字符。
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

- [ ] **Step 4: 更新 README**

把状态行：

```markdown
> **当前状态：开发中。** 项目骨架已经可以一键启动，地图功能按阶段推进中，进度见[实现文档](docs/01-implementation-plan.md#5-阶段计划)。
```

替换为：

```markdown
> **当前状态：开发中。** 数据层已经完成：启动时会自动导入北京 16 个区的边界和约 4100 个 POI。地图功能按阶段推进中，进度见[实现文档](docs/01-implementation-plan.md#5-阶段计划)。
```

在以 `启动后访问 <http://localhost:8080>。` 开头的那一段之后，插入一个新段落：

```markdown
首次启动时，`seed` 服务会先执行数据库迁移，再导入 `data/raw/` 下的数据快照，完成后后端才会启动。之后每次 `docker compose up` 都会重新导入一遍，导入是幂等的，不会产生重复数据。
```

把开发命令代码块：

```bash
make dev-db         # 只启动数据库
make run-backend    # 启动后端，监听 localhost:8000
make run-frontend   # 启动前端开发服务器，/api 会被代理到 localhost:8000
make test           # 运行前后端单元测试
```

替换为：

```bash
make dev-db           # 启动数据库，并通过 seed 服务完成迁移和数据导入
make run-backend      # 启动后端，监听 localhost:8000
make run-frontend     # 启动前端开发服务器，/api 会被代理到 localhost:8000
make test             # 运行前后端单元测试
make test-integration # 运行数据库集成测试（需要先 make dev-db），测试使用独立的 geo_test_* 数据库
```

并在这个代码块之后插入：

```markdown
`Makefile` 会读取 `.env`，所以在 `.env` 里修改 `DB_PORT` 或数据库账号后，本地开发命令也会使用新的设置。数据快照的来源、许可和更新方法见 [data/README.md](data/README.md)。
```

- [ ] **Step 5: 更新技术方案**

在 `docs/02-technical-design.md` 的环境变量表中，在 `DATA_DIR` 这一行之后插入：

```markdown
| `TEST_DATABASE_URL` | 集成测试 | 无（`make test-integration` 会自动设置） | 管理连接串。测试会在同一个服务器上创建并使用 `geo_test_<包名>` 数据库，不会改动开发库 |
```

把 3.3 节中从 `开发时不需要每次都构建镜像：` 到 `其他常用命令：…… / make fetch-osm`。` 为止的内容替换为：

````markdown
开发时不需要每次都构建镜像：

```bash
make dev-db          # 启动 db 容器并等待健康检查通过，再运行一次 seed 服务完成迁移和导入
make run-backend     # go run ./cmd/server，连接本机的数据库端口
make run-frontend    # npm run dev，由 Vite 把 /api 代理到 localhost:8000
```

其他常用命令：`make up` / `make down` / `make logs` / `make seed`（重新导入） / `make test` / `make test-integration` / `make lint` / `make fetch-osm`（更新 POI 快照）。

`Makefile` 会读取 `.env`，本地连接串由 `POSTGRES_USER`、`POSTGRES_PASSWORD`、`POSTGRES_DB`、`DB_PORT` 拼出。`make test-integration` 把这个连接串作为 `TEST_DATABASE_URL` 传给测试，每个测试包在同一个服务器上创建并使用自己的 `geo_test_<包名>` 数据库，不会改动开发库里已导入的数据。
````

把 5.5 节中的这一行：

```markdown
   4. **重算 `district_id`**：用一条空间关联 `UPDATE` 完成，然后把已经不在任何区内的 POI 置为 `NULL`。
```

替换为：

```markdown
   4. **重算 `district_id`**：用一条 `UPDATE` 同时完成分配和清空：每个 POI 取包含它的区（多个区都包含时取 `adcode` 最小的，保证重复导入的结果稳定），不在任何区内的置为 `NULL`。
```

把 7.1 节中的这一句：

```markdown
集成测试使用一份人造数据：两个相邻的正方形「区」，以及十几个分布在区内、区外和边界附近的点。覆盖以下场景：
```

替换为：

```markdown
集成测试带 `integration` 构建标签，通过 `internal/testdb` 为每个测试包创建独立的 `geo_test_<包名>` 数据库。repository 和导入器的测试各自使用一份人造数据：两个相邻的正方形「区」，以及若干分布在区内和区外的点。导入器的测试还会导入真实快照，校验区的数量、POI 数量和所属区匹配率。覆盖以下场景：
```

- [ ] **Step 6: 更新数据库设计**

在 `docs/03-database-design.md` 的 7.4 节，把整个 SQL 代码块（以 `-- 1. 为每个 POI 找到包含它的区` 开头、以第二条 `UPDATE` 的 `);` 结尾）替换为：

````markdown
```sql
-- 一条语句同时完成分配和清空：不在任何区内的 POI 得到 NULL。
-- 多个区都包含同一个点时取 adcode 最小的区，保证重复导入的结果稳定。
UPDATE poi p
SET district_id = m.district_id, updated_at = now()
FROM (
    SELECT p2.id AS poi_id,
           (SELECT d.id FROM district d WHERE ST_Contains(d.geom, p2.geom) ORDER BY d.adcode LIMIT 1) AS district_id
    FROM poi p2
) m
WHERE m.poi_id = p.id
  AND p.district_id IS DISTINCT FROM m.district_id;
```
````

- [ ] **Step 7: 从零开始的端到端验证**

使用一个独立的 compose 项目名，从空数据卷开始验证，不影响开发用的数据库：

```bash
export DB_PORT=15432 FRONTEND_PORT=18080
docker compose -p bgm-e2e up --build -d
for i in $(seq 1 90); do curl -fsS localhost:18080/api/v1/health && break; sleep 5; done; echo
docker compose -p bgm-e2e ps -a --format '{{.Service}} {{.State}} {{.ExitCode}}'
docker compose -p bgm-e2e logs seed | grep '"msg":"import finished"'
docker compose -p bgm-e2e exec -T db psql -U geo -d geo -tA \
  -c "SELECT count(*) FROM district" \
  -c "SELECT count(*) FROM poi" \
  -c "SELECT round(100.0 * count(district_id) / count(*), 1) FROM poi"
docker compose -p bgm-e2e logs backend | grep -c '"path":"/api/v1/health"' || true
docker compose -p bgm-e2e down -v --rmi local
unset DB_PORT FRONTEND_PORT
```

Expected:
- 健康检查循环最终输出 `{"data":{"db":"ok","status":"ok"}}`。
- `ps -a` 中 `seed` 为 `exited 0`，`db`、`backend`、`frontend` 为 `running`。
- `seed` 日志中有 `import finished`，包含 `"districts":16`。
- psql 依次输出 `16`、约 `4110`、约 `99.6`。
- 最后一个 grep 输出 `0`：成功的健康检查已经降为 Debug 级别，不再出现在访问日志里。
- `down -v --rmi local` 只删除 `bgm-e2e` 项目自己的容器、数据卷和它构建的镜像，开发用的 `beijing-geo-map` 项目不受影响。

- [ ] **Step 8: 运行本地检查**

```bash
docker compose up -d --wait db
make test
make lint
make test-integration
```

Expected: 单元测试全部 PASS；`make lint` 没有错误；`make test-integration` 中迁移、导入器、repository 三个包的集成测试全部 PASS。

- [ ] **Step 9: 提交**

```bash
git add docker-compose.yml Makefile .env.example README.md docs/02-technical-design.md docs/03-database-design.md
git commit -m "feat: add seed service, Makefile data targets and docs for the data layer"
```

---

## 阶段完成后（由协调者执行，不属于任何任务）

1. 推送分支：`git push -u origin phase-1-data-layer`。
2. 创建 PR：`gh pr create --base main --head phase-1-data-layer`，标题为 `阶段 1：数据层（迁移、坐标转换、数据快照、导入器、repository）`，正文写明验收标准和验证证据。
3. 等待 CI：`gh pr checks --watch`，确认 `backend`、`frontend`、`docker` 三个任务全部通过。
4. 由用户审查并合并 PR。用户合并之前，不开始阶段 2。
