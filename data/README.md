# 数据快照

| 文件 | 内容 | 来源 | 坐标系 | 许可 |
|---|---|---|---|---|
| `raw/beijing_districts_gcj02.json` | 北京市 16 个区的边界 | [阿里云 DataV.GeoAtlas](https://datav.aliyun.com/portal/school/atlas/area_selector)：`https://geo.datav.aliyun.com/areas_v3/bound/110000_full.json` | GCJ-02 | 数据来自高德开放平台，仅用于学习和非商业演示 |
| `raw/beijing_pois_osm.json` | 地铁站、高校、景点/博物馆、医院、公园 | [OpenStreetMap](https://www.openstreetmap.org/)，经 Overpass API 导出，内容为接口原样输出 | WGS-84 | [ODbL](https://opendatacommons.org/licenses/odbl/)，© OpenStreetMap contributors |

两份快照都拉取于 2026-10-13，POI 快照的确切数据时间见文件中的 `osm3s.timestamp_osm_base` 字段。

导入时，行政区边界会从 GCJ-02 转换为 WGS-84，POI 本来就是 WGS-84，不需要转换。分类、取名和地铁站去重规则见[技术方案](../docs/02-technical-design.md)第 5.5 节。

## 更新快照

- **POI**：运行 `make fetch-osm`（即 `scripts/fetch_osm_pois.sh`）。脚本会带上 User-Agent，失败时间隔 30、60、120 秒重试；只有返回的要素不少于 1000 个、并且响应中没有表示查询超时或内存不足的 `remark` 字段时，才会覆盖现有文件。
- **行政区边界**：

  ```bash
  curl -sSf -o data/raw/beijing_districts_gcj02.json https://geo.datav.aliyun.com/areas_v3/bound/110000_full.json
  ```

更新后运行 `make seed` 重新导入。导入是幂等的：已有记录会被更新，快照里已经没有的 POI 会被删除。

更新快照后先运行 `cd backend && go test ./internal/importer/`：`snapshot_test.go` 要求 POI 数量在 4000 到 4300 之间，OSM 数据增长后可能需要调整这个范围。
