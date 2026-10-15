# 阶段 5 验收记录

验收日期：2026-10-15。对应实现文档第 5 节「阶段 5」的验收标准：在一台只装了 Docker 的机器上，clone 仓库后执行 `docker compose up --build` 就能完整使用。

**从零 clone 启动：** 从 GitHub 把 `phase-5-polish` 分支（`21b13b7`）clone 到一个全新的目录，只用 Docker 构建和启动，不使用本机的任何构建缓存：`docker compose -p bgm-clean build --no-cache`，再 `docker compose -p bgm-clean up -d`。本机 8080 和 5432 端口被占用，所以用 `FRONTEND_PORT=18090`、`DB_PORT=15432` 和独立的 compose 项目名，与开发用的那套服务互不影响。构建约 1 分 30 秒，启动约 20 秒；导入日志：`districts: 16`、`pois: 4112`、`pois_with_district: 4098`、`skipped_outside_region: 1`，与前几个阶段相同。浏览器窗口 1920×1080（Cursor 内置浏览器，基于 Chromium）。

**模拟方式：** 慢网络用 Chrome DevTools Protocol 的 `Network.emulateNetworkConditions` 给每个请求加 8 秒延迟；底图服务不可用用 `Network.setBlockedURLs` 屏蔽高德瓦片域名；WebGL 不可用是在页面加载前把 `canvas.getContext('webgl…')` 改成返回 `null`。代码没有为验收做任何改动。

| 截图 | 操作 | 结果 |
|---|---|---|
| [01-fresh-clone-overview.jpg](01-fresh-clone-overview.jpg) | 从零 clone、构建并启动后打开首页 | 16 个区的边界、POI 聚合、分类开关和三项数据署名正常显示，没有错误提示 |
| [02-fresh-clone-search-nearby.jpg](02-fresh-clone-search-nearby.jpg) | 搜索「北京大学」并回车；在北大附近右键，选择「以此为中心查附近」 | 搜索返回 10 条结果，地图飞到北京大学并打开弹窗；附近面板列出 1 千米内的 14 个地点，按距离排序 |
| [03-backend-down.jpg](03-backend-down.jpg) | 第 13 项：`docker compose stop backend` 后刷新页面 | 右上角提示「后端服务未启动或暂时不可用」，带「重试」按钮；区边界、POI 和分类开关都没有加载 |
| （同 01） | `docker compose start backend` 后点击「重试」 | 不刷新页面，区边界、POI 和分类开关全部恢复，画面与 01 相同 |
| [04-slow-poi-loading.jpg](04-slow-poi-loading.jpg) | 每个请求延迟 8 秒，把地图移到城区 | 顶部中间显示「正在加载 POI…」，新数据到达前保留旧的 POI |
| [05-basemap-error.jpg](05-basemap-error.jpg) | 屏蔽高德瓦片域名后，打开一个之前没有访问过的视野 | 右上角提示「底图加载失败…」，没有「重试」按钮；底图只剩放大的低级别缓存和空白，POI 照常显示 |
| [06-webgl-error.jpg](06-webgl-error.jpg) | 让浏览器创建不了 WebGL 上下文，打开首页 | 显示「地图无法显示」、中文说明、MapLibre 的原始错误信息和「刷新页面」按钮，而不是白屏 |

其他检查（未截图）：

- 首次加载时，「正在加载 POI…」直到状态元素挂载 400 毫秒后才可能出现（这次全市第一次请求刚好超过 400 毫秒，提示只闪现了十几毫秒）。
- 生产镜像的响应头：`index.html` 为 `Cache-Control: no-cache`；`/assets/` 下的文件只有一条 `Cache-Control: public, max-age=31536000, immutable`，压缩时带 `Vary: Accept-Encoding`；MapLibre 的 worker 文件 `Content-Type` 为 `application/javascript`。
- `VITE_TILE_CRS=bd09 npm run build` 构建失败，输出 `VITE_TILE_CRS 只能是 gcj02 或 wgs84，当前为「bd09」`。
- 搜索「%」得到空结果后再输入新词，下拉框直接从「没有找到相关地点」变成「搜索中…」，再显示新结果（在原型中验证）。
