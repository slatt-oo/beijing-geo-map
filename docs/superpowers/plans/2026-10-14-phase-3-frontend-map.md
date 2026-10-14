# 阶段 3：前端地图 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在浏览器中渲染北京地图：高德底图、16 个区的边界（悬停高亮、点击缩放和区详情面板）、按视野加载的 POI 图层（聚合、分类筛选、详情弹窗），区边界和 POI 与底图对齐。同时修复阶段 2 遗留的小 GeoJSON 响应被错误标记为 gzip 的问题，并在导入时丢弃北京范围以外的 POI。

**Architecture:** 直接使用 maplibre-gl 6。`MapView` 创建地图，在 `load` 事件里一次建好所有数据源和图层，再通过 React Context 把地图实例交给各图层组件；图层组件用 TanStack Query 取数，经 `toDisplay()` 转换后调用 `setData`。交互状态放在 Zustand，地图事件回调只注册一次，通过 `getState()` 读取最新状态。可以单测的逻辑（坐标转换、GeoJSON 转换、视野 bbox、分类参数与颜色表达式、瓦片地址、格式化、store）都放在纯函数里，用 Vitest 覆盖；依赖 WebGL 的图层组件靠 lint、类型检查、构建和阶段结束时的浏览器验收来保证。

**Tech Stack:** React 19.3、TypeScript ~6.0.3、Vite 8.3、Vitest 5、Tailwind CSS 4.3；新增 maplibre-gl 6.14、@tanstack/react-query 5.104、zustand 5.0，以及开发依赖 @types/geojson。后端 Go 1.26、Gin v1.12、gin-contrib/gzip v1.2.8，不新增依赖。

**Spec:** `docs/01-implementation-plan.md`（第 5 节「阶段 3」）、`docs/02-technical-design.md`（第 4 节、第 6 节、第 10 节第 1 到 5 项）

## Global Constraints

- 所有工作在 `phase-3-frontend-map` 分支上进行，每个任务结束时提交一次。git 提交身份已经配置好，不要修改；环境会在提交信息末尾自动加一行 `Co-authored-by: Cursor`。
- 前端依赖只新增这四个：`maplibre-gl@^6.14.0`、`@tanstack/react-query@^5.104.2`、`zustand@^5.0.15`，开发依赖 `@types/geojson@^7946.0.16`。不引入 React 地图封装库、turf、jsdom 或 Testing Library。TypeScript 保持 `~6.0.3`，不要升级到 7（typescript-eslint 8 只支持到 6.0）。
- MapLibre 6 的用法：
  - 包里没有默认导出，只能用命名导入，例如 `import { Map as MaplibreMap, Popup } from 'maplibre-gl'`，类型用 `import type`。
  - Web Worker 必须通过 `import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url'` 和 `setWorkerUrl(workerUrl)` 指定，否则打包后地图上的 GeoJSON 图层全部不显示。
  - 样式中不配置 `glyphs`：MapLibre 6 在没有字形地址时用浏览器本地字体绘制文字。
- 坐标系：接口数据、store、查询参数一律是 WGS-84。只能通过 `src/config.ts` 导出的 `toDisplay()` / `toData()` 转换：交给地图数据源的 GeoJSON 一律经过 `transformCollection(…, toDisplay)`；从地图上取到的坐标（视野范围、点击位置）一律经过 `toData()`。
- 前端检查命令：`cd frontend && npm run lint && npm run typecheck && npm test && npm run build`，输出中不能有错误或警告。Vitest 运行在 node 环境，只测纯函数和 store。
- 界面文案和错误提示使用中文。代码注释只用来说明代码本身体现不出来的约束。
- 后端：Go 模块路径 `github.com/slatt-oo/beijing-geo-map/backend`。后端测试只用标准库 `testing`；集成测试文件名为 `*_integration_test.go`，首行 `//go:build integration`，管理连接为 `TEST_DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable'`，测试不能写开发库 `geo`。计划中的 Go 代码如与 gofmt 格式有出入，以 `gofmt -w` 的结果为准；提交前 `gofmt -l` 必须没有输出。
- 本地环境：compose 的 `db` 服务在运行（`127.0.0.1:5432`，已导入完整数据）。本机 Docker 是 Colima，没有 buildx（回退到经典构建器属于正常现象）。8080 端口被占用，启动整套服务时使用 `FRONTEND_PORT=18080`；`docker compose up --build` 可能需要几分钟。验证完执行 `docker compose stop frontend backend`，保留 db。绝不执行 `docker compose down -v`。
- **沙箱说明：** go、docker、curl 以及 git commit 命令都需要 Shell `required_permissions: ["all"]`；`npm install` 需要访问 npm 源，被拦截时用 `required_permissions: ["full_network"]`。临时文件放在仓库内被 git 忽略的 `.superpowers/` 目录下，不要放 `/tmp`。多行 shell 脚本用 bash 运行。

### 本计划的决定

以下内容在规格中没有写明，或与规格的写法不同，由本计划决定：

1. **修复阶段 2 的 gzip 问题。** gin v1.12 的 `c.Data()` 会先设置 `Content-Length`；gin-contrib/gzip 遇到小于 1 KB 的 `Content-Length` 时直接写出未压缩的内容，却不删除已经加上的 `Content-Encoding: gzip`。所以小于 1 KB 的 GeoJSON 响应（小视野的 `/pois`、结果少的 `/pois/nearby`）在浏览器里解码失败。`writeGeoJSON` 改为不经过 `c.Data()`。
2. **导入时丢弃北京范围以外的 POI。** 范围是北京市边界外扩约 1°（经度 114.4 到 118.5，纬度 38.4 到 42.1）。当前快照中只有整条长城的 relation（r318110，中心点在甘肃，约 795 千米外）落在范围外；清河农场飞地（北京管辖、位于天津境内）的两个 POI 仍在范围内，会保留。前端地图的 `maxBounds` 使用同一个范围。
3. **`/pois` 固定以 `limit=5000` 请求。** 当前约 4100 个 POI，全市视野也不会被截断，聚合数字才准确；截断提示保留，供数据增长后使用。
4. **store 记录关闭的分类（`hiddenCategories`），而不是规格写的 `enabledCategories`。** 分类列表加载完成之前，空数组也是确定的状态（全部开启）。
5. **阶段 4 才用到的 store 字段（`myLocation`、`nearby`、`mode`）本阶段不加。**
6. **POI 详情弹窗不显示地址。** 地址由 OSM 的 `addr:*` 标签拼接而成，质量参差不齐（重复的「北京市北京市」、零散片段）。
7. **地图位置写入 URL hash**（MapLibre 的 `hash: true`），刷新和分享链接时回到同一视野，验收截图时也可以直接跳到指定位置。
8. **错误提示条放在右上角、分类开关上方**，由 `QueryCache` 的 `onError` 统一写入 store。
9. **点击由一个处理函数统一分发**：先查 POI 图层，再查区图层；点到 POI 以外的地方会关闭 POI 弹窗。
10. **验收截图存放在 `docs/screenshots/phase-3/`**，由协调者在全部任务完成后用浏览器逐项验证并提交。
11. **客户端取消的请求不再记为错误。** 前端移动地图后，TanStack Query 会取消上一次查询，后端随之收到 `context.Canceled`。原来这会被记为 ERROR 日志并按 500 处理；现在在请求上下文确实已被取消时，按 nginx 的约定记为 499，不写错误日志（客户端已经断开，收不到这个响应）。

---

### Task 1: 后端修正：小 GeoJSON 响应的 gzip 标记、客户端取消的请求、导入时的范围过滤

**Files:**
- Modify: `backend/internal/handler/response.go`（`writeGeoJSON`、`writeServiceError`）
- Modify: `backend/internal/handler/router_test.go`（新增两个测试）
- Modify: `backend/internal/handler/poi_test.go`（新增两个测试）
- Create: `backend/internal/importer/region.go`
- Create: `backend/internal/importer/region_test.go`
- Modify: `backend/internal/importer/importer.go`（`Stats`、`Run`）
- Modify: `backend/internal/importer/testdata/beijing_pois_osm.json`（新增一个范围外的要素）
- Modify: `backend/internal/importer/importer_integration_test.go`
- Modify: `backend/cmd/importer/main.go`（日志字段）
- Modify: `docs/02-technical-design.md`（5.5 节）

**Interfaces:**
- Consumes: `handler` 包内部测试的 `get`、`decodeJSON`、`fakePinger`、`captureLogs`、`assertErrorResponse`（`router_test.go`），`fakeDistrictService`、`fakePOIService`、`newTestRouter`（`fakes_test.go`）；`importer.POI`（`osm.go`）
- Produces: `importer.Stats.SkippedOutsideRegion int`；导入范围 `beijingRegion`（经度 114.4 到 118.5，纬度 38.4 到 42.1），前端 Task 3 的 `REGION_BBOX` 与它一致；客户端取消的请求记为状态码 499

- [ ] **Step 1: 编写 gzip 回归测试**

在 `backend/internal/handler/router_test.go` 中，紧接着 `TestSmallResponsesAreNotGzipped` 之后加入（文件已经导入了 `compress/gzip`、`io`、`net/http`、`strings` 和 `gin`）：

```go
func TestSmallGeoJSONResponsesAreNotMarkedAsGzip(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{}})
	r.GET("/api/v1/test-geojson", func(c *gin.Context) {
		writeGeoJSON(c, gin.H{"type": "FeatureCollection", "features": []any{}})
	})

	// 必须看 Result() 的响应头：它是写出那一刻的快照；rec.Header() 会反映中间件事后的删除，测不出这个问题。
	res := get(t, r, "/api/v1/test-geojson", http.Header{"Accept-Encoding": {"gzip"}}).Result()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if got := res.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q for a %d-byte body, want none", got, len(body))
	}
	if got := res.Header.Get("Content-Type"); got != geoJSONContentType {
		t.Fatalf("Content-Type = %q, want %q", got, geoJSONContentType)
	}
	if string(body) != `{"features":[],"type":"FeatureCollection"}` {
		t.Fatalf("body = %s", body)
	}
}

func TestLargeGeoJSONResponsesAreGzipped(t *testing.T) {
	r := NewRouter(Deps{DB: fakePinger{}})
	name := strings.Repeat("北京", 1000)
	r.GET("/api/v1/test-geojson", func(c *gin.Context) {
		writeGeoJSON(c, gin.H{"type": "FeatureCollection", "name": name})
	})

	res := get(t, r, "/api/v1/test-geojson", http.Header{"Accept-Encoding": {"gzip"}}).Result()

	if got := res.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := res.Header.Get("Content-Type"); got != geoJSONContentType {
		t.Fatalf("Content-Type = %q, want %q", got, geoJSONContentType)
	}
	zr, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatalf("body is not gzip: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if got := decodeJSON(t, body)["name"]; got != name {
		t.Fatalf("decompressed name mismatch (got %d bytes)", len(body))
	}
}
```

- [ ] **Step 2: 运行测试，确认小响应的测试失败**

Run: `cd backend && go test -count=1 ./internal/handler/ -run 'GeoJSONResponses' -v`
Expected: `TestSmallGeoJSONResponsesAreNotMarkedAsGzip` FAIL，提示 `Content-Encoding = "gzip" for a 42-byte body, want none`；`TestLargeGeoJSONResponsesAreGzipped` PASS（它保证修复后大响应仍然压缩）。

- [ ] **Step 3: 修改 `writeGeoJSON`**

把 `backend/internal/handler/response.go` 中的 `writeGeoJSON` 整个替换为：

```go
// writeGeoJSON 不用 c.Data：c.Data 会先设置 Content-Length，gin-contrib/gzip 看到小于 1 KB 的 Content-Length 时
// 直接写出未压缩的内容，却不删除已经加上的 Content-Encoding: gzip，浏览器解码就会失败。
func writeGeoJSON(c *gin.Context, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "encode geojson response", "err", err)
		writeInternal(c)
		return
	}
	c.Header("Content-Type", geoJSONContentType)
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(body)
}
```

- [ ] **Step 4: 运行 handler 包的全部单元测试**

Run: `cd backend && go test -count=1 ./internal/handler/`
Expected: PASS（包括两个新测试，以及已有的 GeoJSON 接口测试）。

- [ ] **Step 5: 编写客户端取消请求的测试**

在 `backend/internal/handler/poi_test.go` 的导入中加入 `"context"` 和 `"net/http/httptest"`，并在文件末尾追加：

```go
func TestClientCanceledRequestsAreNotLoggedAsErrors(t *testing.T) {
	logs := captureLogs(t)
	r := newTestRouter(&fakeDistrictService{}, &fakePOIService{err: context.Canceled})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/pois/42", nil).WithContext(ctx))

	if rec.Code != statusClientClosedRequest {
		t.Fatalf("status = %d, want %d", rec.Code, statusClientClosedRequest)
	}
	if strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Fatalf("a client cancellation must not be logged as an error:\n%s", logs)
	}
}

func TestCanceledErrorsWithoutClientCancellationStayInternal(t *testing.T) {
	logs := captureLogs(t)

	rec := get(t, newTestRouter(&fakeDistrictService{}, &fakePOIService{err: context.Canceled}), "/api/v1/pois/42", nil)

	assertErrorResponse(t, rec, http.StatusInternalServerError, CodeInternal, "服务器内部错误")
	if !strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Fatalf("a cancellation the client did not cause must be logged as an error:\n%s", logs)
	}
}
```

Run: `cd backend && go test -count=1 ./internal/handler/ -run 'Cancel' -v`
Expected: 编译失败，提示 `undefined: statusClientClosedRequest`。

- [ ] **Step 6: 在 `writeServiceError` 中区分客户端取消**

在 `backend/internal/handler/response.go` 的导入中加入 `"context"`。在 `const geoJSONContentType = "application/geo+json"` 这一行之前插入：

