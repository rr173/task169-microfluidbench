// Package topology 维护微流控芯片拓扑：节点、边、隔离区与乐观版本控制。
package topology

import (
	"fmt"
	"strings"

	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/store"
)

// Service 封装拓扑领域的操作。
type Service struct {
	chips *store.ChipsStore
	topo  *store.TopologyStore
}

// NewService 构造拓扑服务。
func NewService(chips *store.ChipsStore, topo *store.TopologyStore) *Service {
	return &Service{chips: chips, topo: topo}
}

// EnsureEditable 校验版本存在且处于可编辑状态；冻结/已批准版本禁止改边。
func (s *Service) EnsureEditable(versionID int64) (*model.ChipVersion, error) {
	v, err := s.chips.GetVersion(versionID)
	if err != nil {
		return nil, err
	}
	switch v.Status {
	case model.VersionEditing, model.VersionRiskFound:
		return v, nil
	case model.VersionPending, model.VersionApproved, model.VersionSuperseded:
		return nil, fmt.Errorf("%w: 版本状态 %s 不可编辑", model.ErrFrozen, v.Status)
	}
	return nil, fmt.Errorf("%w: 未知版本状态 %s", model.ErrInvalid, v.Status)
}

// CreateNode 在可编辑版本中创建节点。
func (s *Service) CreateNode(versionID int64, kind model.NodeKind, name string, x, y float64) (*model.Node, error) {
	if _, err := s.EnsureEditable(versionID); err != nil {
		return nil, err
	}
	if err := model.ValidateNodeInput(kind, name); err != nil {
		return nil, err
	}
	return s.topo.CreateNode(&model.Node{
		VersionID: versionID,
		Kind:      kind,
		Name:      strings.TrimSpace(name),
		X:         x,
		Y:         y,
	})
}

// CreateEdge 创建边：校验端点存在、端口方向唯一、隔离区声明，然后乐观版本号 +1。
func (s *Service) CreateEdge(versionID int64, e *model.Edge, expectedRev int) (*model.Edge, error) {
	v, err := s.EnsureEditable(versionID)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateEdgeInput(e.FromNodeID, e.ToNodeID, e.FromPort, e.ToPort, e.Direction, e.Width); err != nil {
		return nil, err
	}
	// 乐观版本控制：后提交者拿到旧 rev 即冲突。
	if v.Rev != expectedRev {
		return nil, fmt.Errorf("%w: 期望 rev=%d 实际 rev=%d", model.ErrConflict, expectedRev, v.Rev)
	}
	// 端点必须存在且属于本版本。
	nodes, err := s.topo.ListNodes(versionID)
	if err != nil {
		return nil, err
	}
	exist := map[int64]bool{}
	for _, n := range nodes {
		exist[n.ID] = true
	}
	if !exist[e.FromNodeID] || !exist[e.ToNodeID] {
		return nil, fmt.Errorf("%w: 端点 %d/%d 不存在于版本 %d", model.ErrDangling, e.FromNodeID, e.ToNodeID, versionID)
	}
	// 同端口重复方向：同一 (node, port) 作为同一方向终点的边只允许一条。
	edges, err := s.topo.ListEdges(versionID)
	if err != nil {
		return nil, err
	}
	for _, old := range edges {
		if samePortEnd(old, e) {
			return nil, fmt.Errorf("%w: 端口 %d:%s 已用于方向 %d->%d",
				model.ErrDuplicatePort, old.FromNodeID, old.FromPort, old.FromNodeID, old.ToNodeID)
		}
	}
	// 跨隔离区未声明流路检查。
	if err := s.checkIsolationCrossing(versionID, nodes, edges, e); err != nil {
		return nil, err
	}
	ne, err := s.topo.CreateEdge(e)
	if err != nil {
		return nil, err
	}
	_ = s.chips.BumpVersionRev(versionID)
	return ne, nil
}

// DeleteEdge 删除边并乐观版本号 +1。
func (s *Service) DeleteEdge(versionID, edgeID int64, expectedRev int) error {
	v, err := s.EnsureEditable(versionID)
	if err != nil {
		return err
	}
	if v.Rev != expectedRev {
		return fmt.Errorf("%w: 期望 rev=%d 实际 rev=%d", model.ErrConflict, expectedRev, v.Rev)
	}
	if err := s.topo.DeleteEdge(edgeID); err != nil {
		return err
	}
	return s.chips.BumpVersionRev(versionID)
}

