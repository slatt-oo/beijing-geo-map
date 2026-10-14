# 阶段 3 验收记录

验收日期：2026-10-14。环境：`FRONTEND_PORT=18080 docker compose up --build`（生产构建，经 nginx 访问），浏览器窗口 1920×1080。导入日志：`pois: 4112`、`pois_with_district: 4098`、`skipped_outside_region: 1`。

对应实现文档第 5 节「阶段 3」的验收标准，以及技术方案第 10 节手工验证清单的第 1 到 5 项。

| 截图 | 操作 | 结果 |
|---|---|---|
| [01-overview.jpg](01-overview.jpg) | 打开首页 | 显示北京全市，16 个区的边界与底图上的行政区划线重合；POI 聚合显示数量；右上角是分类开关，右下角是三项数据署名；没有错误提示 |
| [02-align-tiananmen.jpg](02-align-tiananmen.jpg) | 放大到天安门（16.3 级） | 地铁站点落在底图的「天安门西」「天安门东」图标上，景点落在国家博物馆、国家大剧院、天安门广场和毛主席纪念堂内；东城区与西城区的边界沿广场西侧路 |
| [03-align-birds-nest.jpg](03-align-birds-nest.jpg) | 放大到鸟巢（16.3 级） | 景点点落在体育场内「鸟巢」标注旁和「北京奥运博物馆」上 |
| [04-align-pku.jpg](04-align-pku.jpg) | 放大到北京大学（15.8 级） | 高校点落在「北京大学」标注旁，地铁站点落在「北京大学东门」站，医院点落在校医院 |
| [05-hover-haidian.jpg](05-hover-haidian.jpg) | 鼠标移到海淀区 | 海淀区高亮，鼠标旁显示区名「海淀区」 |
| [06-district-panel.jpg](06-district-panel.jpg) | 点击海淀区 | 地图缩放到海淀区（左侧给面板留出边距），面板显示面积 430.95 平方千米和五类 POI 的数量 |
| [07-cluster-expand.jpg](07-cluster-expand.jpg) | 点击数量为 108 的聚合圆 | 地图以聚合点为中心放大，聚合拆分为更小的聚合和单个点 |
| [08-filter-hospital-off.jpg](08-filter-hospital-off.jpg) | 关闭「医院」 | 医院点消失，请求参数为 `categories=subway_station,university,attraction,park`；重新打开后医院点恢复 |
| [09-poi-popup.jpg](09-poi-popup.jpg) | 点击单个 POI | 弹窗显示名称「屯佃」、英文名、分类、所在区「海淀区」和 OSM 链接；区的选中状态不变 |

其他检查（未截图）：

- 视野很小、响应小于 1 KB 时（例如 16 级的天安门、鸟巢），POI 正常加载，响应头中没有 `Content-Encoding: gzip`。
- 点击另一个 POI 时弹窗切换为新的 POI，页面上始终只有一个弹窗；点击北京以外的地方，弹窗关闭，区详情面板也关闭。
- 五类分类全部关闭时图层清空，最终状态不再发出请求；被前端取消的中间请求在后端访问日志中记为 499，没有错误日志。
