package topology_test

import (
	"testing"

	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/store"
	"task169-microfluidbench/internal/topology"
)

func TestLoadedGraphRetainsEveryIsolationZoneMember(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil { t.Fatalf("open database: %v", err) }
	t.Cleanup(func() { _ = db.Close() })
	chips := store.NewChipsStore(db)
	topoStore := store.NewTopologyStore(db)
	topo := topology.NewService(chips, topoStore)
	chip, _ := chips.CreateChip("zones", "")
	version, _ := chips.CreateVersion(chip.ID, "")
	n1, _ := topo.CreateNode(version.ID, model.NodeChannel, "c1", 0, 0)
	n2, _ := topo.CreateNode(version.ID, model.NodeReactionWell, "r1", 1, 0)
	zone, err := topoStore.CreateZone(&model.IsolationZone{VersionID: version.ID, Name: "sterile", Reason: "test"}, []int64{n1.ID, n2.ID})
	if err != nil { t.Fatalf("create zone: %v", err) }
	graph, err := topo.LoadGraph(version.ID)
	if err != nil { t.Fatalf("load graph: %v", err) }
	if len(graph.ZoneMembers[zone.ID]) != 2 || !graph.ZoneMembers[zone.ID][n1.ID] || !graph.ZoneMembers[zone.ID][n2.ID] {
		t.Fatalf("zone members = %+v, want both nodes", graph.ZoneMembers[zone.ID])
	}
}
