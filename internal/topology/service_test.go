package topology_test

import (
	"errors"
	"testing"

	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/store"
	"task169-microfluidbench/internal/topology"
)

func TestCreateEdgeHonorsRevisionAndFrozenVersion(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	chips := store.NewChipsStore(db)
	topo := topology.NewService(chips, store.NewTopologyStore(db))
	chip, err := chips.CreateChip("revision-chip", "")
	if err != nil {
		t.Fatalf("create chip: %v", err)
	}
	version, err := chips.CreateVersion(chip.ID, "")
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	from, err := topo.CreateNode(version.ID, model.NodeInlet, "inlet", 0, 0)
	if err != nil {
		t.Fatalf("create inlet: %v", err)
	}
	to, err := topo.CreateNode(version.ID, model.NodeOutlet, "outlet", 1, 0)
	if err != nil {
		t.Fatalf("create outlet: %v", err)
	}
	edge := &model.Edge{VersionID: version.ID, FromNodeID: from.ID, FromPort: model.PortA, ToNodeID: to.ID, ToPort: model.PortA, Direction: model.DirectionOneWay, Width: 10}
	if _, err := topo.CreateEdge(version.ID, edge, 0); err != nil {
		t.Fatalf("create edge: %v", err)
	}
	if _, err := topo.CreateEdge(version.ID, edge, 0); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("stale revision error = %v, want ErrConflict", err)
	}
	if err := chips.UpdateVersionStatus(version.ID, model.VersionApproved); err != nil {
		t.Fatalf("approve version: %v", err)
	}
	if _, err := topo.CreateNode(version.ID, model.NodeChannel, "blocked", 2, 0); !errors.Is(err, model.ErrFrozen) {
		t.Fatalf("frozen version error = %v, want ErrFrozen", err)
	}
}
