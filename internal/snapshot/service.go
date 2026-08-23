// Package snapshot 冻结制造审查快照：拓扑哈希 + 阀门状态表。
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"task169-microfluidbench/internal/flow"
	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/store"
	"task169-microfluidbench/internal/topology"
)

// Service 管理审查快照。
type Service struct {
	snaps *store.SnapshotStore
	chips *store.ChipsStore
	topo  *topology.Service
	flows *store.FlowStore
}

// NewService 构造快照服务。
func NewService(snaps *store.SnapshotStore, chips *store.ChipsStore, topo *topology.Service, flows *store.FlowStore) *Service {
	return &Service{snaps: snaps, topo: topo, flows: flows, chips: chips}
}

// BuildAndFreeze 对版本构建快照并冻结。
// 版本必须处于已批准状态；返回冻结后的快照。
func (s *Service) BuildAndFreeze(versionID int64, frozenBy string) (*model.Snapshot, error) {
	v, err := s.chips.GetVersion(versionID)
	if err != nil {
		return nil, err
	}
	if v.Status != model.VersionApproved {
		return nil, fmt.Errorf("%w: 只有已批准版本可冻结快照，当前 %s", model.ErrStateTransition, v.Status)
	}
	g, err := s.topo.LoadGraph(versionID)
	if err != nil {
		return nil, err
	}
	snap, err := s.snaps.CreateSnapshot(versionID, frozenBy)
	if err != nil {
		return nil, err
	}
	hash := s.TopologyHash(g)
	valveTable := s.ValveStateTable(g)
	if err := s.snaps.FreezeSnapshot(snap.ID, hash, valveTable, len(g.Nodes), len(g.Edges)); err != nil {
		return nil, err
	}
	return s.snaps.GetSnapshot(snap.ID)
}

// TopologyHash 计算拓扑结构哈希：节点类型/名称 + 边端点/方向/端口/宽度 排序后 SHA-256。
func (s *Service) TopologyHash(g *topology.Graph) string {
	var parts []string
	for _, n := range g.Nodes {
		parts = append(parts, fmt.Sprintf("N:%d:%s:%s", n.ID, n.Kind, n.Name))
	}
	sort.Strings(parts)
	for _, e := range g.Edges {
		parts = append(parts, fmt.Sprintf("E:%d:%d:%s:%d:%s:%s:%s:%v", e.ID, e.FromNodeID, e.FromPort, e.ToNodeID, e.ToPort, e.Direction, e.Comment, e.Width))
	}
	sort.Strings(parts)
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ValveStateTable 生成阀门状态表：nodeID -> 各步骤中的开关状态。
func (s *Service) ValveStateTable(g *topology.Graph) string {
	steps, err := s.flows.ListSteps(g.VersionID)
	if err != nil {
		return ""
	}
	table := map[int64]map[int]string{} // valveID -> {order: state}
	for _, st := range steps {
		for _, v := range st.Valves {
			if table[v.NodeID] == nil {
				table[v.NodeID] = map[int]string{}
			}
			table[v.NodeID][st.OrderNo] = string(v.State)
		}
	}
	// 排序阀门 ID 与步骤号保证稳定序列化。
	var valveIDs []int64
	for vid := range table {
		valveIDs = append(valveIDs, vid)
	}
	sort.Slice(valveIDs, func(i, j int) bool { return valveIDs[i] < valveIDs[j] })
	type row struct {
		ValveID int64             `json:"valve_id"`
		States  map[string]string `json:"states"`
	}
	var rows []row
	for _, vid := range valveIDs {
		m := map[string]string{}
		var orders []int
		for o := range table[vid] {
			orders = append(orders, o)
		}
		sort.Ints(orders)
		for _, o := range orders {
			m[fmt.Sprintf("step%d", o)] = table[vid][o]
		}
		rows = append(rows, row{ValveID: vid, States: m})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return ""
	}
	return string(b)
}

// ListByVersion 列出版本的快照。
func (s *Service) ListByVersion(versionID int64) ([]*model.Snapshot, error) {
	return s.snaps.ListByVersion(versionID)
}

// Get 读取快照。
func (s *Service) Get(id int64) (*model.Snapshot, error) {
	return s.snaps.GetSnapshot(id)
}

// EnsureValveTableStable 供外部校验阀门表可解析。
func EnsureValveTableStable(table string) bool {
	var rows []struct {
		ValveID int64             `json:"valve_id"`
		States  map[string]string `json:"states"`
	}
	return json.Unmarshal([]byte(table), &rows) == nil
}

// _ 保证 flow 包引用存在（RunnableEdges 供测试复用）。
var _ = flow.RunnableEdges
