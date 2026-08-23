# BENZHI README — task169-microfluidbench 评测说明

微流控芯片通道拓扑校验台：设计工程师导入芯片拓扑，声明流体类别与阀门状态序列；系统对每一步计算可达通道、死腔、交叉污染路径与无法清洗的残留腔，调整后复核通过并冻结可制造快照。

## 评测命令

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/microfluidbench --smoke-test
```

## 关键 API 契约

- `POST /api/chips` — 入参 `{"name","description"}`；name 必填（≤64 字符）。
- `POST /api/chips/{id}/versions` — 创建新版本，version_no 自动递增，初始状态 `editing`。
- `POST /api/versions/{id}/nodes` — 入参 `{"kind","name","x","y"}`；kind ∈ inlet|outlet|valve|channel|reaction_well|waste_well。
- `POST /api/versions/{id}/edges` — 入参 `{"from_node_id","from_port","to_node_id","to_port","direction","width","expected_rev"}`；悬空端点、同端口重复方向、跨隔离区未声明流路均返回 422；乐观版本冲突返回 409。
- `POST /api/versions/{id}/steps` — 入参 `{"order_no","fluid_type","inlet_id","outlet_id","valves":[{node_id,state}]}`；阀门状态缺失返回 422（ErrValveMissing）。
- `POST /api/steps/{id}/validate` — 返回 `passed/reachable/reachable_path/blocked_edges/dead_volumes/cross_contam_edges/residual_wells/isolation_breaks/message`。
- `POST /api/validations/{id}/risks` — 由校验结果自动生成风险记录。
- `PUT /api/risks/{id}/transit` — 入参 `{"to","owner","resolution"}`；非法状态迁移返回 409。
- `POST /api/versions/{id}/snapshots` — 仅 `approved` 版本可冻结；返回拓扑哈希与阀门状态表；其他状态返回 409（ErrStateTransition）。
- `GET /api/versions/{id}/graph` — 画布联动：节点/边/隔离区/风险节点。

## Docker 双架构验证

```bash
bash build_benzhi_docker.sh task169-microfluidbench linux/amd64
docker run --rm task169-microfluidbench /app/microfluidbench --smoke-test

bash build_benzhi_docker.sh task169-microfluidbench linux/arm64
docker run --rm task169-microfluidbench /app/microfluidbench --smoke-test
```

## --smoke-test 契约

不启动长驻服务；真实执行：创建芯片与版本 → 搭建拓扑（入口/阀门/反应腔/废液腔/出口）→ 声明隔离区 → 创建样本流步骤（关闭 V1 应阻断）→ 打开 V1 复核 → 创建清洗步骤（无死腔/残留）→ 由校验生成风险 → 乐观版本冲突被拒绝 → 批准版本并冻结快照 → 冻结后改边被拒绝 → 关闭并重开数据库验证重启恢复。全部通过以退出码 0 结束，任一失败退出码 1。

## 环境

- Go 1.26.3（GOTOOLCHAIN=local，CGO_ENABLED=0）
- modernc.org/sqlite v1.52.0（SQLite 3.46.1，纯 Go 驱动，离线可构建）
