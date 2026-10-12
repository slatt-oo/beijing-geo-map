# beijing-geo-map

[![CI](https://github.com/slatt-oo/beijing-geo-map/actions/workflows/ci.yml/badge.svg)](https://github.com/slatt-oo/beijing-geo-map/actions/workflows/ci.yml)

一个基于 PostgreSQL + PostGIS、Go 和 React + MapLibre GL 的北京市地图 Demo：渲染北京 16 个区的边界，并支持 POI 标注、浏览器定位与「我在哪个区」、名称搜索和附近查询。

> **当前状态：开发中。** 项目骨架已经可以一键启动，地图功能按阶段推进中，进度见[实现文档](docs/01-implementation-plan.md#5-阶段计划)。

## 功能（第一版）

- **行政区边界**：16 个区的边界，悬停高亮，点击查看面积和各类 POI 数量。
- **我的位置**：浏览器定位，并反查所在的区；定位失败时可以在地图上点选。
- **POI 标注**：地铁站、高校、景点/博物馆、医院、公园五类，按视野加载、聚合、分类筛选。
- **搜索**：按名称模糊搜索，选中后地图飞到该点。
- **附近查询**：以任意一点为中心，查询 N 米内的 POI，按距离排序。

## 技术栈

| 层 | 选型 |
|---|---|
| 数据库 | PostgreSQL 16 + PostGIS 3.4、pg_trgm |
| 后端 | Go 1.26、Gin、pgx v5、golang-migrate |
| 前端 | React 19、TypeScript 6、Vite 8、MapLibre GL JS、TanStack Query、Zustand、Tailwind CSS 4 |
| 运行与 CI | Docker Compose、GitHub Actions |

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

## 文档

1. [实现文档](docs/01-implementation-plan.md)：目标、功能范围、数据来源、阶段计划、风险。
2. [技术方案](docs/02-technical-design.md)：架构、坐标系方案、后端接口、导入器、前端、测试与 CI。
3. [数据库设计](docs/03-database-design.md)：ER 图、表结构、索引、典型 SQL、设计取舍、扩展表设计。

## 数据来源与声明

- POI 数据 © [OpenStreetMap](https://www.openstreetmap.org/copyright) contributors，遵循 ODbL 协议。
- 行政区划边界来自 [阿里云 DataV.GeoAtlas](https://datav.aliyun.com/portal/school/atlas/area_selector)。
- 底图 © 高德地图。本项目使用的高德瓦片地址不是官方开放接口，仅用于学习和非商业演示。

## 许可

代码以 [MIT](LICENSE) 协议发布。数据遵循各自来源的许可协议。