// samePortEnd 判断两条边是否共享同一端口且方向冲突。
func samePortEnd(a, b *model.Edge) bool {
	return (a.FromNodeID == b.FromNodeID && a.FromPort == b.FromPort) ||
		(a.ToNodeID == b.ToNodeID && a.ToPort == b.ToPort)
}

// checkIsolationCrossing 检查新边是否让流体穿越隔离区而未在声明的交叉流路中。
func (s *Service) checkIsolationCrossing(versionID int64, nodes []*model.Node, edges []*model.Edge, e *model.Edge) error {
	zones, err := s.topo.ListZones(versionID)
	if err != nil {
		return err
	}
	if len(zones) == 0 {
		return nil
	}
	// 节点 -> 所属隔离区集合。
	zoneOf := map[int64]map[int64]bool{}
	for _, z := range zones {
		members, err := s.topo.ZoneMembers(z.ID)
		if err != nil {
			return err
		}
		for _, nid := range members {
			if zoneOf[nid] == nil {
				zoneOf[nid] = map[int64]bool{}
			}
			zoneOf[nid][z.ID] = true
		}
	}
	// 若新边两端在不同隔离区，必须出现在该区的 declared_crossings 中。
	zFrom := zoneOf[e.FromNodeID]
	zTo := zoneOf[e.ToNodeID]
	if len(zFrom) == 0 || len(zTo) == 0 {
		return nil
	}
	for zid := range zFrom {
		if !zTo[zid] {
			continue
		}
		// 同一区内部不构成穿越。
	}
	for zidA := range zFrom {
		for zidB := range zTo {
			if zidA == zidB {
				continue
			}
			if !declared(zones, zidA, e.FromNodeID, e.ToNodeID) &&
				!declared(zones, zidB, e.FromNodeID, e.ToNodeID) {
				return fmt.Errorf("%w: 边 %d->%d 穿越隔离区 %d/%d 未声明",
					model.ErrIsolationCross, e.FromNodeID, e.ToNodeID, zidA, zidB)
			}
		}
	}
	return nil
}

// declared 判断 zoneID 的声明交叉流路中是否包含 from|to 组合。
func declared(zones []*model.IsolationZone, zoneID, from, to int64) bool {
	for _, z := range zones {
		if z.ID != zoneID {
			continue
		}
		for _, pair := range strings.Split(z.DeclaredCrossings, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			parts := strings.Split(pair, "|")
			if len(parts) != 2 {
				continue
			}
			var a, b int64
			fmt.Sscanf(parts[0], "%d", &a)
			fmt.Sscanf(parts[1], "%d", &b)
			if (a == from && b == to) || (a == to && b == from) {
				return true
			}
		}
	}
	return false
}

// Graph 是版本拓扑的内存视图，供校验引擎使用。
type Graph struct {
	VersionID int64
	Nodes     []*model.Node
	Edges     []*model.Edge
	Zones     []*model.IsolationZone
	// ZoneMembers: zoneID -> nodeID 集合。
	ZoneMembers map[int64]map[int64]bool
	// ZoneOfNode: nodeID -> zoneID 集合。
	ZoneOfNode map[int64]map[int64]bool
}

// LoadGraph 从数据库加载版本的完整拓扑视图。
func (s *Service) LoadGraph(versionID int64) (*Graph, error) {
	nodes, err := s.topo.ListNodes(versionID)
	if err != nil {
		return nil, err
	}
	edges, err := s.topo.ListEdges(versionID)
	if err != nil {
		return nil, err
	}
	zones, err := s.topo.ListZones(versionID)
	if err != nil {
		return nil, err
	}
	g := &Graph{
		VersionID:   versionID,
		Nodes:       nodes,
		Edges:       edges,
		Zones:       zones,
		ZoneMembers: map[int64]map[int64]bool{},
		ZoneOfNode:  map[int64]map[int64]bool{},
	}
	for _, z := range zones {
		members, err := s.topo.ZoneMembers(z.ID)
		if err != nil {
			return nil, err
		}
		g.ZoneMembers[z.ID] = map[int64]bool{}
		for _, nid := range members {
			g.ZoneMembers[z.ID][nid] = true
			if g.ZoneOfNode[nid] == nil {
				g.ZoneOfNode[nid] = map[int64]bool{}
			}
			g.ZoneOfNode[nid][z.ID] = true
		}
	}
	return g, nil
}

// NodeByID 返回节点（不存在返回 nil）。
func (g *Graph) NodeByID(id int64) *model.Node {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// EdgeByID 返回边（不存在返回 nil）。
func (g *Graph) EdgeByID(id int64) *model.Edge {
	for _, e := range g.Edges {
		if e.ID == id {
			return e
		}
	}
	return nil
}
