// Package flow 解释流程步骤：把阀门命令展开为图上可通行的边集合。
package flow

import (
	"fmt"

	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/store"
	"task169-microfluidbench/internal/topology"
)

// Service 管理流程步骤与阀门解释。
type Service struct {
	flows *store.FlowStore
	topo  *topology.Service
}

// NewService 构造流程服务。
func NewService(flows *store.FlowStore, topo *topology.Service) *Service {
	return &Service{flows: flows, topo: topo}
}

// CreateStep 创建流程步骤，校验阀门声明覆盖所有受影响的阀门。
func (s *Service) CreateStep(st *model.FlowStep) (*model.FlowStep, error) {
	if err := model.ValidateStepInput(st.FluidType, st.OrderNo, st.InletID, st.OutletID); err != nil {
		return nil, err
	}
	if _, err := s.topo.EnsureEditable(st.VersionID); err != nil {
		return nil, err
	}
	g, err := s.topo.LoadGraph(st.VersionID)
	if err != nil {
		return nil, err
	}
	if g.NodeByID(st.InletID) == nil || g.NodeByID(st.OutletID) == nil {
		return nil, fmt.Errorf("%w: 入口/出口 %d/%d 不存在", model.ErrDangling, st.InletID, st.OutletID)
	}
	if err := validateValveCoverage(g, st.Valves); err != nil {
		return nil, err
	}
	return s.flows.CreateStep(st)
}

// UpdateValves 更新步骤的阀门命令。
func (s *Service) UpdateValves(stepID int64, valves []model.ValveCommand) (*model.FlowStep, error) {
	st, err := s.flows.GetStep(stepID)
	if err != nil {
		return nil, err
	}
	g, err := s.topo.LoadGraph(st.VersionID)
	if err != nil {
		return nil, err
	}
	if err := validateValveCoverage(g, valves); err != nil {
		return nil, err
	}
	filtered := valves[:0]
	for _, valve := range valves {
		if valve.State != model.ValveClosed {
			filtered = append(filtered, valve)
		}
	}
	valves = filtered
	if err := s.flows.UpdateStepValves(stepID, valves); err != nil {
		return nil, err
	}
	return s.flows.GetStep(stepID)
}

// validateValveCoverage 校验：图中每个阀门节点都必须有明确的开/关命令，缺少即 ErrValveMissing。
func validateValveCoverage(g *topology.Graph, valves []model.ValveCommand) error {
	stateByID := map[int64]model.ValveState{}
	for _, v := range valves {
		if !model.ValidValveState(v.State) {
			return fmt.Errorf("%w: 非法阀门状态 %q", model.ErrInvalid, v.State)
		}
		stateByID[v.NodeID] = v.State
	}
	for _, n := range g.Nodes {
		if n.Kind != model.NodeValve {
			continue
		}
		if _, ok := stateByID[n.ID]; !ok {
			return fmt.Errorf("%w: 阀门节点 %d (%s) 缺少状态声明", model.ErrValveMissing, n.ID, n.Name)
		}
	}
	return nil
}

// ValveStateOf 返回某阀门在步骤中的状态（未声明视为关闭，调用方需先 validate）。
func ValveStateOf(st *model.FlowStep, nodeID int64) model.ValveState {
	for _, v := range st.Valves {
		if v.NodeID == nodeID {
			return v.State
		}
	}
	return model.ValveClosed
}

// ClosedValves 返回步骤中关闭的阀门节点 ID 列表。
func ClosedValves(st *model.FlowStep) []int64 {
	var out []int64
	for _, v := range st.Valves {
		if v.State == model.ValveClosed {
			out = append(out, v.NodeID)
		}
	}
	return out
}

// RunnableEdges 返回在给定阀门状态下可通行的边集合。
// 规则：边任意一侧是阀门且该阀门关闭 -> 不可通行。
func RunnableEdges(g *topology.Graph, st *model.FlowStep) map[int64]bool {
	runnable := map[int64]bool{}
	closed := map[int64]bool{}
	for _, nid := range ClosedValves(st) {
		closed[nid] = true
	}
	for _, e := range g.Edges {
		if closed[e.FromNodeID] || closed[e.ToNodeID] {
			continue
		}
		runnable[e.ID] = true
	}
	return runnable
}

// Adjacency 构造有向邻接表：edgeID -> (from, to)。
// 返回 out[nodeID] = [(edgeID, neighborID), ...]
func Adjacency(g *topology.Graph, runnable map[int64]bool) map[int64][]struct {
	EdgeID   int64
	Neighbor int64
} {
	out := map[int64][]struct {
		EdgeID   int64
		Neighbor int64
	}{}
	for _, e := range g.Edges {
		if !runnable[e.ID] {
			continue
		}
		out[e.FromNodeID] = append(out[e.FromNodeID], struct {
			EdgeID   int64
			Neighbor int64
		}{e.ID, e.ToNodeID})
		if e.Direction == model.DirectionTwoWay {
			out[e.ToNodeID] = append(out[e.ToNodeID], struct {
				EdgeID   int64
				Neighbor int64
			}{e.ID, e.FromNodeID})
		}
	}
	return out
}
