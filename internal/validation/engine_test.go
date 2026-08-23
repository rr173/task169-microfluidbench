package validation

import (
	"testing"

	"task169-microfluidbench/internal/flow"
	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/topology"
)

// buildTestGraph 构造一个含入口/阀门/反应腔/出口的线性拓扑：
// inlet(1) --e1-- valve(2) --e2-- reaction(3) --e3-- outlet(4)
func buildTestGraph() *topology.Graph {
	return &topology.Graph{
		VersionID: 1,
		Nodes: []*model.Node{
			{ID: 1, Kind: model.NodeInlet, Name: "in"},
			{ID: 2, Kind: model.NodeValve, Name: "v1"},
			{ID: 3, Kind: model.NodeReactionWell, Name: "rxn"},
			{ID: 4, Kind: model.NodeOutlet, Name: "out"},
		},
		Edges: []*model.Edge{
			{ID: 1, VersionID: 1, FromNodeID: 1, ToNodeID: 2, Direction: model.DirectionTwoWay},
			{ID: 2, VersionID: 1, FromNodeID: 2, ToNodeID: 3, Direction: model.DirectionTwoWay},
			{ID: 3, VersionID: 1, FromNodeID: 3, ToNodeID: 4, Direction: model.DirectionOneWay},
		},
		ZoneMembers: map[int64]map[int64]bool{},
		ZoneOfNode:  map[int64]map[int64]bool{},
	}
}

func TestValidateStep_BlockedByClosedValve(t *testing.T) {
	g := buildTestGraph()
	st := &model.FlowStep{
		ID:        1,
		VersionID: 1,
		OrderNo:   1,
		FluidType: model.FluidSample,
		InletID:   1,
		OutletID:  4,
		Valves: []model.ValveCommand{
			{NodeID: 2, State: model.ValveClosed},
		},
	}
	res := NewEngine().ValidateStep(g, st)
	if res.Reachable {
		t.Fatal("关闭阀门后应不可达")
	}
	if !res.Passed {
		t.Logf("预期阻断: message=%s", res.Message)
	}
	if len(res.BlockedEdges) == 0 {
		t.Fatal("应存在阻断边")
	}
}

func TestValidateStep_ReachableWhenOpen(t *testing.T) {
	g := buildTestGraph()
	st := &model.FlowStep{
		ID:        1,
		VersionID: 1,
		OrderNo:   1,
		FluidType: model.FluidSample,
		InletID:   1,
		OutletID:  4,
		Valves: []model.ValveCommand{
			{NodeID: 2, State: model.ValveOpen},
		},
	}
	res := NewEngine().ValidateStep(g, st)
	if !res.Reachable {
		t.Fatalf("阀门打开后应可达: %s", res.Message)
	}
	if len(res.ReachablePath) != 4 {
		t.Fatalf("路径应为 4 节点, got %v", res.ReachablePath)
	}
	if !res.Passed {
		t.Fatalf("线性拓扑应通过: %s", res.Message)
	}
}

func TestRunnableEdges_TwoWayVsOneWay(t *testing.T) {
	g := buildTestGraph()
	st := &model.FlowStep{Valves: []model.ValveCommand{{NodeID: 2, State: model.ValveOpen}}}
	runnable := flow.RunnableEdges(g, st)
	adj := flow.Adjacency(g, runnable)
	// 单向边 e3 (3->4) 不允许 4->3 反向通行。
	if len(adj[4]) != 0 {
		t.Fatalf("单向边不应反向通行: %v", adj[4])
	}
	// 双向边 e1 (1<->2) 双向可通行。
	if len(adj[1]) != 1 || adj[1][0].Neighbor != 2 {
		t.Fatalf("双向边应正向通行: %v", adj[1])
	}
	if len(adj[2]) != 2 {
		t.Fatalf("v1 应连接 inlet 与 reaction: %v", adj[2])
	}
}

func TestDeadVolumeDetection(t *testing.T) {
	g := buildTestGraph()
	// 增加一条带阀门的废液支路：v1(2) --e4-- waste(5)。
	g.Nodes = append(g.Nodes, &model.Node{ID: 5, Kind: model.NodeWasteWell, Name: "waste"})
	g.Edges = append(g.Edges, &model.Edge{
		ID: 4, VersionID: 1, FromNodeID: 2, ToNodeID: 5, Direction: model.DirectionTwoWay,
	})
	st := &model.FlowStep{
		ID:        1,
		VersionID: 1,
		OrderNo:   1,
		FluidType: model.FluidSample,
		InletID:   1,
		OutletID:  4,
		Valves: []model.ValveCommand{
			{NodeID: 2, State: model.ValveOpen},
			{NodeID: 5, State: model.ValveOpen},
		},
	}
	res := NewEngine().ValidateStep(g, st)
	// waste(5) 是废液腔且阀门打开，可从入口覆盖，不应算死腔。
	if len(res.DeadVolumes) != 0 {
		t.Fatalf("可达废液腔不应算死腔: %v", res.DeadVolumes)
	}
	if !res.Passed {
		t.Fatalf("应通过: %s", res.Message)
	}
}

func TestIsolationBreakDetection(t *testing.T) {
	g := buildTestGraph()
	// 阀门 v1(2) 与反应腔(3) 分属两个隔离区，边 e2 (2->3) 跨区未声明。
	// 样本流路径 1->2->3->4 将经过该边，应被检出。
	g.Zones = []*model.IsolationZone{
		{ID: 10, VersionID: 1, Name: "valve-zone"},
		{ID: 20, VersionID: 1, Name: "rxn-zone"},
	}
	g.ZoneMembers[10] = map[int64]bool{2: true}
	g.ZoneMembers[20] = map[int64]bool{3: true}
	g.ZoneOfNode[2] = map[int64]bool{10: true}
	g.ZoneOfNode[3] = map[int64]bool{20: true}
	st := &model.FlowStep{
		ID:        1,
		VersionID: 1,
		OrderNo:   1,
		FluidType: model.FluidSample,
		InletID:   1,
		OutletID:  4,
		Valves:    []model.ValveCommand{{NodeID: 2, State: model.ValveOpen}},
	}
	res := NewEngine().ValidateStep(g, st)
	if len(res.IsolationBreaks) == 0 {
		t.Fatal("跨隔离区未声明应被检出")
	}
	if res.Passed {
		t.Fatal("存在隔离区穿越时不应通过")
	}
}
