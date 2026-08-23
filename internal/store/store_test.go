package store

import (
	"path/filepath"
	"testing"

	"task169-microfluidbench/internal/model"
)

// newTestDB 创建内存数据库。
func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestChipVersionPersistence(t *testing.T) {
	db := newTestDB(t)
	chips := NewChipsStore(db)

	c, err := chips.CreateChip("test-chip", "持久化测试")
	if err != nil {
		t.Fatalf("创建芯片: %v", err)
	}
	v1, err := chips.CreateVersion(c.ID, "v1")
	if err != nil {
		t.Fatalf("创建版本: %v", err)
	}
	v2, err := chips.CreateVersion(c.ID, "v2")
	if err != nil {
		t.Fatalf("创建版本2: %v", err)
	}
	if v1.VersionNo != 1 || v2.VersionNo != 2 {
		t.Fatalf("版本号应递增: %d, %d", v1.VersionNo, v2.VersionNo)
	}
	if v1.Status != model.VersionEditing {
		t.Fatalf("新版本应为 editing, got %s", v1.Status)
	}
	// 乐观版本号递增。
	if err := chips.BumpVersionRev(v1.ID); err != nil {
		t.Fatalf("递增 rev: %v", err)
	}
	ok, err := chips.RevOK(v1.ID, 1)
	if err != nil || !ok {
		t.Fatalf("rev 应为 1: ok=%v err=%v", ok, err)
	}
	ok, _ = chips.RevOK(v1.ID, 0)
	if ok {
		t.Fatal("旧 rev 不应匹配")
	}
	// 状态迁移持久化。
	if err := chips.UpdateVersionStatus(v1.ID, model.VersionPending); err != nil {
		t.Fatalf("更新状态: %v", err)
	}
	got, err := chips.GetVersion(v1.ID)
	if err != nil {
		t.Fatalf("读取版本: %v", err)
	}
	if got.Status != model.VersionPending {
		t.Fatalf("状态未持久化: %s", got.Status)
	}
}

func TestTopologyAndZones(t *testing.T) {
	db := newTestDB(t)
	chips := NewChipsStore(db)
	topo := NewTopologyStore(db)

	c, _ := chips.CreateChip("chip", "")
	v, _ := chips.CreateVersion(c.ID, "")

	n1, err := topo.CreateNode(&model.Node{VersionID: v.ID, Kind: model.NodeInlet, Name: "in"})
	if err != nil {
		t.Fatalf("创建节点: %v", err)
	}
	n2, err := topo.CreateNode(&model.Node{VersionID: v.ID, Kind: model.NodeOutlet, Name: "out"})
	if err != nil {
		t.Fatalf("创建节点2: %v", err)
	}
	e, err := topo.CreateEdge(&model.Edge{
		VersionID: v.ID, FromNodeID: n1.ID, FromPort: "A",
		ToNodeID: n2.ID, ToPort: "B", Direction: model.DirectionOneWay, Width: 10,
	})
	if err != nil {
		t.Fatalf("创建边: %v", err)
	}
	if e.ID == 0 {
		t.Fatal("边 ID 不应为 0")
	}
	// 隔离区与成员。
	z, err := topo.CreateZone(&model.IsolationZone{
		VersionID: v.ID, Name: "zone", Reason: "test",
		DeclaredCrossings: "1|2",
	}, []int64{n1.ID, n2.ID})
	if err != nil {
		t.Fatalf("创建隔离区: %v", err)
	}
	members, err := topo.ZoneMembers(z.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("隔离区成员应为 2: %v err=%v", members, err)
	}
	// 删除节点应级联清理边。
	if err := topo.DeleteNode(n1.ID); err != nil {
		t.Fatalf("删除节点: %v", err)
	}
	edges, err := topo.ListEdges(v.ID)
	if err != nil || len(edges) != 0 {
		t.Fatalf("删除节点后边应清空: %v err=%v", edges, err)
	}
}

func TestValidationAndRiskFlow(t *testing.T) {
	db := newTestDB(t)
	chips := NewChipsStore(db)
	flows := NewFlowStore(db)
	valid := NewValidationStore(db)
	risks := NewRiskStore(db)

	c, _ := chips.CreateChip("chip", "")
	v, _ := chips.CreateVersion(c.ID, "")

	st, err := flows.CreateStep(&model.FlowStep{
		VersionID: v.ID, OrderNo: 1, FluidType: model.FluidSample,
		InletID: 1, OutletID: 2,
		Valves: []model.ValveCommand{{NodeID: 3, State: model.ValveOpen}},
	})
	if err != nil {
		t.Fatalf("创建步骤: %v", err)
	}
	if st.Status != model.StepDraft {
		t.Fatalf("默认状态应为 draft: %s", st.Status)
	}
	vr, err := valid.SaveValidation(&model.ValidationResult{
		StepID: st.ID, VersionID: v.ID, Passed: false, Reachable: false,
		BlockedEdges: []int64{1}, Message: "blocked",
	})
	if err != nil {
		t.Fatalf("保存校验: %v", err)
	}
	got, err := valid.GetValidation(vr.ID)
	if err != nil {
		t.Fatalf("读取校验: %v", err)
	}
	if got.Passed || len(got.BlockedEdges) != 1 {
		t.Fatalf("校验结果未正确往返: %+v", got)
	}
	r, err := risks.CreateRisk(&model.Risk{
		VersionID: v.ID, ValidationID: vr.ID, Kind: "unreachable",
		Severity: "high", Title: "阻断",
	})
	if err != nil {
		t.Fatalf("创建风险: %v", err)
	}
	if r.Status != model.RiskNew {
		t.Fatalf("默认风险状态应为 new: %s", r.Status)
	}
	if err := risks.UpdateRiskStatus(r.ID, model.RiskConfirmed, "alice", "确认"); err != nil {
		t.Fatalf("更新风险状态: %v", err)
	}
	gotR, err := risks.GetRisk(r.ID)
	if err != nil || gotR.Status != model.RiskConfirmed || gotR.Owner != "alice" {
		t.Fatalf("风险状态未持久化: %+v err=%v", gotR, err)
	}
}