```go
// statusClientClosedRequest 沿用 nginx 的约定，表示客户端在响应之前断开了连接（前端移动地图后会取消上一次查询）。
// 这个状态码只出现在访问日志里，客户端收不到。
const statusClientClosedRequest = 499

```

在 `writeServiceError` 的 `switch` 中，紧接着 `NotFound` 那个分支（`writeError(c, http.StatusNotFound, CodeNotFound, notFoundMessage)`）之后、`default:` 之前加入：

```go
	case errors.Is(err, context.Canceled) && c.Request.Context().Err() != nil:
		c.AbortWithStatus(statusClientClosedRequest)
```

Run: `cd backend && go test -count=1 ./internal/handler/`
Expected: PASS

- [ ] **Step 7: 编写导入范围的单元测试**

创建 `backend/internal/importer/region_test.go`：

```go
package importer

import "testing"

func TestWithinRegionDropsPOIsOutsideTheRegion(t *testing.T) {
	pois := []POI{
		{OSMType: "n", OSMID: 1, Lng: 116.391, Lat: 39.906},     // 天安门
		{OSMType: "r", OSMID: 318110, Lng: 107.925, Lat: 35.722}, // 整条长城的 relation，中心点在甘肃
		{OSMType: "w", OSMID: 2, Lng: 117.665, Lat: 39.227},      // 清河农场飞地：北京管辖，位于天津境内
		{OSMType: "n", OSMID: 3, Lng: 114.4, Lat: 42.1},          // 范围的角点也算在内
	}

	kept, skipped := withinRegion(pois, beijingRegion)

	if skipped != 1 || len(kept) != 3 || kept[0].OSMID != 1 || kept[1].OSMID != 2 || kept[2].OSMID != 3 {
		t.Fatalf("kept %+v, skipped %d", kept, skipped)
	}
}
```

- [ ] **Step 8: 运行测试，确认失败**

Run: `cd backend && go test -count=1 ./internal/importer/ -run TestWithinRegion`
Expected: 编译失败，提示 `undefined: withinRegion` 和 `undefined: beijingRegion`。

- [ ] **Step 9: 实现导入范围**

创建 `backend/internal/importer/region.go`：

```go
package importer

type region struct {
	MinLng, MinLat, MaxLng, MaxLat float64
}

// beijingRegion 是北京市边界外扩约 1° 的范围（WGS-84）。OSM 中少数要素的中心点远在北京之外
// （例如整条长城的 relation，中心点在甘肃），这些 POI 不导入。前端 src/config.ts 的 REGION_BBOX 必须与它一致。
var beijingRegion = region{MinLng: 114.4, MinLat: 38.4, MaxLng: 118.5, MaxLat: 42.1}

func (r region) contains(lng, lat float64) bool {
	return lng >= r.MinLng && lng <= r.MaxLng && lat >= r.MinLat && lat <= r.MaxLat
}

// withinRegion 按原顺序返回落在 r 内的 POI，以及被丢弃的数量。
func withinRegion(pois []POI, r region) ([]POI, int) {
	kept := make([]POI, 0, len(pois))
	for _, p := range pois {
		if r.contains(p.Lng, p.Lat) {
			kept = append(kept, p)
		}
	}
	return kept, len(pois) - len(kept)
}
```

Run: `cd backend && go test -count=1 ./internal/importer/ -run TestWithinRegion`
Expected: PASS

- [ ] **Step 10: 在测试快照中加入一个范围外的要素，并更新集成测试的期望**

在 `backend/internal/importer/testdata/beijing_pois_osm.json` 中，紧接着 `"id":1004` 那一行之后插入一行：

```json
  {"type":"relation","id":3002,"center":{"lat":35.72,"lon":107.93},"tags":{"tourism":"attraction","name":"远处长城"}},
```

在 `backend/internal/importer/importer_integration_test.go` 的 `TestRunLoadsFixtureSnapshot` 中，把期望的统计信息改为：

```go
	want := importer.Stats{
		Districts:            2,
		POIs:                 6,
		POIsWithDistrict:     5,
		POIsByCategory:       map[string]int{"subway_station": 1, "university": 1, "attraction": 2, "hospital": 1, "park": 1},
		SkippedOutsideRegion: 1,
	}
```

在同一文件的 `TestRunImportsRealSnapshot` 中，紧接着检查各分类数量的 `for` 循环之后加入：

```go
	var greatWall int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM poi WHERE osm_type = 'r' AND osm_id = 318110`).Scan(&greatWall); err != nil {
		t.Fatalf("count r318110: %v", err)
	}
	if stats.SkippedOutsideRegion == 0 || greatWall != 0 {
		t.Errorf("skipped = %d, r318110 stored = %d: the Great Wall relation centred about 800 km away must be skipped",
			stats.SkippedOutsideRegion, greatWall)
	}
```

`TestRunRejectsSnapshotBelowMinimumWithoutTouchingData` 不用改，但它现在还会检验「最少数量按过滤后的 POI 计算」：去掉 w2002 后快照解析出 6 个 POI，其中 1 个在范围外，过滤后只剩 5 个，低于 `MinPOIs: 6`，应当被拒绝。

- [ ] **Step 11: 运行集成测试，确认失败**

Run: `cd backend && TEST_DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable' go test -count=1 -tags=integration ./internal/importer/`
Expected: 编译失败，提示 `unknown field SkippedOutsideRegion in struct literal`。

- [ ] **Step 12: 在 `Run` 中应用范围过滤**

在 `backend/internal/importer/importer.go` 中，把 `Stats` 改为：

```go
type Stats struct {
	Districts            int
	POIs                 int
	POIsWithDistrict     int
	POIsByCategory       map[string]int
	DeletedPOIs          int64
	SkippedOutsideRegion int
}
```

把 `Run` 中读取 POI 快照之后、开启事务之前的这一段：

```go
	pois, err := readPOIs(filepath.Join(opts.DataDir, POIsFile))
	if err != nil {
		return Stats{}, err
	}
	if len(pois) < opts.MinPOIs {
		return Stats{}, fmt.Errorf("snapshot has %d POIs, fewer than the minimum %d; refusing to import", len(pois), opts.MinPOIs)
	}
```

替换为：

```go
	pois, err := readPOIs(filepath.Join(opts.DataDir, POIsFile))
	if err != nil {
		return Stats{}, err
	}
	pois, skipped := withinRegion(pois, beijingRegion)
	if len(pois) < opts.MinPOIs {
		return Stats{}, fmt.Errorf("snapshot has %d POIs inside the Beijing region, fewer than the minimum %d; refusing to import", len(pois), opts.MinPOIs)
	}
```

再把末尾的：

```go
	stats.DeletedPOIs = deleted
```

替换为：

```go
	stats.DeletedPOIs = deleted
	stats.SkippedOutsideRegion = skipped
```

过滤后的 `pois` 同时用于 upsert 和同步删除，所以数据库里已有的范围外 POI 会在下一次导入时被同步删除。

在 `backend/cmd/importer/main.go` 的 `slog.Info("import finished", ...)` 中，紧接着 `"deleted_pois", stats.DeletedPOIs,` 加一行：

```go
		"skipped_outside_region", stats.SkippedOutsideRegion,
```

- [ ] **Step 13: 运行集成测试，确认通过**

Run: `cd backend && TEST_DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable' go test -count=1 -tags=integration ./internal/importer/ ./internal/handler/`
Expected: PASS（handler 包的端到端测试也使用这份测试快照，范围外的要素被丢弃，期望不变）。

- [ ] **Step 14: 更新技术方案 5.3 节和 5.5 节**

在 `docs/02-technical-design.md` 中做四处替换。

5.3 节错误码表格的最后一行，把：

```text
| `UNAVAILABLE` | 503 | 健康检查时数据库不可用 |
```

替换为：

```text
| `UNAVAILABLE` | 503 | 健康检查时数据库不可用 |

客户端在响应之前断开连接时（例如前端移动地图后取消了上一次查询），访问日志中的状态码记为 499（沿用 nginx 的约定），不写错误日志；客户端本身收不到这个响应。
```

5.5 节，把：

```text
   2. **导入 POI**：解析 Overpass 返回的 JSON。节点取 `lat`/`lon`，线和关系取 `center`。然后依次进行分类映射、名称选择和地铁站去重，最后按 `(osm_type, osm_id)` upsert。
```

替换为：

```text
   2. **导入 POI**：解析 Overpass 返回的 JSON。节点取 `lat`/`lon`，线和关系取 `center`。然后依次进行分类映射、名称选择和地铁站去重，再丢弃坐标落在北京市边界外扩约 1° 范围（经度 114.4 到 118.5，纬度 38.4 到 42.1）以外的 POI，最后按 `(osm_type, osm_id)` upsert。OSM 中少数要素的中心点远在北京之外，例如整条长城的 relation，中心点在甘肃；清河农场飞地（北京管辖、位于天津境内）的 POI 仍在范围内，会保留。前端地图的可视范围与这个范围相同。
```

把：

```text
快照解析出的 POI 少于 1000 条时，导入器直接报错退出，防止因为快照损坏而误删全部数据。
```

替换为：

```text
范围过滤后的 POI 少于 1000 条时，导入器直接报错退出，防止因为快照损坏而误删全部数据。
```

把：

```text
3. 输出统计信息：各表行数、各分类数量、`district_id` 填充率。
```

替换为：

```text
3. 输出统计信息：各表行数、各分类数量、`district_id` 填充率、同步删除的数量，以及因落在范围外而丢弃的数量。
```

- [ ] **Step 15: 全量检查并提交**

```bash
cd backend
go test -count=1 ./... && go vet -tags=integration ./... && test -z "$(gofmt -l .)"
TEST_DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable' go test -count=1 -race -tags=integration ./...
cd ..
git add backend/internal/handler/response.go backend/internal/handler/router_test.go backend/internal/handler/poi_test.go \
  backend/internal/importer/region.go backend/internal/importer/region_test.go backend/internal/importer/importer.go \
  backend/internal/importer/testdata/beijing_pois_osm.json backend/internal/importer/importer_integration_test.go \
  backend/cmd/importer/main.go docs/02-technical-design.md
git commit -m "fix(backend): fix gzip marking of small GeoJSON, log client cancellations as 499, skip POIs outside Beijing at import"
```

Expected: 两条测试命令都 PASS，`gofmt -l` 没有输出。

---

### Task 2: 前端依赖、坐标转换 `coord.ts`、瓦片配置与共享样例

**Files:**
- Modify: `frontend/package.json`、`frontend/package-lock.json`（由 npm 修改）
- Modify: `frontend/tsconfig.app.json`
- Create: `frontend/src/vite-env.d.ts`
- Create: `frontend/src/lib/coord.ts`、`frontend/src/lib/coord.test.ts`
- Create: `frontend/src/lib/tiles.ts`、`frontend/src/lib/tiles.test.ts`
- Create: `frontend/src/config.ts`
- Modify: `testdata/coord_cases.json`

**Interfaces:**
- Consumes: `testdata/coord_cases.json`（Go 的 `internal/geo` 测试也读取它）
- Produces:
  - `src/lib/coord.ts`：`type LngLat = [number, number]`、`type TileCrs = 'gcj02' | 'wgs84'`、`type CoordTransform = { toDisplay(p: LngLat): LngLat; toData(p: LngLat): LngLat }`、`outOfChina(lng, lat): boolean`、`wgs84ToGcj02(p): LngLat`、`gcj02ToWgs84(p): LngLat`、`createCoordTransform(crs): CoordTransform`
  - `src/lib/tiles.ts`：`DEFAULT_TILE_URL`、`expandTileUrl(template: string): string[]`、`parseTileCrs(value: string | undefined): TileCrs`
  - `src/config.ts`：`tileUrls: string[]`、`toDisplay`、`toData`（Task 3 会在这个文件里追加两个范围常量）

- [ ] **Step 1: 安装依赖**

```bash
cd frontend
npm install maplibre-gl@^6.14.0 @tanstack/react-query@^5.104.2 zustand@^5.0.15
npm install -D @types/geojson@^7946.0.16
```

Expected: `package.json` 的 `dependencies` 新增三项，`devDependencies` 新增 `@types/geojson`；`package-lock.json` 中的地址都是 `https://registry.npmjs.org/`。

- [ ] **Step 2: 允许导入 JSON，声明构建时环境变量**

在 `frontend/tsconfig.app.json` 的 `compilerOptions` 中，紧接着 `"noEmit": true,` 加一行（测试要导入仓库根目录下的共享样例）：

```json
    "resolveJsonModule": true,
```

创建 `frontend/src/vite-env.d.ts`：

```ts
interface ImportMetaEnv {
  readonly VITE_TILE_URL?: string
  readonly VITE_TILE_CRS?: string
}
```

- [ ] **Step 3: 扩充共享样例**

carry-forward 中记录过：样例只有一个境外点（东京，位于东边），反向转换的境外判断在西、南、北三个方向都没有覆盖。把 `testdata/coord_cases.json` 整个替换为下面的内容（新增的三个点由 gcoord 1.0.7 生成，境外点原样返回）：

