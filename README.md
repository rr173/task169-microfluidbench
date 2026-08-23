# task169-microfluidbench — 微流控芯片通道拓扑校验台

微流控芯片设计工程师在网页审查通道、阀门、入口、出口与反应腔的连接关系，确认样本、试剂和清洗液可以按设计流动且不会穿越隔离区。工程师绘制或导入芯片拓扑，声明实验流程中的流体类别与阀门状态序列；系统对每一步计算可达通道、死腔、交叉污染路径和无法清洗的残留腔。工程师针对问题调整拓扑或隔离声明，复核通过后冻结一份可制造的拓扑审查快照。

## 业务闭环

1. **芯片拓扑与版本**：创建芯片与多版本设计，版本状态机 `editing -> pending -> risk_found -> approved -> superseded`。
2. **阀门状态流程**：为每个流程步骤声明流体类别（样本/试剂/清洗液）与全部阀门的开关状态。
3. **流体校验**：可达性分析（BFS）、阻断边、死腔、交叉污染、隔离区穿越、清洗残留。
4. **风险处置**：校验结果自动生成风险，工程师确认/解决/豁免。
5. **制造快照**：已批准版本可冻结快照，保存拓扑哈希与阀门状态表。

## 关键约束

- 同端口禁止重复方向；边引用不存在的节点被拒绝；流程步骤必须声明全部阀门状态。
- 边编辑采用乐观版本控制（rev），后提交者冲突。
- 跨隔离区未声明的流路被拒绝；已冻结版本不可改边，只能派生新版本。

## 标准命令

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/microfluidbench --smoke-test
```

## API 入口（前缀 /api）

- 芯片与版本：`POST/GET /api/chips`、`GET /api/chips/{id}`、`POST /api/chips/{id}/versions`、`GET /api/chips/{id}/versions`、`GET /api/versions/{id}`、`POST /api/versions/{id}/transition`
- 节点：`POST/GET /api/versions/{id}/nodes`、`GET/PUT/DELETE /api/nodes/{id}`
- 边：`POST/GET /api/versions/{id}/edges`、`GET/DELETE /api/edges/{id}`
- 隔离区：`POST/GET /api/versions/{id}/zones`、`DELETE /api/zones/{id}`
- 流程步骤：`POST/GET /api/versions/{id}/steps`、`GET /api/steps/{id}`、`PUT /api/steps/{id}/valves`
- 校验：`POST /api/steps/{id}/validate`、`POST /api/versions/{id}/validate-all`、`GET /api/versions/{id}/validations`、`GET /api/validations/{id}`
- 风险：`POST /api/validations/{id}/risks`、`GET /api/versions/{id}/risks`、`GET /api/risks/{id}`、`PUT /api/risks/{id}/transit`、`POST /api/versions/{id}/risks`
- 快照：`POST/GET /api/versions/{id}/snapshots`、`GET /api/snapshots/{id}`
- 联动与统计：`GET /api/versions/{id}/graph`、`GET /api/health`、`GET /api/stats`

Web 画布页面：`GET /`（拓扑画布 + 流程步骤 + 风险联动）。

## 持久化

SQLite（modernc.org/sqlite v1.52.0，纯 Go，CGO 无关）。保存拓扑节点/边、阀门序列、隔离区、流程快照、风险证据与审查决定；重启后从最后校验步骤恢复。

## 环境

- Go 1.26.3（GOTOOLCHAIN=local，CGO_ENABLED=0）
- SQLite 3.46.1（见 component-versions.json）