// TestValidationRoundTripAllIDs 校验校验结果落库/回读完整保留全部边/节点 ID（含奇数 ID），
// 使审查者能完整追溯本次流路校验为何失败。
func TestValidationRoundTripAllIDs(t *testing.T) {
	db := newTestDB(t)
	chips := NewChipsStore(db)
	flows := NewFlowStore(db)
	valid := NewValidationStore(db)

	c, _ := chips.CreateChip("chip", "")
	v, _ := chips.CreateVersion(c.ID, "")

	st, err := flows.CreateStep(&model.FlowStep{
		VersionID: v.ID, OrderNo: 1, FluidType: model.FluidSample,
		InletID: 1, OutletID: 2,
		Valves: []model.ValveCommand{{NodeID: 3, State: model.ValveOpen}},
	})
	if err != nil {
		t.Fatalf("创建步骤: %v", err)
	}
	// 混合奇偶 ID，覆盖"奇数 ID 不被丢弃"的回归点。
	want := &model.ValidationResult{
		StepID: st.ID, VersionID: v.ID, Passed: false, Reachable: false,
		ReachablePath:    []int64{1, 2, 3},
		BlockedEdges:     []int64{1, 2, 3, 4, 5},
		DeadVolumes:      []int64{3, 5, 7},
		CrossContamEdges: []int64{2, 4, 6},
		ResidualWells:    []int64{5, 9},
		IsolationBreaks:  []int64{1, 3, 7},
		Message:          "阻断证据完整",
	}
	vr, err := valid.SaveValidation(want)
	if err != nil {
		t.Fatalf("保存校验: %v", err)
	}
	got, err := valid.GetValidation(vr.ID)
	if err != nil {
		t.Fatalf("读取校验: %v", err)
	}
	if !int64SliceEqual(got.BlockedEdges, want.BlockedEdges) {
		t.Fatalf("BlockedEdges 未完整往返: got %v want %v", got.BlockedEdges, want.BlockedEdges)
	}
	if !int64SliceEqual(got.ReachablePath, want.ReachablePath) {
		t.Fatalf("ReachablePath 未完整往返: got %v want %v", got.ReachablePath, want.ReachablePath)
	}
	if !int64SliceEqual(got.DeadVolumes, want.DeadVolumes) {
		t.Fatalf("DeadVolumes 未完整往返: got %v want %v", got.DeadVolumes, want.DeadVolumes)
	}
	if !int64SliceEqual(got.CrossContamEdges, want.CrossContamEdges) {
		t.Fatalf("CrossContamEdges 未完整往返: got %v want %v", got.CrossContamEdges, want.CrossContamEdges)
	}
	if !int64SliceEqual(got.ResidualWells, want.ResidualWells) {
		t.Fatalf("ResidualWells 未完整往返: got %v want %v", got.ResidualWells, want.ResidualWells)
	}
	if !int64SliceEqual(got.IsolationBreaks, want.IsolationBreaks) {
		t.Fatalf("IsolationBreaks 未完整往返: got %v want %v", got.IsolationBreaks, want.IsolationBreaks)
	}
	// ListByVersion 走同样的编/解码路径，亦须完整。
	list, err := valid.ListByVersion(v.ID)
	if err != nil {
		t.Fatalf("列校验: %v", err)
	}
	if len(list) != 1 || !int64SliceEqual(list[0].BlockedEdges, want.BlockedEdges) {
		t.Fatalf("ListByVersion BlockedEdges 未完整往返: %+v", list)
	}
}

func int64SliceEqual(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFileDBReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("打开文件库: %v", err)
	}
	chips := NewChipsStore(db)
	c, _ := chips.CreateChip("persist", "")
	_, _ = chips.CreateVersion(c.ID, "")
	db.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("重开文件库: %v", err)
	}
	defer db2.Close()
	chips2 := NewChipsStore(db2)
	got, err := chips2.GetChip(c.ID)
	if err != nil {
		t.Fatalf("重开后读取: %v", err)
	}
	if got.Name != "persist" {
		t.Fatalf("重启恢复数据不一致: %s", got.Name)
	}
	vs, err := chips2.ListVersions(c.ID)
	if err != nil || len(vs) != 1 {
		t.Fatalf("重开后版本数异常: %v err=%v", vs, err)
	}
}