```json
{
  "source": "gcoord@1.0.7 transform(WGS84 -> GCJ02)，生成于 2026-10-13，2026-10-14 补充塔什干、雅加达、雅库茨克三个境外点；gcj02 保留 9 位小数",
  "cases": [
    { "name": "天安门", "wgs84": [116.391275, 39.906217], "gcj02": [116.397516005, 39.907618208] },
    { "name": "鸟巢", "wgs84": [116.390297, 39.991853], "gcj02": [116.396548401, 39.993257194] },
    { "name": "北京大学", "wgs84": [116.304826, 39.986913], "gcj02": [116.310919103, 39.988184685] },
    { "name": "首都机场", "wgs84": [116.587115, 40.07715], "gcj02": [116.592921528, 40.078198902] },
    { "name": "东京（境外）", "wgs84": [139.767125, 35.681236], "gcj02": [139.767125, 35.681236] },
    { "name": "塔什干（境外，西）", "wgs84": [69.240562, 41.311081], "gcj02": [69.240562, 41.311081] },
    { "name": "雅加达（境外，南）", "wgs84": [106.845599, -6.208763], "gcj02": [106.845599, -6.208763] },
    { "name": "雅库茨克（境外，北）", "wgs84": [129.732178, 62.035452], "gcj02": [129.732178, 62.035452] }
  ]
}
```

Run: `cd backend && go test -count=1 ./internal/geo/`
Expected: PASS（Go 实现对新增的样例同样成立）。

- [ ] **Step 4: 编写坐标转换和瓦片配置的测试**

创建 `frontend/src/lib/coord.test.ts`：

```ts
import { describe, expect, it } from 'vitest'
import fixture from '../../../testdata/coord_cases.json'
import { createCoordTransform, gcj02ToWgs84, outOfChina, wgs84ToGcj02, type LngLat } from './coord'

type Case = { name: string; wgs84: LngLat; gcj02: LngLat }

const cases = fixture.cases as Case[]
const TOLERANCE = 1e-6

function expectClose(actual: LngLat, expected: LngLat) {
  expect(Math.abs(actual[0] - expected[0])).toBeLessThan(TOLERANCE)
  expect(Math.abs(actual[1] - expected[1])).toBeLessThan(TOLERANCE)
}

describe('wgs84ToGcj02', () => {
  it.each(cases)('matches the shared fixture: $name', ({ wgs84, gcj02 }) => {
    expectClose(wgs84ToGcj02(wgs84), gcj02)
  })
})

describe('gcj02ToWgs84', () => {
  it.each(cases)('inverts the forward transform: $name', ({ wgs84, gcj02 }) => {
    const inverse = gcj02ToWgs84(gcj02)
    expectClose(wgs84ToGcj02(inverse), gcj02)
    expectClose(inverse, wgs84)
  })

  it.each(cases.filter(({ wgs84 }) => outOfChina(...wgs84)))('returns points outside China unchanged: $name', ({ wgs84 }) => {
    expect(gcj02ToWgs84(wgs84)).toEqual(wgs84)
  })
})

describe('createCoordTransform', () => {
  const tiananmen = cases.find(({ name }) => name === '天安门')!

  it('converts between WGS-84 data and a GCJ-02 basemap', () => {
    const { toDisplay, toData } = createCoordTransform('gcj02')
    expectClose(toDisplay(tiananmen.wgs84), tiananmen.gcj02)
    expectClose(toData(tiananmen.gcj02), tiananmen.wgs84)
  })

  it('passes coordinates through for a WGS-84 basemap', () => {
    const { toDisplay, toData } = createCoordTransform('wgs84')
    expect(toDisplay(tiananmen.wgs84)).toEqual(tiananmen.wgs84)
    expect(toData(tiananmen.gcj02)).toEqual(tiananmen.gcj02)
  })
})
```

创建 `frontend/src/lib/tiles.test.ts`：

```ts
import { describe, expect, it } from 'vitest'
import { DEFAULT_TILE_URL, expandTileUrl, parseTileCrs } from './tiles'

describe('expandTileUrl', () => {
  it('expands {s} into the four Gaode subdomains', () => {
    expect(expandTileUrl(DEFAULT_TILE_URL)).toEqual([
      'https://webrd01.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
      'https://webrd02.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
      'https://webrd03.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
      'https://webrd04.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
    ])
  })

  it('keeps a template without {s} as the only URL', () => {
    expect(expandTileUrl('https://tile.openstreetmap.org/{z}/{x}/{y}.png')).toEqual([
      'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
    ])
  })
})

describe('parseTileCrs', () => {
  it('defaults to gcj02 when unset or empty', () => {
    expect(parseTileCrs(undefined)).toBe('gcj02')
    expect(parseTileCrs('')).toBe('gcj02')
  })

  it('accepts the two supported values', () => {
    expect(parseTileCrs('gcj02')).toBe('gcj02')
    expect(parseTileCrs('wgs84')).toBe('wgs84')
  })

  it('rejects anything else', () => {
    expect(() => parseTileCrs('WGS84')).toThrow('VITE_TILE_CRS 只能是 gcj02 或 wgs84，当前为「WGS84」')
  })
})
```

- [ ] **Step 5: 运行测试，确认失败**

Run: `cd frontend && npx vitest run src/lib/coord.test.ts src/lib/tiles.test.ts`
Expected: FAIL，提示无法解析 `./coord` 和 `./tiles`。

- [ ] **Step 6: 实现坐标转换和瓦片配置**

创建 `frontend/src/lib/coord.ts`（算法与后端 `backend/internal/geo/gcj02.go` 逐行对应）：

```ts
export type LngLat = [number, number]

export type TileCrs = 'gcj02' | 'wgs84'

export type CoordTransform = {
  toDisplay: (p: LngLat) => LngLat
  toData: (p: LngLat) => LngLat
}

// 克拉索夫斯基椭球参数，与后端 internal/geo 相同（后端写作 0.00669342162296594323，取 float64 后就是这个值）。
const KRASOVSKY_A = 6378245.0
const KRASOVSKY_EE = 0.006693421622965943

const INVERSE_TOLERANCE = 1e-7
const MAX_INVERSE_ITERATIONS = 10

export function outOfChina(lng: number, lat: number): boolean {
  return lng < 72.004 || lng > 137.8347 || lat < 0.8293 || lat > 55.8271
}

export function wgs84ToGcj02([lng, lat]: LngLat): LngLat {
  if (outOfChina(lng, lat)) return [lng, lat]
  const [dLng, dLat] = offset(lng, lat)
  return [lng + dLng, lat + dLat]
}

// 没有解析解，用迭代法反算：每轮按正向转换的残差修正估计值，与后端 GCJ02ToWGS84 的循环逐步一致。
export function gcj02ToWgs84([lng, lat]: LngLat): LngLat {
  if (outOfChina(lng, lat)) return [lng, lat]
  let wLng = lng
  let wLat = lat
  for (let i = 0; i < MAX_INVERSE_ITERATIONS; i++) {
    const [gLng, gLat] = wgs84ToGcj02([wLng, wLat])
    const dLng = gLng - lng
    const dLat = gLat - lat
    wLng -= dLng
    wLat -= dLat
    if (Math.abs(dLng) < INVERSE_TOLERANCE && Math.abs(dLat) < INVERSE_TOLERANCE) break
  }
  return [wLng, wLat]
}

// 底图是 GCJ-02 时，接口数据（WGS-84）显示前要转换，从地图上取到的坐标查询前要反算；底图是 WGS-84 时原样使用。
export function createCoordTransform(crs: TileCrs): CoordTransform {
  if (crs === 'wgs84') {
    const same = ([lng, lat]: LngLat): LngLat => [lng, lat]
    return { toDisplay: same, toData: same }
  }
  return { toDisplay: wgs84ToGcj02, toData: gcj02ToWgs84 }
}

function offset(lng: number, lat: number): LngLat {
  const x = lng - 105.0
  const y = lat - 35.0
  let dLat = transformLat(x, y)
  let dLng = transformLng(x, y)
  const radLat = (lat / 180.0) * Math.PI
  const magic = 1 - KRASOVSKY_EE * Math.sin(radLat) * Math.sin(radLat)
  const sqrtMagic = Math.sqrt(magic)
  dLat = (dLat * 180.0) / (((KRASOVSKY_A * (1 - KRASOVSKY_EE)) / (magic * sqrtMagic)) * Math.PI)
  dLng = (dLng * 180.0) / ((KRASOVSKY_A / sqrtMagic) * Math.cos(radLat) * Math.PI)
  return [dLng, dLat]
}

function transformLat(x: number, y: number): number {
  let r = -100.0 + 2.0 * x + 3.0 * y + 0.2 * y * y + 0.1 * x * y + 0.2 * Math.sqrt(Math.abs(x))
  r += ((20.0 * Math.sin(6.0 * x * Math.PI) + 20.0 * Math.sin(2.0 * x * Math.PI)) * 2.0) / 3.0
  r += ((20.0 * Math.sin(y * Math.PI) + 40.0 * Math.sin((y / 3.0) * Math.PI)) * 2.0) / 3.0
  r += ((160.0 * Math.sin((y / 12.0) * Math.PI) + 320.0 * Math.sin((y * Math.PI) / 30.0)) * 2.0) / 3.0
  return r
}

function transformLng(x: number, y: number): number {
  let r = 300.0 + x + 2.0 * y + 0.1 * x * x + 0.1 * x * y + 0.1 * Math.sqrt(Math.abs(x))
  r += ((20.0 * Math.sin(6.0 * x * Math.PI) + 20.0 * Math.sin(2.0 * x * Math.PI)) * 2.0) / 3.0
  r += ((20.0 * Math.sin(x * Math.PI) + 40.0 * Math.sin((x / 3.0) * Math.PI)) * 2.0) / 3.0
  r += ((150.0 * Math.sin((x / 12.0) * Math.PI) + 300.0 * Math.sin((x / 30.0) * Math.PI)) * 2.0) / 3.0
  return r
}
```

创建 `frontend/src/lib/tiles.ts`：

```ts
import type { TileCrs } from './coord'

export const DEFAULT_TILE_URL =
  'https://webrd0{s}.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}'

// MapLibre 不认识 {s} 占位符，要展开成 1 到 4 号子域名的多个地址，由 MapLibre 轮流请求。
export function expandTileUrl(template: string): string[] {
  if (!template.includes('{s}')) return [template]
  return ['1', '2', '3', '4'].map((s) => template.replaceAll('{s}', s))
}

export function parseTileCrs(value: string | undefined): TileCrs {
  if (value === undefined || value === '' || value === 'gcj02') return 'gcj02'
  if (value === 'wgs84') return 'wgs84'
  throw new Error(`VITE_TILE_CRS 只能是 gcj02 或 wgs84，当前为「${value}」`)
}
```

创建 `frontend/src/config.ts`：

```ts
import { createCoordTransform } from './lib/coord'
import { DEFAULT_TILE_URL, expandTileUrl, parseTileCrs } from './lib/tiles'

export const tileUrls = expandTileUrl(import.meta.env.VITE_TILE_URL || DEFAULT_TILE_URL)

export const { toDisplay, toData } = createCoordTransform(parseTileCrs(import.meta.env.VITE_TILE_CRS))
```

- [ ] **Step 7: 运行全部前端检查并提交**

```bash
cd frontend && npm run lint && npm run typecheck && npm test && npm run build && cd ..
git add frontend/package.json frontend/package-lock.json frontend/tsconfig.app.json frontend/src/vite-env.d.ts \
  frontend/src/lib/coord.ts frontend/src/lib/coord.test.ts frontend/src/lib/tiles.ts frontend/src/lib/tiles.test.ts \
  frontend/src/config.ts testdata/coord_cases.json
git commit -m "feat(frontend): add coordinate conversion, tile config and map dependencies"
```

Expected: 四条命令都没有错误或警告；Vitest 中坐标转换的 8 个样例全部通过。

---

### Task 3: GeoJSON 坐标转换、视野 bbox 与格式化工具

**Files:**
- Create: `frontend/src/lib/bbox.ts`、`frontend/src/lib/bbox.test.ts`
- Create: `frontend/src/lib/geojson.ts`、`frontend/src/lib/geojson.test.ts`
- Create: `frontend/src/lib/format.ts`、`frontend/src/lib/format.test.ts`
- Modify: `frontend/src/config.ts`（追加两个范围常量）

**Interfaces:**
- Consumes: `LngLat`（`src/lib/coord.ts`）
- Produces:
  - `src/lib/bbox.ts`：`type BBox = [number, number, number, number]`（minLng, minLat, maxLng, maxLat）、`type ViewBounds = { west; south; east; north }`、`viewToDataBBox(view: ViewBounds, toData): BBox`、`bboxParam(box: BBox): string`
  - `src/lib/geojson.ts`：`transformGeometry(geometry: Geometry, f): Geometry`、`transformCollection<G, P>(collection: FeatureCollection<G, P>, f): FeatureCollection<G, P>`、`bboxOfGeometry(geometry: Geometry): BBox`
  - `src/lib/format.ts`：`formatArea(km2: number): string`
  - `src/config.ts`：`BEIJING_BBOX: BBox`、`REGION_BBOX: BBox`

- [ ] **Step 1: 编写测试**

创建 `frontend/src/lib/bbox.test.ts`：

```ts
import { describe, expect, it } from 'vitest'
import { bboxParam, viewToDataBBox } from './bbox'
import type { LngLat } from './coord'

const identity = ([lng, lat]: LngLat): LngLat => [lng, lat]

describe('viewToDataBBox', () => {
  it('rounds outward to 0.001 degrees', () => {
    const box = viewToDataBBox({ west: 116.30049, south: 39.90001, east: 116.40001, north: 39.99999 }, identity)
    expect(box).toEqual([116.3, 39.9, 116.401, 40])
  })

  it('takes the extent of all four converted corners', () => {
    // 纬度越高向西偏得越多：最小经度来自西北角而不是西南角。
    const skew = ([lng, lat]: LngLat): LngLat => [lng - (lat - 39) * 0.5, lat]
    const box = viewToDataBBox({ west: 116, south: 39, east: 117, north: 40 }, skew)
    expect(box).toEqual([115.5, 39, 117, 40])
  })

  it('clamps to valid longitude and latitude', () => {
    const box = viewToDataBBox({ west: -200, south: -95, east: 200, north: 95 }, identity)
    expect(box).toEqual([-180, -90, 180, 90])
  })
})

describe('bboxParam', () => {
  it('formats the box as minLng,minLat,maxLng,maxLat', () => {
    expect(bboxParam([116.3, 39.9, 116.401, 40])).toBe('116.3,39.9,116.401,40')
  })
})
```

