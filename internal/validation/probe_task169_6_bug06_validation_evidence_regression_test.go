package validation_test

import (
	"testing"

	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

func TestValidationEvidenceRetainsAllBlockedEdges(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil { t.Fatalf("open database: %v", err) }
	app := service.New(db)
	t.Cleanup(func() { _ = app.Close() })
	chip, _ := app.Chips.CreateChip("evidence", "")
	version, _ := app.Chips.CreateVersion(chip.ID, "")
	inlet, _ := app.Topology.CreateNode(version.ID, model.NodeInlet, "in", 0, 0)
	valve, _ := app.Topology.CreateNode(version.ID, model.NodeValve, "v", 1, 0)
	outlet, _ := app.Topology.CreateNode(version.ID, model.NodeOutlet, "out", 2, 0)
	app.Topology.CreateEdge(version.ID, &model.Edge{VersionID: version.ID, FromNodeID: inlet.ID, FromPort: model.PortA, ToNodeID: valve.ID, ToPort: model.PortA, Direction: model.DirectionOneWay, Width: 10}, 0)
	app.Topology.CreateEdge(version.ID, &model.Edge{VersionID: version.ID, FromNodeID: valve.ID, FromPort: model.PortB, ToNodeID: outlet.ID, ToPort: model.PortA, Direction: model.DirectionOneWay, Width: 10}, 1)
	step, err := app.Flow.CreateStep(&model.FlowStep{VersionID: version.ID, OrderNo: 1, FluidType: model.FluidSample, InletID: inlet.ID, OutletID: outlet.ID, Valves: []model.ValveCommand{{NodeID: valve.ID, State: model.ValveClosed}}})
	if err != nil { t.Fatalf("create step: %v", err) }
	vr, err := app.Validation.ValidateStep(step.ID)
	if err != nil { t.Fatalf("validate: %v", err) }
	if len(vr.BlockedEdges) != 2 { t.Fatalf("blocked evidence = %v", vr.BlockedEdges) }
	got, err := app.Validation.GetValidation(vr.ID)
	if err != nil { t.Fatalf("reload validation: %v", err) }
	if len(got.BlockedEdges) != 2 { t.Fatalf("reloaded evidence = %v", got.BlockedEdges) }
}
