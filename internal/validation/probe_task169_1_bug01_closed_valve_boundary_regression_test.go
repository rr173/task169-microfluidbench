package validation_test

import (
	"testing"

	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

func TestClosedValveBlocksEveryAttachedChannelAndCreatesRisk(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := service.New(db)
	chip, err := app.Chips.CreateChip("closed-valve-boundary", "")
	if err != nil {
		t.Fatalf("create chip: %v", err)
	}
	version, err := app.Chips.CreateVersion(chip.ID, "")
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	inlet, err := app.Topology.CreateNode(version.ID, model.NodeInlet, "inlet", 0, 0)
	if err != nil {
		t.Fatalf("create inlet: %v", err)
	}
	valve, err := app.Topology.CreateNode(version.ID, model.NodeValve, "v1", 1, 0)
	if err != nil {
		t.Fatalf("create valve: %v", err)
	}
	outlet, err := app.Topology.CreateNode(version.ID, model.NodeOutlet, "outlet", 2, 0)
	if err != nil {
		t.Fatalf("create outlet: %v", err)
	}
	if _, err := app.Topology.CreateEdge(version.ID, &model.Edge{VersionID: version.ID, FromNodeID: inlet.ID, FromPort: model.PortA, ToNodeID: valve.ID, ToPort: model.PortA, Direction: model.DirectionOneWay, Width: 10}, 0); err != nil {
		t.Fatalf("create inlet edge: %v", err)
	}
	if _, err := app.Topology.CreateEdge(version.ID, &model.Edge{VersionID: version.ID, FromNodeID: valve.ID, FromPort: model.PortB, ToNodeID: outlet.ID, ToPort: model.PortA, Direction: model.DirectionOneWay, Width: 10}, 1); err != nil {
		t.Fatalf("create outlet edge: %v", err)
	}
	step, err := app.Flow.CreateStep(&model.FlowStep{VersionID: version.ID, OrderNo: 1, FluidType: model.FluidSample, InletID: inlet.ID, OutletID: outlet.ID, Valves: []model.ValveCommand{{NodeID: valve.ID, State: model.ValveClosed}}})
	if err != nil {
		t.Fatalf("create step: %v", err)
	}
	result, err := app.Validation.ValidateStep(step.ID)
	if err != nil {
		t.Fatalf("validate closed valve: %v", err)
	}
	if result.Reachable {
		t.Fatal("closed valve must make the outlet unreachable")
	}
	if len(result.BlockedEdges) != 2 {
		t.Fatalf("blocked edges = %v, want both channels attached to the closed valve", result.BlockedEdges)
	}
	risks, err := app.Risk.CreateFromValidation(result.ID)
	if err != nil {
		t.Fatalf("create risks: %v", err)
	}
	if len(risks) != 1 || risks[0].Kind != "unreachable" {
		t.Fatalf("risks = %+v, want one unreachable-flow risk", risks)
	}
}