创建 `frontend/src/lib/geojson.test.ts`：

```ts
import type { FeatureCollection, MultiPolygon, Point } from 'geojson'
import { describe, expect, it } from 'vitest'
import type { LngLat } from './coord'
import { bboxOfGeometry, transformCollection, transformGeometry } from './geojson'

const shift = ([lng, lat]: LngLat): LngLat => [lng + 1, lat + 2]

const square: MultiPolygon = {
  type: 'MultiPolygon',
  coordinates: [
    [
      [
        [116, 39],
        [117, 39],
        [117, 40],
        [116, 40],
        [116, 39],
      ],
    ],
    [
      [
        [118, 41],
        [118.5, 41],
        [118.5, 41.5],
        [118, 41],
      ],
    ],
  ],
}

describe('transformGeometry', () => {
  it('transforms every position of a MultiPolygon without touching the input', () => {
    const before = structuredClone(square)

    const out = transformGeometry(square, shift) as MultiPolygon

    expect(out.coordinates[0][0][2]).toEqual([118, 42])
    expect(out.coordinates[1][0][1]).toEqual([119.5, 43])
    expect(square).toEqual(before)
  })

  it('keeps a third coordinate such as altitude', () => {
    expect(transformGeometry({ type: 'Point', coordinates: [116, 39, 50] }, shift)).toEqual({
      type: 'Point',
      coordinates: [117, 41, 50],
    })
  })

  it('recurses into geometry collections', () => {
    const out = transformGeometry(
      {
        type: 'GeometryCollection',
        geometries: [
          { type: 'Point', coordinates: [116, 39] },
          { type: 'LineString', coordinates: [[116, 39], [117, 40]] },
        ],
      },
      shift,
    )
    expect(out).toEqual({
      type: 'GeometryCollection',
      geometries: [
        { type: 'Point', coordinates: [117, 41] },
        { type: 'LineString', coordinates: [[117, 41], [118, 42]] },
      ],
    })
  })
})

describe('transformCollection', () => {
  it('converts geometries and keeps ids, properties and foreign members', () => {
    const collection: FeatureCollection<Point, { name: string }> & { truncated: boolean } = {
      type: 'FeatureCollection',
      truncated: true,
      features: [{ type: 'Feature', id: 7, properties: { name: '北京大学' }, geometry: { type: 'Point', coordinates: [116.3, 39.98] } }],
    }

    const out = transformCollection(collection, shift)

    expect(out).toEqual({
      type: 'FeatureCollection',
      truncated: true,
      features: [{ type: 'Feature', id: 7, properties: { name: '北京大学' }, geometry: { type: 'Point', coordinates: [117.3, 41.98] } }],
    })
  })
})

describe('bboxOfGeometry', () => {
  it('returns the extent of all polygons', () => {
    expect(bboxOfGeometry(square)).toEqual([116, 39, 118.5, 41.5])
  })
})
```

创建 `frontend/src/lib/format.test.ts`：

```ts
import { describe, expect, it } from 'vitest'
import { formatArea } from './format'

describe('formatArea', () => {
  it('keeps at most two decimals and groups thousands', () => {
    expect(formatArea(430.77)).toBe('430.77 平方千米')
    expect(formatArea(2229.4)).toBe('2,229.4 平方千米')
    expect(formatArea(41.8)).toBe('41.8 平方千米')
  })
})
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd frontend && npx vitest run src/lib/bbox.test.ts src/lib/geojson.test.ts src/lib/format.test.ts`
Expected: FAIL，提示无法解析 `./bbox`、`./geojson`、`./format`。

- [ ] **Step 3: 实现**

创建 `frontend/src/lib/bbox.ts`：

```ts
import type { LngLat } from './coord'

// 依次为 minLng, minLat, maxLng, maxLat。
export type BBox = [number, number, number, number]

export type ViewBounds = { west: number; south: number; east: number; north: number }

// 向外取整到 0.001°（约 100 米）：视野只移动一点时请求参数不变，可以直接命中缓存。
const STEP = 1000

// 地图视野是底图坐标系，四个角都要换算成数据坐标系再取外包框，因为偏移量随位置变化。
export function viewToDataBBox(view: ViewBounds, toData: (p: LngLat) => LngLat): BBox {
  const corners: LngLat[] = [
    [view.west, view.south],
    [view.west, view.north],
    [view.east, view.south],
    [view.east, view.north],
  ]
  const converted = corners.map(toData)
  const lngs = converted.map(([lng]) => lng)
  const lats = converted.map(([, lat]) => lat)
  return [
    clamp(Math.floor(Math.min(...lngs) * STEP) / STEP, -180, 180),
    clamp(Math.floor(Math.min(...lats) * STEP) / STEP, -90, 90),
    clamp(Math.ceil(Math.max(...lngs) * STEP) / STEP, -180, 180),
    clamp(Math.ceil(Math.max(...lats) * STEP) / STEP, -90, 90),
  ]
}

export function bboxParam(box: BBox): string {
  return box.join(',')
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}
```

创建 `frontend/src/lib/geojson.ts`：

```ts
import type { FeatureCollection, Geometry, Position } from 'geojson'
import type { BBox } from './bbox'
import type { LngLat } from './coord'

type Transform = (p: LngLat) => LngLat

export function transformGeometry(geometry: Geometry, f: Transform): Geometry {
  switch (geometry.type) {
    case 'Point':
      return { ...geometry, coordinates: mapPosition(geometry.coordinates, f) }
    case 'MultiPoint':
    case 'LineString':
      return { ...geometry, coordinates: geometry.coordinates.map((p) => mapPosition(p, f)) }
    case 'MultiLineString':
    case 'Polygon':
      return { ...geometry, coordinates: geometry.coordinates.map((line) => line.map((p) => mapPosition(p, f))) }
    case 'MultiPolygon':
      return {
        ...geometry,
        coordinates: geometry.coordinates.map((polygon) => polygon.map((ring) => ring.map((p) => mapPosition(p, f)))),
      }
    case 'GeometryCollection':
      return { ...geometry, geometries: geometry.geometries.map((g) => transformGeometry(g, f)) }
  }
}

// 返回新的 FeatureCollection，只转换几何，属性和顶层的其他字段（例如 truncated）原样保留。
export function transformCollection<G extends Geometry, P>(
  collection: FeatureCollection<G, P>,
  f: Transform,
): FeatureCollection<G, P> {
  return {
    ...collection,
    features: collection.features.map((feature) => ({ ...feature, geometry: transformGeometry(feature.geometry, f) as G })),
  }
}

export function bboxOfGeometry(geometry: Geometry): BBox {
  const box: BBox = [Infinity, Infinity, -Infinity, -Infinity]
  for (const [lng, lat] of positions(geometry)) {
    box[0] = Math.min(box[0], lng)
    box[1] = Math.min(box[1], lat)
    box[2] = Math.max(box[2], lng)
    box[3] = Math.max(box[3], lat)
  }
  return box
}

function mapPosition(p: Position, f: Transform): Position {
  const [lng, lat] = f([p[0], p[1]])
  return p.length > 2 ? [lng, lat, ...p.slice(2)] : [lng, lat]
}

function positions(geometry: Geometry): Position[] {
  switch (geometry.type) {
    case 'Point':
      return [geometry.coordinates]
    case 'MultiPoint':
    case 'LineString':
      return geometry.coordinates
    case 'MultiLineString':
    case 'Polygon':
      return geometry.coordinates.flat()
    case 'MultiPolygon':
      return geometry.coordinates.flat(2)
    case 'GeometryCollection':
      return geometry.geometries.flatMap(positions)
  }
}
```

创建 `frontend/src/lib/format.ts`：

```ts
export function formatArea(km2: number): string {
  return `${km2.toLocaleString('zh-CN', { maximumFractionDigits: 2 })} 平方千米`
}
```

把 `frontend/src/config.ts` 整个替换为：

```ts
import type { BBox } from './lib/bbox'
import { createCoordTransform } from './lib/coord'
import { DEFAULT_TILE_URL, expandTileUrl, parseTileCrs } from './lib/tiles'

export const tileUrls = expandTileUrl(import.meta.env.VITE_TILE_URL || DEFAULT_TILE_URL)

export const { toDisplay, toData } = createCoordTransform(parseTileCrs(import.meta.env.VITE_TILE_CRS))

// 北京市 16 个区边界的外包框（WGS-84），作为初始视野。
export const BEIJING_BBOX: BBox = [115.42, 39.44, 117.51, 41.06]

// 北京市外扩约 1° 的范围（WGS-84）。地图不能移出这个范围；后端导入器用同一个范围过滤 POI。
export const REGION_BBOX: BBox = [114.4, 38.4, 118.5, 42.1]
```

- [ ] **Step 4: 运行全部前端检查并提交**

```bash
cd frontend && npm run lint && npm run typecheck && npm test && npm run build && cd ..
git add frontend/src/lib/bbox.ts frontend/src/lib/bbox.test.ts frontend/src/lib/geojson.ts frontend/src/lib/geojson.test.ts \
  frontend/src/lib/format.ts frontend/src/lib/format.test.ts frontend/src/config.ts
git commit -m "feat(frontend): add GeoJSON, bbox and formatting helpers"
```

Expected: 四条命令都没有错误或警告。

---

### Task 4: 接口类型、查询 hooks、分类工具与全局状态

**Files:**
- Create: `frontend/src/api/types.ts`
- Create: `frontend/src/api/queries.ts`、`frontend/src/api/queries.test.ts`
- Modify: `frontend/src/api/client.ts`、`frontend/src/api/client.test.ts`（新增 `errorMessage`）
- Create: `frontend/src/lib/categories.ts`、`frontend/src/lib/categories.test.ts`
- Create: `frontend/src/store/useAppStore.ts`、`frontend/src/store/useAppStore.test.ts`

**Interfaces:**
- Consumes: `apiGet`、`ApiError`（`src/api/client.ts`）；`BBox`、`bboxParam`（`src/lib/bbox.ts`）；`LngLat`（`src/lib/coord.ts`）
- Produces:
  - `src/api/types.ts`：`DataResponse<T>`、`Category`、`DistrictProperties`、`DistrictCollection`、`CategoryCount`、`DistrictDetail`、`POIProperties`、`POICollection`、`POIDetail`（与后端 `internal/service/types.go` 的 JSON 一一对应）
  - `src/api/client.ts`：`errorMessage(err: unknown): string`
  - `src/api/queries.ts`：`POI_LIMIT = 5000`；查询选项工厂 `districtsQuery()`、`districtQuery(adcode)`、`categoriesQuery()`、`poisQuery(bbox, categories)`、`poiDetailQuery(id)`（`queryOptions`，可以在 Node 中测试）；hooks `useDistricts()`、`useDistrict(adcode: string | null)`、`useCategories()`、`usePoisInBBox(bbox: BBox | null, categories: string[] | undefined | null)`、`usePoiDetail(id: number | null)`。分类全部关闭（`categories === null`）时 `usePoisInBBox` 不请求，也不沿用上一次的结果
  - `src/lib/categories.ts`：`FALLBACK_POI_COLOR`、`categoriesParam(allCodes: string[], hidden: string[]): string[] | undefined | null`、`categoryColorExpression(categories: Category[]): ExpressionSpecification | string`
  - `src/store/useAppStore.ts`：`useAppStore`，状态 `hiddenCategories: string[]`、`selectedDistrict: string | null`、`activePoiId: number | null`、`errorMessage: string | null`，动作 `toggleCategory(code)`、`selectDistrict(adcode | null)`、`openPoi(id | null)`、`showError(message)`、`dismissError()`

- [ ] **Step 1: 编写测试**

创建 `frontend/src/lib/categories.test.ts`：

```ts
import { describe, expect, it } from 'vitest'
import { categoriesParam, categoryColorExpression, FALLBACK_POI_COLOR } from './categories'

const codes = ['subway_station', 'university', 'attraction', 'hospital', 'park']

describe('categoriesParam', () => {
  it('omits the parameter when nothing is hidden', () => {
    expect(categoriesParam(codes, [])).toBeUndefined()
  })

  it('lists the enabled categories in catalog order', () => {
    expect(categoriesParam(codes, ['hospital', 'subway_station'])).toEqual(['university', 'attraction', 'park'])
  })

  it('returns null when every category is hidden', () => {
    expect(categoriesParam(codes, [...codes].reverse())).toBeNull()
  })
})

describe('categoryColorExpression', () => {
  it('maps each category code to its color', () => {
    const expr = categoryColorExpression([
      { code: 'park', name: '公园', color: '#43A047' },
      { code: 'hospital', name: '医院', color: '#8E24AA' },
    ])
    expect(expr).toEqual(['match', ['get', 'category'], 'park', '#43A047', 'hospital', '#8E24AA', FALLBACK_POI_COLOR])
  })

  it('falls back to a plain color before categories load', () => {
    expect(categoryColorExpression([])).toBe(FALLBACK_POI_COLOR)
  })
})
```

创建 `frontend/src/store/useAppStore.test.ts`：

```ts
import { beforeEach, describe, expect, it } from 'vitest'
import { useAppStore } from './useAppStore'

const initial = useAppStore.getState()

describe('useAppStore', () => {
  beforeEach(() => {
    useAppStore.setState(initial, true)
  })

  it('toggles a category off and back on', () => {
    const { toggleCategory } = useAppStore.getState()

    toggleCategory('hospital')
    toggleCategory('park')
    expect(useAppStore.getState().hiddenCategories).toEqual(['hospital', 'park'])

    toggleCategory('hospital')
    expect(useAppStore.getState().hiddenCategories).toEqual(['park'])
  })

  it('shows and dismisses an error message', () => {
    useAppStore.getState().showError('网络错误')
    expect(useAppStore.getState().errorMessage).toBe('网络错误')

    useAppStore.getState().dismissError()
    expect(useAppStore.getState().errorMessage).toBeNull()
  })
})
```

