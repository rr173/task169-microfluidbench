// Package model 定义微流控芯片通道拓扑校验台的核心实体、枚举与共享错误。
package model

// NodeKind 描述芯片拓扑节点的物理类型。
type NodeKind string

const (
	NodeInlet        NodeKind = "inlet"        // 入口：样本/试剂/清洗液注入点
	NodeOutlet       NodeKind = "outlet"       // 出口：废液排出点
	NodeValve        NodeKind = "valve"        // 阀门：可开闭，控制流路通断
	NodeChannel      NodeKind = "channel"      // 通道：流体输运管道
	NodeReactionWell NodeKind = "reaction_well" // 反应腔：样本与试剂发生反应的腔室
	NodeWasteWell    NodeKind = "waste_well"   // 废液腔：收集废弃流体
)

// Direction 描述边的方向语义。
type Direction string

const (
	DirectionOneWay Direction = "one_way" // 单向：仅允许 from -> to
	DirectionTwoWay Direction = "two_way" // 双向：允许 from <-> to
)

// EdgePort 表示边挂载在节点上的端口（同一端口禁止重复方向）。
type EdgePort string

const (
	PortA EdgePort = "A"
	PortB EdgePort = "B"
)

// FluidType 描述一次流程步骤所运输的流体类别。
type FluidType string

const (
	FluidSample   FluidType = "sample"   // 样本
	FluidReagent  FluidType = "reagent"  // 试剂
	FluidRinse    FluidType = "rinse"    // 清洗液
	FluidMixture  FluidType = "mixture"  // 混合液（样本+试剂反应产物）
)

// ValveState 描述阀门在某个流程步骤中的开关状态。
type ValveState string

const (
	ValveOpen   ValveState = "open"
	ValveClosed ValveState = "closed"
)

// 芯片版本状态机：编辑中 -> 待校验 -> 风险发现 <-> 待校验 -> 已批准 -> 已替代。
const (
	VersionEditing    = "editing"     // 编辑中：允许改边
	VersionPending    = "pending"     // 待校验：冻结边编辑，等待完整校验
	VersionRiskFound  = "risk_found"  // 风险发现：校验发现风险，回编辑
	VersionApproved   = "approved"    // 已批准：全部校验通过
	VersionSuperseded = "superseded"  // 已替代：被派生版本取代
)

// 流程步骤状态机：草拟 -> 可执行 -> 受阻 -> 已验证。
const (
	StepDraft     = "draft"
	StepRunnable  = "runnable"
	StepBlocked   = "blocked"
	StepVerified  = "verified"
)

// 风险状态：新建 -> 已确认 -> 已解决 / 已豁免。
const (
	RiskNew      = "new"
	RiskConfirmed = "confirmed"
	RiskResolved = "resolved"
	RiskWaived   = "waived"
)

// 审查快照状态：构建中 -> 已冻结。
const (
	SnapshotBuilding = "building"
	SnapshotFrozen   = "frozen"
)

// Chip 是一个微流控芯片设计的主体，包含其全部版本。
type Chip struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   Time `json:"created_at"`
	UpdatedAt   Time `json:"updated_at"`
}

// ChipVersion 是一个芯片的某个设计版本。
type ChipVersion struct {
	ID         int64     `json:"id"`
	ChipID     int64     `json:"chip_id"`
	VersionNo  int       `json:"version_no"`
	Status     string    `json:"status"`
	Rev        int       `json:"rev"` // 乐观版本号：每次边编辑自增
	Note       string    `json:"note"`
	CreatedAt  Time `json:"created_at"`
	UpdatedAt  Time `json:"updated_at"`
}

// Node 是拓扑中的一个节点。
type Node struct {
	ID        int64     `json:"id"`
	VersionID int64     `json:"version_id"`
	Kind      NodeKind  `json:"kind"`
	Name      string    `json:"name"`
	X         float64   `json:"x"` // 画布坐标（仅用于展示）
	Y         float64   `json:"y"`
	CreatedAt Time `json:"created_at"`
}

