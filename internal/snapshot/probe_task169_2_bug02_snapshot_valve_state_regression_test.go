package snapshot_test

import (
	"strings"
	"testing"

	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

func TestFrozenSnapshotPreservesClosedValveState(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	app := service.New(db)
	t.Cleanup(func() { _ = app.Close() })
	chip, err := app.Chips.CreateChip("snapshot-valves", "")
	if err != nil {
		t.Fatalf("create chip: %v", err)
	}
	version, err := app.Chips.CreateVersion(chip.ID, "")
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	inlet, _ := app.Topology.CreateNode(version.ID, model.NodeInlet, "in", 0, 0)
	valve, _ := app.Topology.CreateNode(version.ID, model.NodeValve, "v1", 1, 0)
	outlet, _ := app.Topology.CreateNode(version.ID, model.NodeOutlet, "out", 2, 0)
	if _, err := app.Topology.CreateEdge(version.ID, &model.Edge{VersionID: version.ID, FromNodeID: inlet.ID, FromPort: model.PortA, ToNodeID: valve.ID, ToPort: model.PortA, Direction: model.DirectionTwoWay, Width: 10}, 0); err != nil {
		t.Fatalf("create first edge: %v", err)
	}
	if _, err := app.Topology.CreateEdge(version.ID, &model.Edge{VersionID: version.ID, FromNodeID: valve.ID, FromPort: model.PortB, ToNodeID: outlet.ID, ToPort: model.PortA, Direction: model.DirectionTwoWay, Width: 10}, 1); err != nil {
		t.Fatalf("create second edge: %v", err)
	}
	step, err := app.Flow.CreateStep(&model.FlowStep{VersionID: version.ID, OrderNo: 1, FluidType: model.FluidSample, InletID: inlet.ID, OutletID: outlet.ID, Valves: []model.ValveCommand{{NodeID: valve.ID, State: model.ValveOpen}}})
	if err != nil {
		t.Fatalf("create step: %v", err)
	}
	step, err = app.Flow.UpdateValves(step.ID, []model.ValveCommand{{NodeID: valve.ID, State: model.ValveClosed}})
	if err != nil {
		t.Fatalf("close valve: %v", err)
	}
	if len(step.Valves) != 1 || step.Valves[0].State != model.ValveClosed {
		t.Fatalf("updated step valves = %+v", step.Valves)
	}
	if err := app.Chips.UpdateVersionStatus(version.ID, model.VersionApproved); err != nil {
		t.Fatalf("approve version: %v", err)
	}
	snap, err := app.Snapshot.BuildAndFreeze(version.ID, "test")
	if err != nil {
		t.Fatalf("freeze snapshot: %v", err)
	}
	if !strings.Contains(snap.ValveStateTable, "closed") {
		t.Fatalf("snapshot valve table = %q, want closed state", snap.ValveStateTable)
	}
}