创建 `frontend/src/api/queries.test.ts`（TanStack Query 的 `QueryClient`、`QueryObserver` 不依赖 React，可以直接在 Node 中运行）：

```ts
import { QueryClient, QueryObserver, skipToken } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { BBox } from '../lib/bbox'
import { districtQuery, POI_LIMIT, poiDetailQuery, poisQuery } from './queries'

const box: BBox = [116.3, 39.9, 116.4, 40]
const emptyCollection = { type: 'FeatureCollection', features: [], truncated: false }

function stubFetch(body: unknown) {
  const fetchMock = vi.fn<typeof fetch>(
    async () => new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } }),
  )
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('poisQuery', () => {
  it('keeps all, none and a category list in separate cache entries', () => {
    const keys = [poisQuery(box, undefined), poisQuery(box, null), poisQuery(box, ['park'])].map((q) =>
      JSON.stringify(q.queryKey),
    )
    expect(new Set(keys).size).toBe(3)
  })

  it('skips the request without a bbox or when every category is hidden', () => {
    expect(poisQuery(null, undefined).queryFn).toBe(skipToken)
    expect(poisQuery(box, null).queryFn).toBe(skipToken)
  })

  it('requests the bbox, the categories and the fixed limit', async () => {
    const fetchMock = stubFetch(emptyCollection)

    await new QueryClient().fetchQuery(poisQuery(box, ['park', 'university']))

    expect(fetchMock.mock.calls[0][0]).toBe(
      `/api/v1/pois?bbox=116.3%2C39.9%2C116.4%2C40&categories=park%2Cuniversity&limit=${POI_LIMIT}`,
    )
  })

  it('does not fall back to the previous result when every category is hidden', async () => {
    stubFetch(emptyCollection)
    const observer = new QueryObserver(new QueryClient(), poisQuery(box, undefined))
    const unsubscribe = observer.subscribe(() => {})
    await vi.waitFor(() => expect(observer.getCurrentResult().isSuccess).toBe(true))

    observer.setOptions(poisQuery(box, null))

    expect(observer.getCurrentResult().data).toBeUndefined()
    unsubscribe()
  })
})

describe('detail queries', () => {
  it('unwrap the data envelope', async () => {
    stubFetch({ data: { id: 42, name: '北京大学' } })

    await expect(new QueryClient().fetchQuery(poiDetailQuery(42))).resolves.toEqual({ id: 42, name: '北京大学' })
  })

  it('skip the request without an id', () => {
    expect(poiDetailQuery(null).queryFn).toBe(skipToken)
    expect(districtQuery(null).queryFn).toBe(skipToken)
  })
})
```

在 `frontend/src/api/client.test.ts` 中，把第二行的导入改为：

```ts
import { ApiError, apiGet, buildUrl, errorMessage } from './client'
```

并在文件末尾追加：

```ts

describe('errorMessage', () => {
  it('uses the server message of an ApiError', () => {
    expect(errorMessage(new ApiError(404, 'NOT_FOUND', 'POI 不存在'))).toBe('POI 不存在')
  })

  it('falls back to a generic message for anything else', () => {
    expect(errorMessage(new TypeError('boom'))).toBe('请求失败，请稍后重试')
  })
})
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd frontend && npx vitest run src/lib/categories.test.ts src/store/useAppStore.test.ts src/api/client.test.ts src/api/queries.test.ts`
Expected: FAIL，提示无法解析 `./categories`、`./useAppStore`、`./queries`，以及 `errorMessage` 不存在。

- [ ] **Step 3: 实现接口类型、`errorMessage` 和分类工具**

创建 `frontend/src/api/types.ts`：

```ts
import type { FeatureCollection, MultiPolygon, Point } from 'geojson'
import type { LngLat } from '../lib/coord'

// 与后端 internal/service/types.go 的 JSON 结构一一对应，坐标都是 WGS-84。

export type DataResponse<T> = { data: T }

export type Category = { code: string; name: string; color: string }

export type DistrictProperties = {
  adcode: string
  name: string
  area_km2: number
  poi_count: number
  center: LngLat
}

export type DistrictCollection = FeatureCollection<MultiPolygon, DistrictProperties>

export type CategoryCount = { code: string; name: string; count: number }

export type DistrictDetail = {
  adcode: string
  name: string
  area_km2: number
  center: LngLat
  poi_counts: CategoryCount[]
}

export type POIProperties = { id: number; name: string; category: string }

export type POICollection = FeatureCollection<Point, POIProperties> & { truncated: boolean }

export type POIDetail = {
  id: number
  name: string
  name_en: string | null
  category: Category
  district: { adcode: string; name: string } | null
  location: LngLat
  address: string | null
  tags: Record<string, string>
  osm_url: string
}
```

在 `frontend/src/api/client.ts` 中，紧接在 `async function toApiError(res: Response): Promise<ApiError> {` 这一行之前插入：

```ts
export function errorMessage(err: unknown): string {
  return err instanceof ApiError ? err.message : '请求失败，请稍后重试'
}

```

创建 `frontend/src/lib/categories.ts`：

```ts
import type { ExpressionSpecification } from 'maplibre-gl'
import type { Category } from '../api/types'

export const FALLBACK_POI_COLOR = '#64748b'

// 请求 /pois 时的 categories 参数：undefined 表示不传（全部分类），null 表示分类全部关闭、不必请求。
export function categoriesParam(allCodes: string[], hidden: string[]): string[] | undefined | null {
  if (hidden.length === 0) return undefined
  const enabled = allCodes.filter((code) => !hidden.includes(code))
  return enabled.length === 0 ? null : enabled
}

export function categoryColorExpression(categories: Category[]): ExpressionSpecification | string {
  if (categories.length === 0) return FALLBACK_POI_COLOR
  const [first, ...rest] = categories
  return [
    'match',
    ['get', 'category'],
    first.code,
    first.color,
    ...rest.flatMap((c) => [c.code, c.color]),
    FALLBACK_POI_COLOR,
  ]
}
```

- [ ] **Step 4: 实现 store 和查询 hooks**

创建 `frontend/src/store/useAppStore.ts`：

```ts
import { create } from 'zustand'

export type AppState = {
  hiddenCategories: string[]
  selectedDistrict: string | null
  activePoiId: number | null
  errorMessage: string | null
  toggleCategory: (code: string) => void
  selectDistrict: (adcode: string | null) => void
  openPoi: (id: number | null) => void
  showError: (message: string) => void
  dismissError: () => void
}

export const useAppStore = create<AppState>()((set) => ({
  hiddenCategories: [],
  selectedDistrict: null,
  activePoiId: null,
  errorMessage: null,
  toggleCategory: (code) =>
    set((state) => ({
      hiddenCategories: state.hiddenCategories.includes(code)
        ? state.hiddenCategories.filter((c) => c !== code)
        : [...state.hiddenCategories, code],
    })),
  selectDistrict: (adcode) => set({ selectedDistrict: adcode }),
  openPoi: (id) => set({ activePoiId: id }),
  showError: (message) => set({ errorMessage: message }),
  dismissError: () => set({ errorMessage: null }),
}))
```

创建 `frontend/src/api/queries.ts`（每个查询先定义成 `queryOptions` 工厂，hook 只调用 `useQuery`，这样查询键、跳过条件和请求参数都能在 Node 中测试）：

```ts
import { keepPreviousData, queryOptions, skipToken, useQuery } from '@tanstack/react-query'
import { bboxParam, type BBox } from '../lib/bbox'
import { apiGet } from './client'
import type { Category, DataResponse, DistrictCollection, DistrictDetail, POICollection, POIDetail } from './types'

// 当前快照约 4100 个 POI：按接口上限请求时，整个北京也不会被截断，聚合数字才准确。
export const POI_LIMIT = 5000

export function districtsQuery() {
  return queryOptions({
    queryKey: ['districts'],
    queryFn: ({ signal }) => apiGet<DistrictCollection>('/districts', undefined, signal),
    staleTime: Infinity,
  })
}

export function districtQuery(adcode: string | null) {
  return queryOptions({
    queryKey: ['district', adcode],
    queryFn:
      adcode === null
        ? skipToken
        : ({ signal }) => apiGet<DataResponse<DistrictDetail>>(`/districts/${adcode}`, undefined, signal).then((r) => r.data),
    staleTime: Infinity,
  })
}

export function categoriesQuery() {
  return queryOptions({
    queryKey: ['poi-categories'],
    queryFn: ({ signal }) => apiGet<DataResponse<Category[]>>('/poi-categories', undefined, signal).then((r) => r.data),
    staleTime: Infinity,
  })
}

// categories 为 undefined 表示全部分类；为 null 表示分类全部关闭：此时不请求，也不沿用上一次的结果。
export function poisQuery(bbox: BBox | null, categories: string[] | undefined | null) {
  return queryOptions({
    queryKey: ['pois', bbox && bboxParam(bbox), categories === null ? 'none' : (categories ?? 'all')],
    queryFn:
      bbox === null || categories === null
        ? skipToken
        : ({ signal }) => apiGet<POICollection>('/pois', { bbox: bboxParam(bbox), categories, limit: POI_LIMIT }, signal),
    placeholderData: categories === null ? undefined : keepPreviousData,
    staleTime: 5 * 60 * 1000,
  })
}

export function poiDetailQuery(id: number | null) {
  return queryOptions({
    queryKey: ['poi', id],
    queryFn:
      id === null
        ? skipToken
        : ({ signal }) => apiGet<DataResponse<POIDetail>>(`/pois/${id}`, undefined, signal).then((r) => r.data),
    staleTime: 5 * 60 * 1000,
  })
}

export function useDistricts() {
  return useQuery(districtsQuery())
}

export function useDistrict(adcode: string | null) {
  return useQuery(districtQuery(adcode))
}

export function useCategories() {
  return useQuery(categoriesQuery())
}

export function usePoisInBBox(bbox: BBox | null, categories: string[] | undefined | null) {
  return useQuery(poisQuery(bbox, categories))
}

export function usePoiDetail(id: number | null) {
  return useQuery(poiDetailQuery(id))
}
```

`poisQuery` 在分类全部关闭时把 `placeholderData` 设为 `undefined`：TanStack Query 不管查询是否被跳过都会使用占位数据，沿用 `keepPreviousData` 会把关闭之前的 POI 原样返回。

- [ ] **Step 5: 运行全部前端检查并提交**

```bash
cd frontend && npm run lint && npm run typecheck && npm test && npm run build && cd ..
git add frontend/src/api/types.ts frontend/src/api/queries.ts frontend/src/api/queries.test.ts frontend/src/api/client.ts frontend/src/api/client.test.ts \
  frontend/src/lib/categories.ts frontend/src/lib/categories.test.ts \
  frontend/src/store/useAppStore.ts frontend/src/store/useAppStore.test.ts
git commit -m "feat(frontend): add API types, query hooks, category helpers and app store"
```

Expected: 四条命令都没有错误或警告。

---

### Task 5: 地图外壳：底图、数据图层、错误提示条，以及镜像与 nginx 配置

**Files:**
- Create: `frontend/src/map/layers.ts`
- Create: `frontend/src/map/MapView.tsx`
- Create: `frontend/src/components/ErrorBanner.tsx`
- Modify: `frontend/src/main.tsx`、`frontend/src/App.tsx`（整体替换）
- Modify: `frontend/vite.config.ts`
- Modify: `frontend/nginx.conf`
- Modify: `frontend/Dockerfile`
- Modify: `docker-compose.yml`、`.env.example`

**Interfaces:**
- Consumes: `tileUrls`、`toDisplay`、`BEIJING_BBOX`、`REGION_BBOX`（`src/config.ts`）；`FALLBACK_POI_COLOR`（`src/lib/categories.ts`）；`errorMessage`（`src/api/client.ts`）；`useAppStore`
- Produces:
  - `src/map/layers.ts`：`SOURCE = { districts, districtLabels, pois }`、`LAYER = { districtFill, districtLine, districtLabel, poiCluster, poiClusterCount, poiPoint }`、`EMPTY_COLLECTION`、`baseStyle(tiles: string[]): StyleSpecification`、`addDataLayers(map): void`
  - `src/map/MapView.tsx`：`MapView`（`children?: ReactNode`，地图加载完成、图层建好之后才渲染 children）、`useMap(): MaplibreMap`
  - `src/components/ErrorBanner.tsx`：`ErrorBanner`

这个任务和后面两个任务的代码都依赖 WebGL 和真实的地图，没有 Vitest 测试：用 lint、类型检查和构建保证代码正确，地图效果在全部任务完成后由协调者在浏览器中验收。

- [ ] **Step 1: 图层定义**

创建 `frontend/src/map/layers.ts`：

