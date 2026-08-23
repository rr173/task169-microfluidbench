package validation

import (
	"testing"

	"task169-microfluidbench/internal/model"
)

// 关闭阀门时，所有连接该阀门的通道（含入向边）都应被标记为阻断，
// 且为不可达流路生成 unreachable 风险。
func TestValidateStep_ClosedValveBlocksInboundAndOutbound(t *testing.T) {
	g := buildTestGraph()
	// 增加一条以阀门为 ToNodeID 的入向边：inlet2(6) --e5--> valve(2)。
	g.Nodes = append(g.Nodes, &model.Node{ID: 6, Kind: model.NodeInlet, Name: "in2"})
	g.Edges = append(g.Edges, &model.Edge{
		ID: 5, VersionID: 1, FromNodeID: 6, ToNodeID: 2, Direction: model.DirectionTwoWay,
	})
	st := &model.FlowStep{
		ID:        1,
		VersionID: 1,
		OrderNo:   1,
		FluidType: model.FluidSample,
		InletID:   1,
		OutletID:  4,
		Valves:    []model.ValveCommand{{NodeID: 2, State: model.ValveClosed}},
	}
	res := NewEngine().ValidateStep(g, st)
	if res.Reachable {
		t.Fatal("关闭阀门后应不可达")
	}
	want := map[int64]bool{1: true, 2: true, 5: true} // e1(入向)、e2(出向)、e5(入向) 均应阻断
	got := map[int64]bool{}
	for _, id := range res.BlockedEdges {
		got[id] = true
	}
	for w := range want {
		if !got[w] {
			t.Errorf("入向/出向通道 %d 应被阻断, 实际 BlockedEdges=%v", w, res.BlockedEdges)
		}
	}
	// 风险建议：不可达时必须生成 unreachable 风险。
	vr := &model.ValidationResult{ID: 1, VersionID: 1, Reachable: res.Reachable, BlockedEdges: res.BlockedEdges}
	risks := (&Service{}).SuggestRisk(vr)
	if len(risks) == 0 {
		t.Fatal("不可达流路应生成 unreachable 风险")
	}
	var gotUnreachable bool
	for _, r := range risks {
		if r.Kind == "unreachable" {
			gotUnreachable = true
		}
	}
	if !gotUnreachable {
		t.Errorf("应存在 unreachable 风险, 实际=%+v", risks)
	}
}

// 单条阻断边（仅一个阀门、两端口）仍应生成 unreachable 风险：阈值不应为 >1。
func TestSuggestRisk_UnreachableOnSingleBlockedEdge(t *testing.T) {
	vr := &model.ValidationResult{ID: 1, VersionID: 1, Reachable: false, BlockedEdges: []int64{2}}
	risks := (&Service{}).SuggestRisk(vr)
	found := false
	for _, r := range risks {
		if r.Kind == "unreachable" {
			found = true
		}
	}
	if !found {
		t.Fatalf("单条阻断边且不可达时应生成 unreachable 风险, 实际=%+v", risks)
	}
}

// 可达时即使存在阻断边（侧支阀门关闭）也不应误报 unreachable 风险。
func TestSuggestRisk_NoUnreachableWhenReachable(t *testing.T) {
	vr := &model.ValidationResult{ID: 1, VersionID: 1, Reachable: true, BlockedEdges: []int64{7}}
	risks := (&Service{}).SuggestRisk(vr)
	for _, r := range risks {
		if r.Kind == "unreachable" {
			t.Fatalf("可达时不应生成 unreachable 风险, 实际=%+v", risks)
		}
	}
}