// Edge 是拓扑中的一条通道边。
type Edge struct {
	ID           int64     `json:"id"`
	VersionID    int64     `json:"version_id"`
	FromNodeID   int64     `json:"from_node_id"`
	FromPort     EdgePort  `json:"from_port"`
	ToNodeID     int64     `json:"to_node_id"`
	ToPort       EdgePort  `json:"to_port"`
	Direction    Direction `json:"direction"`
	Width        float64   `json:"width"` // 通道宽度（um）
	Comment      string    `json:"comment"`
	CreatedAt    Time `json:"created_at"`
}

// IsolationZone 是隔离区：其成员节点不允许流体自由穿越，
// 除非通过显式声明的交叉流路（declared_crossings）。
type IsolationZone struct {
	ID                int64     `json:"id"`
	VersionID         int64     `json:"version_id"`
	Name              string    `json:"name"`
	Reason            string    `json:"reason"`
	DeclaredCrossings string    `json:"declared_crossings"` // 逗号分隔的 "nodeA|nodeB" 声明流路
	CreatedAt         Time `json:"created_at"`
}

// ZoneMember 是隔离区与节点的关联。
type ZoneMember struct {
	ZoneID int64 `json:"zone_id"`
	NodeID int64 `json:"node_id"`
}

// ValveCommand 是单个阀门在步骤中的命令。
type ValveCommand struct {
	NodeID int64      `json:"node_id"`
	State  ValveState `json:"state"`
}

// FlowStep 是一次实验流程步骤。
type FlowStep struct {
	ID          int64           `json:"id"`
	VersionID   int64           `json:"version_id"`
	OrderNo     int             `json:"order_no"`
	FluidType   FluidType       `json:"fluid_type"`
	InletID     int64           `json:"inlet_id"`
	OutletID    int64           `json:"outlet_id"`
	Status      string          `json:"status"`
	Valves      []ValveCommand  `json:"valves"` // 必须覆盖该步骤途经的所有阀门
	CreatedAt   Time       `json:"created_at"`
	UpdatedAt   Time       `json:"updated_at"`
}

// ValidationResult 是一次流程步骤的校验输出。
type ValidationResult struct {
	ID               int64     `json:"id"`
	StepID           int64     `json:"step_id"`
	VersionID        int64     `json:"version_id"`
	Passed           bool      `json:"passed"`
	Reachable        bool      `json:"reachable"`
	ReachablePath    []int64   `json:"reachable_path"`   // 找到的可达路径（节点 ID 序列）
	BlockedEdges     []int64   `json:"blocked_edges"`    // 阻断边（阀门关闭导致）
	DeadVolumes      []int64   `json:"dead_volumes"`     // 死腔节点（清洗液无法覆盖的支路）
	CrossContamEdges []int64   `json:"cross_contam_edges"` // 交叉污染路径
	ResidualWells    []int64   `json:"residual_wells"`   // 无法清洗的残留腔
	IsolationBreaks  []int64   `json:"isolation_breaks"` // 跨隔离区未声明流路
	Message          string    `json:"message"`
	CreatedAt        Time `json:"created_at"`
}

// Risk 是校验发现的问题记录。
type Risk struct {
	ID          int64     `json:"id"`
	VersionID   int64     `json:"version_id"`
	ValidationID int64    `json:"validation_id"`
	Kind        string    `json:"kind"` // dead_volume | cross_contamination | residual | isolation_break | unreachable
	Severity    string    `json:"severity"` // high | medium | low
	Title       string    `json:"title"`
	Evidence    string    `json:"evidence"` // 证据描述
	Status      string    `json:"status"`
	Owner       string    `json:"owner"`
	Resolution  string    `json:"resolution"`
	CreatedAt   Time `json:"created_at"`
	UpdatedAt   Time `json:"updated_at"`
}

// Snapshot 是冻结的制造审查快照。
type Snapshot struct {
	ID               int64     `json:"id"`
	VersionID        int64     `json:"version_id"`
	Status           string    `json:"status"` // building | frozen
	TopologyHash     string    `json:"topology_hash"` // 拓扑结构哈希
	ValveStateTable  string    `json:"valve_state_table"` // 阀门状态表 JSON
	NodeCount        int       `json:"node_count"`
	EdgeCount        int       `json:"edge_count"`
	FrozenBy         string    `json:"frozen_by"`
	CreatedAt        Time `json:"created_at"`
	FrozenAt         *Time `json:"frozen_at"`
}