```ts
import type { FeatureCollection } from 'geojson'
import type { Map as MaplibreMap, StyleSpecification } from 'maplibre-gl'
import { FALLBACK_POI_COLOR } from '../lib/categories'

export const SOURCE = {
  districts: 'districts',
  districtLabels: 'district-labels',
  pois: 'pois',
} as const

export const LAYER = {
  districtFill: 'district-fill',
  districtLine: 'district-line',
  districtLabel: 'district-label',
  poiCluster: 'poi-cluster',
  poiClusterCount: 'poi-cluster-count',
  poiPoint: 'poi-point',
} as const

export const EMPTY_COLLECTION: FeatureCollection = { type: 'FeatureCollection', features: [] }

export function baseStyle(tiles: string[]): StyleSpecification {
  return {
    version: 8,
    sources: {
      basemap: { type: 'raster', tiles, tileSize: 256, maxzoom: 18, attribution: '底图 © 高德地图' },
    },
    layers: [{ id: 'basemap', type: 'raster', source: 'basemap' }],
  }
}

// 所有数据图层在地图加载时一次建好（数据先为空），保证从下到上的顺序固定；之后只更新数据和样式。
export function addDataLayers(map: MaplibreMap): void {
  map.addSource(SOURCE.districts, {
    type: 'geojson',
    data: EMPTY_COLLECTION,
    promoteId: 'adcode',
    attribution: '行政区划 © 阿里云 DataV.GeoAtlas',
  })
  map.addSource(SOURCE.districtLabels, { type: 'geojson', data: EMPTY_COLLECTION })
  map.addSource(SOURCE.pois, {
    type: 'geojson',
    data: EMPTY_COLLECTION,
    cluster: true,
    clusterRadius: 50,
    clusterMaxZoom: 14,
    attribution: '© OpenStreetMap contributors（ODbL）',
  })

  map.addLayer({
    id: LAYER.districtFill,
    type: 'fill',
    source: SOURCE.districts,
    paint: {
      'fill-color': '#2563eb',
      'fill-opacity': [
        'case',
        ['boolean', ['feature-state', 'selected'], false],
        0.22,
        ['boolean', ['feature-state', 'hover'], false],
        0.14,
        0.04,
      ],
    },
  })
  map.addLayer({
    id: LAYER.districtLine,
    type: 'line',
    source: SOURCE.districts,
    paint: { 'line-color': '#1d4ed8', 'line-width': 1.2, 'line-opacity': 0.8 },
  })
  map.addLayer({
    id: LAYER.districtLabel,
    type: 'symbol',
    source: SOURCE.districtLabels,
    layout: { 'text-field': ['get', 'name'], 'text-size': 13 },
    paint: { 'text-color': '#1e3a8a', 'text-halo-color': '#ffffff', 'text-halo-width': 1.5 },
  })
  map.addLayer({
    id: LAYER.poiCluster,
    type: 'circle',
    source: SOURCE.pois,
    filter: ['has', 'point_count'],
    paint: {
      'circle-color': '#f97316',
      'circle-opacity': 0.85,
      'circle-radius': ['step', ['get', 'point_count'], 14, 50, 18, 200, 24],
      'circle-stroke-color': '#ffffff',
      'circle-stroke-width': 1.5,
    },
  })
  map.addLayer({
    id: LAYER.poiClusterCount,
    type: 'symbol',
    source: SOURCE.pois,
    filter: ['has', 'point_count'],
    layout: { 'text-field': ['get', 'point_count_abbreviated'], 'text-size': 12, 'text-allow-overlap': true },
    paint: { 'text-color': '#ffffff' },
  })
  map.addLayer({
    id: LAYER.poiPoint,
    type: 'circle',
    source: SOURCE.pois,
    filter: ['!', ['has', 'point_count']],
    paint: {
      'circle-color': FALLBACK_POI_COLOR,
      'circle-radius': 5,
      'circle-stroke-color': '#ffffff',
      'circle-stroke-width': 1,
    },
  })
}
```

- [ ] **Step 2: 地图组件**

创建 `frontend/src/map/MapView.tsx`：

```tsx
import 'maplibre-gl/dist/maplibre-gl.css'
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url'
import { Map as MaplibreMap, setWorkerUrl } from 'maplibre-gl'
import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { BEIJING_BBOX, REGION_BBOX, tileUrls, toDisplay } from '../config'
import type { BBox } from '../lib/bbox'
import type { LngLat } from '../lib/coord'
import { addDataLayers, baseStyle } from './layers'

// MapLibre 6 在运行时按 import.meta.url 拼出 worker 地址，打包后那个文件并不存在，所以显式交给 Vite 输出并指定。
setWorkerUrl(workerUrl)

const MapContext = createContext<MaplibreMap | null>(null)

export function useMap(): MaplibreMap {
  const map = useContext(MapContext)
  if (!map) throw new Error('useMap 只能在 MapView 内部使用')
  return map
}

function displayBounds([minLng, minLat, maxLng, maxLat]: BBox): [LngLat, LngLat] {
  return [toDisplay([minLng, minLat]), toDisplay([maxLng, maxLat])]
}

// 地图加载完成、数据图层建好之后才渲染 children，子组件可以直接使用这些图层。
export function MapView({ children }: { children?: ReactNode }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [map, setMap] = useState<MaplibreMap | null>(null)

  useEffect(() => {
    const instance = new MaplibreMap({
      container: containerRef.current!,
      style: baseStyle(tileUrls),
      bounds: displayBounds(BEIJING_BBOX),
      maxBounds: displayBounds(REGION_BBOX),
      maxZoom: 18,
      hash: true,
      attributionControl: { compact: false },
    })
    instance.on('load', () => {
      addDataLayers(instance)
      setMap(instance)
    })
    return () => instance.remove()
  }, [])

  return (
    <>
      {/* maplibre-gl.css 会给容器加上 position: relative，所以用 h-full 撑满，而不是 absolute inset-0。 */}
      <div ref={containerRef} className="h-full w-full" />
      {map && <MapContext value={map}>{children}</MapContext>}
    </>
  )
}
```

- [ ] **Step 3: 错误提示条、应用入口和页面骨架**

创建 `frontend/src/components/ErrorBanner.tsx`：

```tsx
import { useAppStore } from '../store/useAppStore'

export function ErrorBanner() {
  const message = useAppStore((s) => s.errorMessage)
  const dismissError = useAppStore((s) => s.dismissError)

  if (!message) return null
  return (
    <div
      role="alert"
      className="pointer-events-auto flex max-w-xs items-start gap-2 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 shadow-lg ring-1 ring-red-200"
    >
      <span className="flex-1">{message}</span>
      <button type="button" aria-label="关闭提示" onClick={dismissError} className="leading-none hover:text-red-900">
        ×
      </button>
    </div>
  )
}
```

把 `frontend/src/main.tsx` 整个替换为：

```tsx
import { QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { errorMessage } from './api/client'
import App from './App'
import { useAppStore } from './store/useAppStore'
import './index.css'

const queryClient = new QueryClient({
  queryCache: new QueryCache({
    onError: (error) => useAppStore.getState().showError(errorMessage(error)),
  }),
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
)
```

把 `frontend/src/App.tsx` 整个替换为（阶段 0 的健康检查页面不再需要）：

```tsx
import { ErrorBanner } from './components/ErrorBanner'
import { MapView } from './map/MapView'

export default function App() {
  return (
    <main className="relative h-full w-full overflow-hidden">
      <MapView />
      <div className="pointer-events-none absolute right-3 top-3 z-10 flex flex-col items-end gap-2">
        <ErrorBanner />
      </div>
    </main>
  )
}
```

- [ ] **Step 4: 构建配置**

在 `frontend/vite.config.ts` 中，把：

```ts
  plugins: [react(), tailwindcss()],
  server: {
```

替换为：

```ts
  plugins: [react(), tailwindcss()],
  build: {
    // maplibre-gl 打包压缩后本身就有约 1 MB，拆成单独的包也一样会触发默认 500 kB 的警告。
    chunkSizeWarningLimit: 1500,
  },
  server: {
```

在 `frontend/nginx.conf` 中，把：

```nginx
    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
```

替换为：

```nginx
    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";

        # nginx 自带的 mime.types 不认识 .mjs，会按 application/octet-stream 返回，浏览器拒绝把 MapLibre 的 worker 当作模块加载。
        location ~ \.mjs$ {
            types { application/javascript mjs; }
        }
    }
```

把 `frontend/Dockerfile` 中的：

```dockerfile
COPY . .
RUN npm run build
```

替换为（构建阶段的 `ARG` 会作为环境变量传给 `RUN`，Vite 在构建时读取 `VITE_*` 变量）：

```dockerfile
COPY . .
ARG VITE_TILE_URL=
ARG VITE_TILE_CRS=gcj02
RUN npm run build
```

在 `docker-compose.yml` 的 `frontend.build.args` 中，紧接着 `NPM_REGISTRY: ${NPM_REGISTRY:-https://registry.npmjs.org/}` 加两行：

```yaml
        VITE_TILE_URL: ${VITE_TILE_URL:-}
        VITE_TILE_CRS: ${VITE_TILE_CRS:-gcj02}
```

在 `.env.example` 末尾追加：

```text

# 底图瓦片地址和坐标系，构建前端镜像时写入页面。留空使用高德瓦片；
# 换成 OSM 等 WGS-84 底图时，把 VITE_TILE_CRS 设为 wgs84，前端就不再做坐标转换。
VITE_TILE_URL=
VITE_TILE_CRS=gcj02
```

- [ ] **Step 5: 运行前端检查**

Run: `cd frontend && npm run lint && npm run typecheck && npm test && npm run build`
Expected: 四条命令都没有错误或警告；`dist/assets/` 下有一个 `maplibre-gl-worker-*.mjs` 文件。

- [ ] **Step 6: 用 compose 检查镜像和 nginx 配置**

```bash
FRONTEND_PORT=18080 docker compose up --build -d
# 等 backend 变为 healthy（docker compose ps），然后：
curl -s http://localhost:18080/ | grep -c 'id="root"'                     # 期望 1
JS=$(curl -s http://localhost:18080/ | grep -o '/assets/index-[A-Za-z0-9_-]*\.js' | head -1)
WORKER=$(curl -s "http://localhost:18080$JS" | grep -o 'maplibre-gl-worker-[A-Za-z0-9_-]*\.mjs' | head -1)
echo "$WORKER"                                                              # 期望非空
curl -s -o /dev/null -D - "http://localhost:18080/assets/$WORKER" | grep -i '^content-type'   # 期望 application/javascript
docker compose stop frontend backend
```

记录全部输出。

- [ ] **Step 7: 提交**

```bash
git add frontend/src/map/layers.ts frontend/src/map/MapView.tsx frontend/src/components/ErrorBanner.tsx \
  frontend/src/main.tsx frontend/src/App.tsx frontend/vite.config.ts frontend/nginx.conf frontend/Dockerfile \
  docker-compose.yml .env.example
git commit -m "feat(frontend): render the basemap with empty data layers and serve the MapLibre worker"
```

---

### Task 6: 行政区图层：悬停高亮、点击缩放、区详情面板与点击分发

**Files:**
- Create: `frontend/src/map/DistrictLayer.tsx`
- Create: `frontend/src/map/MapInteractions.tsx`
- Create: `frontend/src/components/DistrictPanel.tsx`
- Modify: `frontend/src/index.css`
- Modify: `frontend/src/App.tsx`（整体替换）

**Interfaces:**
- Consumes: `useMap`（`src/map/MapView.tsx`）；`SOURCE`、`LAYER`（`src/map/layers.ts`）；`useDistricts`、`useDistrict`、`useCategories`；`transformCollection`、`bboxOfGeometry`；`toDisplay`；`formatArea`；`FALLBACK_POI_COLOR`；`useAppStore`（`selectedDistrict`、`selectDistrict`、`openPoi`）
- Produces:
  - `DistrictLayer`：区边界和区名标注的数据、悬停状态与提示、选中状态、选中后缩放
  - `MapInteractions`：全部点击的分发（POI 聚合、单个 POI、区）和鼠标指针样式；单个 POI 的点击调用 `openPoi(id)`，Task 7 的弹窗据此显示
  - `DistrictPanel`：左上角的区详情面板

- [ ] **Step 1: 区图层组件**

创建 `frontend/src/map/DistrictLayer.tsx`：

```tsx
import type { FeatureCollection, Point } from 'geojson'
import { Popup, type GeoJSONSource } from 'maplibre-gl'
import { useEffect, useMemo } from 'react'
import { useDistricts } from '../api/queries'
import { toDisplay } from '../config'
import { bboxOfGeometry, transformCollection } from '../lib/geojson'
import { useAppStore } from '../store/useAppStore'
import { LAYER, SOURCE } from './layers'
import { useMap } from './MapView'

// 左侧面板约 300 像素宽，缩放到区时给它留出位置。
const FIT_PADDING = { top: 60, bottom: 60, left: 340, right: 60 }

export function DistrictLayer() {
  const map = useMap()
  const { data } = useDistricts()
  const selected = useAppStore((s) => s.selectedDistrict)

  const display = useMemo(() => (data ? transformCollection(data, toDisplay) : null), [data])

  useEffect(() => {
    if (!display) return
    map.getSource<GeoJSONSource>(SOURCE.districts)?.setData(display)
    const labels: FeatureCollection<Point, { name: string }> = {
      type: 'FeatureCollection',
      features: display.features.map((f) => ({
        type: 'Feature',
        properties: { name: f.properties.name },
        geometry: { type: 'Point', coordinates: toDisplay(f.properties.center) },
      })),
    }
    map.getSource<GeoJSONSource>(SOURCE.districtLabels)?.setData(labels)
  }, [map, display])

  useEffect(() => {
    const tooltip = new Popup({ closeButton: false, closeOnClick: false, offset: 14, className: 'district-tooltip' })
    let hovered: string | null = null
    const setHovered = (adcode: string | null) => {
      if (hovered) map.setFeatureState({ source: SOURCE.districts, id: hovered }, { hover: false })
      hovered = adcode
      if (hovered) map.setFeatureState({ source: SOURCE.districts, id: hovered }, { hover: true })
    }
    const move = map.on('mousemove', LAYER.districtFill, (e) => {
      const feature = e.features?.[0]
      if (!feature) return
      setHovered(String(feature.properties.adcode))
      tooltip.setLngLat(e.lngLat).setText(String(feature.properties.name)).addTo(map)
    })
    const leave = map.on('mouseleave', LAYER.districtFill, () => {
      setHovered(null)
      tooltip.remove()
    })
    return () => {
      move.unsubscribe()
      leave.unsubscribe()
      setHovered(null)
      tooltip.remove()
    }
  }, [map])

  useEffect(() => {
    if (!selected) return
    map.setFeatureState({ source: SOURCE.districts, id: selected }, { selected: true })
    return () => {
      map.setFeatureState({ source: SOURCE.districts, id: selected }, { selected: false })
    }
  }, [map, selected])

  useEffect(() => {
    const feature = display?.features.find((f) => f.properties.adcode === selected)
    if (!feature) return
    const [minLng, minLat, maxLng, maxLat] = bboxOfGeometry(feature.geometry)
    map.fitBounds(
      [
        [minLng, minLat],
        [maxLng, maxLat],
      ],
      { padding: FIT_PADDING },
    )
  }, [map, display, selected])

  return null
}
```

