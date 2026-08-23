package flow_test

import (
	"errors"
	"testing"

	"task169-microfluidbench/internal/flow"
	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/store"
	"task169-microfluidbench/internal/topology"
)

func TestCreateStepRequiresEveryValveState(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	chips := store.NewChipsStore(db)
	topoStore := store.NewTopologyStore(db)
	topo := topology.NewService(chips, topoStore)
	steps := flow.NewService(store.NewFlowStore(db), topo)
	chip, err := chips.CreateChip("valve-coverage", "")
	if err != nil {
		t.Fatalf("create chip: %v", err)
	}
	version, err := chips.CreateVersion(chip.ID, "")
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	inlet, err := topo.CreateNode(version.ID, model.NodeInlet, "inlet", 0, 0)
	if err != nil {
		t.Fatalf("create inlet: %v", err)
	}
	valve, err := topo.CreateNode(version.ID, model.NodeValve, "v1", 1, 0)
	if err != nil {
		t.Fatalf("create valve: %v", err)
	}
	outlet, err := topo.CreateNode(version.ID, model.NodeOutlet, "outlet", 2, 0)
	if err != nil {
		t.Fatalf("create outlet: %v", err)
	}

	_, err = steps.CreateStep(&model.FlowStep{
		VersionID: version.ID, OrderNo: 1, FluidType: model.FluidSample,
		InletID: inlet.ID, OutletID: outlet.ID,
	})
	if !errors.Is(err, model.ErrValveMissing) {
		t.Fatalf("missing valve state error = %v, want ErrValveMissing", err)
	}

	step, err := steps.CreateStep(&model.FlowStep{
		VersionID: version.ID, OrderNo: 1, FluidType: model.FluidSample,
		InletID: inlet.ID, OutletID: outlet.ID,
		Valves: []model.ValveCommand{{NodeID: valve.ID, State: model.ValveOpen}},
	})
	if err != nil {
		t.Fatalf("create complete step: %v", err)
	}
	updated, err := steps.UpdateValves(step.ID, []model.ValveCommand{{NodeID: valve.ID, State: model.ValveClosed}})
	if err != nil {
		t.Fatalf("replace valve states: %v", err)
	}
	if len(updated.Valves) != 1 || updated.Valves[0].State != model.ValveClosed {
		t.Fatalf("updated valves = %+v, want one closed valve", updated.Valves)
	}
}
