package snapshot_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

// TestSnapshotPreservesFinalClosedValveState 冻结快照后，阀门状态表必须保留流程
// 最后声明的 closed 状态，且关闭并重开数据库后仍可读取该状态。
func TestSnapshotPreservesFinalClosedValveState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("打开数据库: %v", err)
	}
	app := service.New(db)

	chip, err := app.Chips.CreateChip("closed-valve", "")
	if err != nil {
		t.Fatalf("创建芯片: %v", err)
	}
	ver, err := app.Chips.CreateVersion(chip.ID, "")
	if err != nil {
		t.Fatalf("创建版本: %v", err)
	}
	inlet, _ := app.Topology.CreateNode(ver.ID, model.NodeInlet, "in", 0, 0)
	v1, _ := app.Topology.CreateNode(ver.ID, model.NodeValve, "v1", 1, 0)
	outlet, _ := app.Topology.CreateNode(ver.ID, model.NodeOutlet, "out", 2, 0)
	_, _ = app.Topology.CreateEdge(ver.ID, &model.Edge{
		VersionID: ver.ID, FromNodeID: inlet.ID, FromPort: "A",
		ToNodeID: v1.ID, ToPort: "A", Direction: model.DirectionTwoWay, Width: 10,
	}, ver.Rev)
	_, _ = app.Topology.CreateEdge(ver.ID, &model.Edge{
		VersionID: ver.ID, FromNodeID: v1.ID, FromPort: "B",
		ToNodeID: outlet.ID, ToPort: "A", Direction: model.DirectionOneWay, Width: 10,
	}, ver.Rev)

	// 步骤声明 V1 关闭，并经阀门更新（UpdateValves）后仍为 closed。
	step, err := app.Flow.CreateStep(&model.FlowStep{
		VersionID: ver.ID, OrderNo: 1, FluidType: model.FluidSample,
		InletID: inlet.ID, OutletID: outlet.ID,
		Valves: []model.ValveCommand{{NodeID: v1.ID, State: model.ValveClosed}},
	})
	if err != nil {
		t.Fatalf("创建步骤: %v", err)
	}
	updated, err := app.Flow.UpdateValves(step.ID, []model.ValveCommand{
		{NodeID: v1.ID, State: model.ValveClosed},
	})
	if err != nil {
		t.Fatalf("更新阀门: %v", err)
	}
	if got := valveState(updated.Valves, v1.ID); got != model.ValveClosed {
		t.Fatalf("更新后阀门状态 = %s, want closed", got)
	}

	// 批准并冻结快照。
	if err := app.Chips.UpdateVersionStatus(ver.ID, model.VersionApproved); err != nil {
		t.Fatalf("批准版本: %v", err)
	}
	snap, err := app.Snapshot.BuildAndFreeze(ver.ID, "tester")
	if err != nil {
		t.Fatalf("冻结快照: %v", err)
	}
	if snap.Status != model.SnapshotFrozen {
		t.Fatalf("快照状态 = %s, want frozen", snap.Status)
	}
	if !stateIsClosed(t, snap.ValveStateTable, v1.ID) {
		t.Fatalf("冻结快照阀门表未保留 closed: %s", snap.ValveStateTable)
	}

	// 关闭并重开数据库，验证关闭状态在重启后仍可读取。
	if err := app.Close(); err != nil {
		t.Fatalf("关闭数据库: %v", err)
	}
	db2, err := store.Open(path)
	if err != nil {
		t.Fatalf("重开数据库: %v", err)
	}
	defer db2.Close()
	app2 := service.New(db2)
	snaps, err := app2.Snaps.ListByVersion(ver.ID)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("重启后快照数异常: %v err=%v", snaps, err)
	}
	if !stateIsClosed(t, snaps[0].ValveStateTable, v1.ID) {
		t.Fatalf("重启后快照阀门表未保留 closed: %s", snaps[0].ValveStateTable)
	}
}

// valveState 返回某阀门在命令列表中的状态。
func valveState(valves []model.ValveCommand, nodeID int64) model.ValveState {
	for _, v := range valves {
		if v.NodeID == nodeID {
			return v.State
		}
	}
	return model.ValveClosed
}

// stateIsClosed 解析阀门状态表，确认指定阀门在某步骤中为 closed。
func stateIsClosed(t *testing.T, table string, valveID int64) bool {
	t.Helper()
	if table == "" {
		return false
	}
	var rows []struct {
		ValveID int64             `json:"valve_id"`
		States  map[string]string `json:"states"`
	}
	if err := json.Unmarshal([]byte(table), &rows); err != nil {
		t.Fatalf("解析阀门表失败: %v", err)
	}
	for _, r := range rows {
		if r.ValveID != valveID {
			continue
		}
		for _, s := range r.States {
			if s == string(model.ValveClosed) {
				return true
			}
		}
	}
	return false
}