- [ ] **Step 2: 点击分发**

创建 `frontend/src/map/MapInteractions.tsx`：

```tsx
import type { Point } from 'geojson'
import type { GeoJSONSource } from 'maplibre-gl'
import { useEffect } from 'react'
import type { LngLat } from '../lib/coord'
import { useAppStore } from '../store/useAppStore'
import { LAYER, SOURCE } from './layers'
import { useMap } from './MapView'

const POI_LAYERS = [LAYER.poiCluster, LAYER.poiPoint]
const CLICKABLE_LAYERS = [...POI_LAYERS, LAYER.districtFill]

// 点击统一在这里分发：POI 优先于它下面的区；点到 POI 以外的地方会关闭 POI 弹窗。
export function MapInteractions() {
  const map = useMap()

  useEffect(() => {
    const click = map.on('click', (e) => {
      const { openPoi, selectDistrict } = useAppStore.getState()
      const [poi] = map.queryRenderedFeatures(e.point, { layers: POI_LAYERS })
      if (poi?.layer.id === LAYER.poiCluster) {
        const center = (poi.geometry as Point).coordinates as LngLat
        map
          .getSource<GeoJSONSource>(SOURCE.pois)
          ?.getClusterExpansionZoom(Number(poi.properties.cluster_id))
          .then((zoom) => map.easeTo({ center, zoom }))
          // 数据在点击后刷新时聚合 ID 会失效，这次点击直接放弃即可。
          .catch(() => {})
        return
      }
      if (poi) {
        openPoi(Number(poi.properties.id))
        return
      }
      openPoi(null)
      const [district] = map.queryRenderedFeatures(e.point, { layers: [LAYER.districtFill] })
      selectDistrict(district ? String(district.properties.adcode) : null)
    })
    const move = map.on('mousemove', (e) => {
      const hit = map.queryRenderedFeatures(e.point, { layers: CLICKABLE_LAYERS }).length > 0
      map.getCanvas().style.cursor = hit ? 'pointer' : ''
    })
    return () => {
      click.unsubscribe()
      move.unsubscribe()
    }
  }, [map])

  return null
}
```

- [ ] **Step 3: 区详情面板与悬停提示样式**

创建 `frontend/src/components/DistrictPanel.tsx`：

```tsx
import { useCategories, useDistrict } from '../api/queries'
import { FALLBACK_POI_COLOR } from '../lib/categories'
import { formatArea } from '../lib/format'
import { useAppStore } from '../store/useAppStore'

export function DistrictPanel() {
  const adcode = useAppStore((s) => s.selectedDistrict)
  const selectDistrict = useAppStore((s) => s.selectDistrict)
  const { data: detail } = useDistrict(adcode)
  const { data: categories } = useCategories()

  if (adcode === null) return null
  const colors = new Map(categories?.map((c) => [c.code, c.color]))
  return (
    <aside className="absolute left-4 top-4 z-10 w-72 rounded-lg bg-white/95 p-4 shadow-lg ring-1 ring-slate-200">
      <div className="flex items-start justify-between gap-2">
        <h2 className="text-lg font-semibold text-slate-900">{detail?.name ?? '加载中…'}</h2>
        <button
          type="button"
          aria-label="关闭区详情"
          onClick={() => selectDistrict(null)}
          className="rounded px-1.5 text-lg leading-none text-slate-500 hover:bg-slate-100 hover:text-slate-800"
        >
          ×
        </button>
      </div>
      {detail && (
        <>
          <p className="mt-1 text-sm text-slate-500">面积 {formatArea(detail.area_km2)}</p>
          <ul className="mt-3 space-y-1.5">
            {detail.poi_counts.map((c) => (
              <li key={c.code} className="flex items-center gap-2 text-sm">
                <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: colors.get(c.code) ?? FALLBACK_POI_COLOR }} />
                <span className="flex-1 text-slate-700">{c.name}</span>
                <span className="tabular-nums text-slate-900">{c.count}</span>
              </li>
            ))}
          </ul>
        </>
      )}
    </aside>
  )
}
```

在 `frontend/src/index.css` 末尾追加：

```css

/* 悬停提示跟随鼠标，不能挡住下面的地图，否则鼠标移到提示上会触发 mouseleave 导致闪烁。 */
.maplibregl-popup.district-tooltip {
  pointer-events: none;
}
```

- [ ] **Step 4: 页面组合**

把 `frontend/src/App.tsx` 整个替换为：

```tsx
import { DistrictPanel } from './components/DistrictPanel'
import { ErrorBanner } from './components/ErrorBanner'
import { DistrictLayer } from './map/DistrictLayer'
import { MapInteractions } from './map/MapInteractions'
import { MapView } from './map/MapView'

export default function App() {
  return (
    <main className="relative h-full w-full overflow-hidden">
      <MapView>
        <DistrictLayer />
        <MapInteractions />
      </MapView>
      <DistrictPanel />
      <div className="pointer-events-none absolute right-3 top-3 z-10 flex flex-col items-end gap-2">
        <ErrorBanner />
      </div>
    </main>
  )
}
```

- [ ] **Step 5: 运行前端检查并提交**

```bash
cd frontend && npm run lint && npm run typecheck && npm test && npm run build && cd ..
git add frontend/src/map/DistrictLayer.tsx frontend/src/map/MapInteractions.tsx frontend/src/components/DistrictPanel.tsx \
  frontend/src/index.css frontend/src/App.tsx
git commit -m "feat(frontend): add the district layer with hover, click-to-zoom and a detail panel"
```

Expected: 四条命令都没有错误或警告。

---

### Task 7: POI 图层：按视野加载、聚合、分类筛选与详情弹窗

**Files:**
- Create: `frontend/src/map/PoiLayer.tsx`
- Create: `frontend/src/map/PoiPopup.tsx`
- Create: `frontend/src/components/PoiDetailCard.tsx`
- Create: `frontend/src/components/CategoryFilter.tsx`
- Modify: `frontend/src/App.tsx`（整体替换）

**Interfaces:**
- Consumes: `useMap`；`SOURCE`、`LAYER`、`EMPTY_COLLECTION`；`useCategories`、`usePoisInBBox`、`usePoiDetail`；`viewToDataBBox`、`BBox`；`categoriesParam`、`categoryColorExpression`；`transformCollection`；`toData`、`toDisplay`；`useAppStore`（`hiddenCategories`、`toggleCategory`、`activePoiId`、`openPoi`）；`POIDetail`
- Produces: `PoiLayer`、`PoiPopup`、`PoiDetailCard`、`CategoryFilter`

- [ ] **Step 1: POI 图层组件**

创建 `frontend/src/map/PoiLayer.tsx`：

```tsx
import type { GeoJSONSource, Map as MaplibreMap } from 'maplibre-gl'
import { useEffect, useMemo, useState } from 'react'
import { useCategories, usePoisInBBox } from '../api/queries'
import { toData, toDisplay } from '../config'
import { viewToDataBBox, type BBox } from '../lib/bbox'
import { categoriesParam, categoryColorExpression } from '../lib/categories'
import { transformCollection } from '../lib/geojson'
import { useAppStore } from '../store/useAppStore'
import { EMPTY_COLLECTION, LAYER, SOURCE } from './layers'
import { useMap } from './MapView'

const MOVE_DEBOUNCE_MS = 300

function currentBBox(map: MaplibreMap): BBox {
  const b = map.getBounds()
  return viewToDataBBox({ west: b.getWest(), south: b.getSouth(), east: b.getEast(), north: b.getNorth() }, toData)
}

export function PoiLayer() {
  const map = useMap()
  const [bbox, setBbox] = useState<BBox>(() => currentBBox(map))
  const { data: categories } = useCategories()
  const hidden = useAppStore((s) => s.hiddenCategories)

  const codes = useMemo(() => categories?.map((c) => c.code) ?? [], [categories])
  const requested = categoriesParam(codes, hidden)
  const { data } = usePoisInBBox(bbox, requested)

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined
    const moveEnd = map.on('moveend', () => {
      clearTimeout(timer)
      timer = setTimeout(() => setBbox(currentBBox(map)), MOVE_DEBOUNCE_MS)
    })
    return () => {
      clearTimeout(timer)
      moveEnd.unsubscribe()
    }
  }, [map])

  useEffect(() => {
    if (categories) map.setPaintProperty(LAYER.poiPoint, 'circle-color', categoryColorExpression(categories))
  }, [map, categories])

  const display = useMemo(() => (data ? transformCollection(data, toDisplay) : EMPTY_COLLECTION), [data])

  useEffect(() => {
    map.getSource<GeoJSONSource>(SOURCE.pois)?.setData(display)
  }, [map, display])

  if (!data?.truncated) return null
  return (
    <div className="pointer-events-none absolute left-1/2 top-4 z-10 -translate-x-1/2 rounded-full bg-amber-50 px-4 py-1.5 text-sm text-amber-800 shadow ring-1 ring-amber-200">
      当前范围数据过多，请放大地图
    </div>
  )
}
```

分类全部关闭时 `usePoisInBBox` 不请求，也不沿用上一次的结果，`data` 为 `undefined`，图层随之清空。

- [ ] **Step 2: POI 详情卡片与弹窗**

创建 `frontend/src/components/PoiDetailCard.tsx`：

```tsx
import type { POIDetail } from '../api/types'

export function PoiDetailCard({ poi }: { poi: POIDetail }) {
  return (
    <div className="space-y-1.5 pr-4 text-sm text-slate-700">
      <div>
        <h3 className="text-base font-semibold text-slate-900">{poi.name}</h3>
        {poi.name_en && <p className="text-xs text-slate-500">{poi.name_en}</p>}
      </div>
      <p className="flex items-center gap-1.5">
        <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: poi.category.color }} />
        {poi.category.name}
      </p>
      <p>所在区：{poi.district ? poi.district.name : '不在区界数据范围内'}</p>
      <a href={poi.osm_url} target="_blank" rel="noreferrer" className="inline-block text-blue-600 hover:underline">
        在 OpenStreetMap 中查看
      </a>
    </div>
  )
}
```

创建 `frontend/src/map/PoiPopup.tsx`：

```tsx
import { Popup } from 'maplibre-gl'
import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { usePoiDetail } from '../api/queries'
import { PoiDetailCard } from '../components/PoiDetailCard'
import { toDisplay } from '../config'
import { useAppStore } from '../store/useAppStore'
import { useMap } from './MapView'

// 弹窗内容由 React 渲染到一个固定的 div 里，再把这个 div 交给 MapLibre 的 Popup。
export function PoiPopup() {
  const map = useMap()
  const id = useAppStore((s) => s.activePoiId)
  const { data: poi } = usePoiDetail(id)
  const [container] = useState(() => document.createElement('div'))

  useEffect(() => {
    if (!poi) return
    const popup = new Popup({ closeOnClick: false, offset: 10, maxWidth: '300px' })
      .setLngLat(toDisplay(poi.location))
      .setDOMContent(container)
      .addTo(map)
    const onClose = () => useAppStore.getState().openPoi(null)
    popup.on('close', onClose)
    return () => {
      popup.off('close', onClose)
      popup.remove()
    }
  }, [map, poi, container])

  return poi ? createPortal(<PoiDetailCard poi={poi} />, container) : null
}
```

`closeOnClick: false` 是必要的：MapLibre 的「点击地图关闭弹窗」监听器注册得比 `MapInteractions` 晚，点击另一个 POI 时会先打开新弹窗、再把它关掉。点击空白处关闭弹窗由 `MapInteractions` 负责。清理时先移除 `close` 监听器再关闭弹窗，否则切换到另一个 POI 时会把刚打开的那个清掉。

- [ ] **Step 3: 分类开关**

创建 `frontend/src/components/CategoryFilter.tsx`：

```tsx
import { useCategories } from '../api/queries'
import { useAppStore } from '../store/useAppStore'

export function CategoryFilter() {
  const { data: categories } = useCategories()
  const hidden = useAppStore((s) => s.hiddenCategories)
  const toggleCategory = useAppStore((s) => s.toggleCategory)

  if (!categories) return null
  return (
    <fieldset className="pointer-events-auto rounded-lg bg-white/95 p-2 shadow-lg ring-1 ring-slate-200">
      <legend className="sr-only">POI 分类</legend>
      {categories.map((c) => {
        const visible = !hidden.includes(c.code)
        return (
          <label key={c.code} className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm hover:bg-slate-100">
            <input
              type="checkbox"
              checked={visible}
              onChange={() => toggleCategory(c.code)}
              style={{ accentColor: c.color }}
            />
            <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: c.color }} />
            <span className={visible ? 'text-slate-800' : 'text-slate-400'}>{c.name}</span>
          </label>
        )
      })}
    </fieldset>
  )
}
```

- [ ] **Step 4: 页面组合**

把 `frontend/src/App.tsx` 整个替换为：

```tsx
import { CategoryFilter } from './components/CategoryFilter'
import { DistrictPanel } from './components/DistrictPanel'
import { ErrorBanner } from './components/ErrorBanner'
import { DistrictLayer } from './map/DistrictLayer'
import { MapInteractions } from './map/MapInteractions'
import { MapView } from './map/MapView'
import { PoiLayer } from './map/PoiLayer'
import { PoiPopup } from './map/PoiPopup'

export default function App() {
  return (
    <main className="relative h-full w-full overflow-hidden">
      <MapView>
        <DistrictLayer />
        <PoiLayer />
        <MapInteractions />
        <PoiPopup />
      </MapView>
      <DistrictPanel />
      <div className="pointer-events-none absolute right-3 top-3 z-10 flex flex-col items-end gap-2">
        <ErrorBanner />
        <CategoryFilter />
      </div>
    </main>
  )
}
```

- [ ] **Step 5: 运行前端检查并提交**

```bash
cd frontend && npm run lint && npm run typecheck && npm test && npm run build && cd ..
git add frontend/src/map/PoiLayer.tsx frontend/src/map/PoiPopup.tsx frontend/src/components/PoiDetailCard.tsx \
  frontend/src/components/CategoryFilter.tsx frontend/src/App.tsx
git commit -m "feat(frontend): add the POI layer with clustering, category filter and detail popup"
```

Expected: 四条命令都没有错误或警告。

---

### Task 8: 文档

**Files:**
- Modify: `docs/02-technical-design.md`（第 2 节、4.4 节、6.1、6.3、6.4、6.5 节，新增 6.6 节，7.2 节）
- Modify: `README.md`（状态说明）

**Interfaces:**
- Consumes: Task 1 到 Task 7 的实现
- Produces: 与代码一致的技术方案和 README

下面每一处原文都只出现一次；如果找不到完全一致的原文，停下来报告 NEEDS_CONTEXT，并附上实际的上下文。

- [ ] **Step 1: 第 2 节仓库结构**

把：

```text
    └── src/
        ├── main.tsx / App.tsx
        ├── api/                # fetch 封装、类型定义、TanStack Query hooks
        ├── lib/                # coord.ts、geojson.ts、bbox.ts 等纯函数
        ├── map/                # 底图样式、地图实例 hook、各图层
        ├── store/              # Zustand 全局状态
        ├── hooks/              # useGeolocation、useDebounce
        └── components/         # 搜索框、分类筛选、面板、按钮、弹窗
```

替换为：

```text
    └── src/
        ├── main.tsx / App.tsx
        ├── config.ts           # 底图地址、坐标转换函数、初始视野和地图可视范围
        ├── api/                # fetch 封装、接口类型、TanStack Query hooks
        ├── lib/                # coord.ts、geojson.ts、bbox.ts、tiles.ts 等纯函数
        ├── map/                # 地图实例、图层定义、各图层组件、点击分发、POI 弹窗
        ├── store/              # Zustand 全局状态
        ├── hooks/              # useGeolocation、useDebounce（阶段 4）
        └── components/         # 分类开关、区详情面板、POI 详情卡片、错误提示条
```

- [ ] **Step 2: 4.4 节共享样例**

把：

```text
样例文件中每条记录包含名称、WGS-84 坐标和 GCJ-02 坐标，覆盖天安门、鸟巢、北京大学、首都机场，以及一个中国境外的点。
```

替换为：

```text
样例文件中每条记录包含名称、WGS-84 坐标和 GCJ-02 坐标，覆盖天安门、鸟巢、北京大学、首都机场，以及东、西、南、北四个方向的中国境外点（东京、塔什干、雅加达、雅库茨克）。前端的反向转换与 Go 的循环逐步一致：每轮都调用带境外判断的正向转换。
```

- [ ] **Step 3: 6.1 节 store 字段**

把：

```text
| `enabledCategories` | 当前开启的 POI 分类 |
| `selectedDistrict` | 当前选中的区 |
| `myLocation` | 我的位置，格式为 `{lng, lat, accuracy}`，WGS-84 |
| `nearby` | 附近查询的中心和半径，格式为 `{center, radius}` 或 `null` |
| `mode` | 当前模式：`normal`（普通）或 `pickLocation`（在地图上点选位置） |
| `activePoiId` | 当前打开详情弹窗的 POI |
```

替换为：

```text
| `hiddenCategories` | 当前关闭的 POI 分类编码，为空表示全部开启。记录关闭的而不是开启的分类，这样分类列表加载完成之前也有确定的状态 |
| `selectedDistrict` | 当前选中的区（`adcode`） |
| `activePoiId` | 当前打开详情弹窗的 POI |
| `errorMessage` | 右上角提示条显示的错误信息 |
| `myLocation` | 我的位置，格式为 `{lng, lat, accuracy}`，WGS-84（阶段 4） |
| `nearby` | 附近查询的中心和半径，格式为 `{center, radius}` 或 `null`（阶段 4） |
| `mode` | 当前模式：`normal`（普通）或 `pickLocation`（在地图上点选位置）（阶段 4） |
```

- [ ] **Step 4: 6.3 节图层说明**

把：

```text
所有数据在调用 `setData` 之前都要经过 `toDisplay()`，由 `lib/geojson.ts` 递归转换 GeoJSON 中的每个坐标。
```

替换为：

```text
所有数据在调用 `setData` 之前都要经过 `toDisplay()`，由 `lib/geojson.ts` 递归转换 GeoJSON 中的每个坐标。

所有数据源和图层在地图的 `load` 事件里一次建好（数据先为空），保证从下到上的顺序固定，之后只调用 `setData` 和 `setPaintProperty`。区名标注使用单独的点数据源，由 `/districts` 返回的 `center` 生成。样式中不配置 `glyphs`：MapLibre 6 在没有字形地址时用浏览器本地字体绘制文字，区名和聚合数字都不依赖外部字形服务。
```

- [ ] **Step 5: 6.4 节交互流程**

把：

```text
1. 页面首次加载时请求一次 `/districts`，结果长期缓存。
2. 鼠标移入时设置 `hover` 状态并显示区名提示。
3. 点击时用 `fitBounds` 缩放到该区，然后请求 `/districts/{adcode}`，在面板中展示面积和各分类数量。
```

替换为：

```text
1. 页面首次加载时请求一次 `/districts`，结果长期缓存。
2. 鼠标移入时设置 `hover` 状态，并在鼠标旁显示区名提示。
3. 点击时设置 `selected` 状态，用 `fitBounds` 缩放到该区（左侧给面板留出边距），然后请求 `/districts/{adcode}`，在面板中展示面积和各分类数量。点击北京以外的地方会取消选中。
```

把：

```text
1. 地图 `moveend` 事件触发后防抖 300ms。
2. 取当前视野范围，经过 `toData()` 转换后请求 `/pois`。
3. 请求参数（bbox 和分类）作为 TanStack Query 的缓存键，视野回到之前的位置时直接使用缓存。
4. 点击聚合圆时调用 `getClusterExpansionZoom` 放大。
5. 点击单个点时请求 `/pois/{id}`，并在 maplibre `Popup` 中通过 React portal 渲染详情。
6. 结果被截断时（`truncated`），提示「当前范围数据过多，请放大地图」。
```

替换为：

```text
1. 地图 `moveend` 事件触发后防抖 300ms。
2. 取当前视野的四个角，分别经过 `toData()` 转换后取外包框，向外取整到 0.001°（约 100 米），再请求 `/pois`。`limit` 固定为 5000：当前约 4100 个 POI，全市视野也不会被截断，聚合数字才准确。
3. 请求参数（bbox 和分类）作为 TanStack Query 的缓存键，视野回到之前的位置时直接使用缓存；换视野时先保留旧数据，新数据到达后再替换。分类全部关闭时不发请求，直接清空图层。
4. 点击统一由一个处理函数分发：先查 POI 图层，再查区图层，所以点击区内的 POI 不会同时选中区。点击聚合圆时调用 `getClusterExpansionZoom` 放大；点到 POI 以外的地方会关闭详情弹窗。
5. 点击单个点时请求 `/pois/{id}`，并在 maplibre `Popup` 中通过 React portal 渲染详情：名称、英文名、分类、所在区和 OSM 链接。地址由 OSM 标签拼接而成，质量参差不齐，第一版不显示。
6. 结果被截断时（`truncated`），提示「当前范围数据过多，请放大地图」。
```

- [ ] **Step 6: 6.5 节错误提示，新增 6.6 节**

把：

```text
界面统一把错误显示为右上角的提示条，不会白屏。
```

替换为：

```text
所有查询的错误由 `QueryCache` 的 `onError` 统一写入 store，界面在右上角（分类开关上方）显示提示条，不会白屏。

### 6.6 地图工程细节

- **Web Worker 地址**：MapLibre 6 在运行时根据 `import.meta.url` 拼出 worker 文件的地址，打包后这个文件并不存在。因此用 `import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url'` 让 Vite 输出 worker 文件，再调用 `setWorkerUrl(workerUrl)`。
- **`.mjs` 的 MIME 类型**：nginx 自带的 `mime.types` 不认识 `.mjs`，会按 `application/octet-stream` 返回，浏览器拒绝把它当作模块加载。`nginx.conf` 在 `/assets/` 下为 `.mjs` 单独声明 `application/javascript`。
- **可视范围**：地图的 `maxBounds` 是北京市边界外扩约 1° 的范围（经度 114.4 到 118.5，纬度 38.4 到 42.1，WGS-84，换算到底图坐标系后使用）。导入器用同一个范围过滤 POI，见 5.5 节。
- **地址栏中的地图位置**：开启 MapLibre 的 `hash` 选项，地图位置写在地址栏中（`#缩放级别/纬度/经度`，底图坐标系），刷新或分享链接时回到同一视野。开发模式下 React StrictMode 会把地图创建两次，第一次销毁时会清掉地址里的位置，所以开发模式下打开带位置的链接不生效，生产构建不受影响。
- **容器尺寸**：`maplibre-gl.css` 会给地图容器加上 `position: relative`，所以容器用 `h-full w-full` 撑满父元素，而不是 `absolute inset-0`。
```

- [ ] **Step 7: 7.2 节前端测试**

把：

```text
| Vitest 单元测试 | `lib/coord.ts`（读取同一份共享样例）、`lib/geojson.ts` 的递归坐标转换、`lib/bbox.ts`、`api/client.ts` 的参数拼接和错误解析 |
```

替换为：

```text
| Vitest 单元测试 | `lib/coord.ts`（读取同一份共享样例）、`lib/geojson.ts` 的递归坐标转换、`lib/bbox.ts`、`lib/tiles.ts`、`lib/categories.ts`、`lib/format.ts`、`store/useAppStore.ts`、`api/client.ts` 的参数拼接和错误解析、`api/queries.ts` 的查询键和跳过条件（用 TanStack Query 的 `QueryObserver` 在 Node 中运行）。地图图层组件依赖 WebGL，不写单元测试，逻辑尽量放进上面这些纯函数 |
```

- [ ] **Step 8: README 状态说明**

在 `README.md` 中，把：

```text
> **当前状态：开发中。** 后端 API 已经完成，9 个接口都可以通过 `/api/v1` 访问，说明见[技术方案](docs/02-technical-design.md#54-接口清单)。地图前端按阶段推进中，进度见[实现文档](docs/01-implementation-plan.md#5-阶段计划)。
```

替换为：

```text
> **当前状态：开发中。** 后端 API 和地图前端的行政区、POI 图层已经完成：打开页面可以看到 16 个区的边界和按视野加载、聚合显示的 POI，支持悬停高亮、点击查看区详情、分类筛选和 POI 详情。定位、搜索和附近查询在阶段 4 实现，进度见[实现文档](docs/01-implementation-plan.md#5-阶段计划)。
```

- [ ] **Step 9: 检查并提交**

```bash
grep -n 'enabledCategories' docs/02-technical-design.md   # 期望没有输出
git add docs/02-technical-design.md README.md
git commit -m "docs: describe the phase 3 map frontend"
```

---

## 阶段完成后（由协调者执行，不属于任何任务）

1. **浏览器验收（技术方案第 10 节第 1 到 5 项，以及实现文档阶段 3 的验收标准）**：执行 `FRONTEND_PORT=18080 docker compose up --build -d`（seed 会重新导入，日志中应有 `skipped_outside_region: 1`，首次运行时 `deleted_pois: 1`），用浏览器打开 `http://localhost:18080` 逐项操作并截图：
   - 全市视野：16 个区的边界与底图对齐，POI 聚合显示数字，右下角有三项数据署名。
   - 对齐抽查：天安门、鸟巢、北京大学附近放大到 15 级以上，POI 点与底图上对应的标注重合。
   - 悬停高亮和区名提示；点击海淀区后缩放，面板显示面积和各分类数量。
   - 点击聚合圆放大；关闭「医院」后医院类 POI 消失，重新打开后恢复。
   - 点击 POI，弹窗显示名称、分类、所在区和 OSM 链接；视野很小（响应小于 1 KB）时 POI 也能正常加载。
   截图转成 JPEG 存放在 `docs/screenshots/phase-3/`，并在同一目录的 `README.md` 中逐项记录验收结果，单独提交。验收完执行 `docker compose stop frontend backend`。
2. 推送分支：`git push -u origin phase-3-frontend-map`。
3. 创建 PR：`gh pr create --base main --head phase-3-frontend-map`，标题为 `阶段 3：前端地图（底图、行政区、POI 图层）`，正文写明验收标准和验证证据。
4. 等待 CI：`gh pr checks --watch`，确认 `backend`、`frontend`、`docker` 三个任务全部通过。
5. 由用户审查并合并 PR。用户合并之前，不开始阶段 4。
