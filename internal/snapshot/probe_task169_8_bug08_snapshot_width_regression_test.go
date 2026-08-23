package snapshot_test

import (
	"testing"
	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

func TestSnapshotHashAndGraphRetainChannelWidth(t *testing.T) {
	db, err := store.Open(":memory:"); if err != nil { t.Fatal(err) }
	app := service.New(db); t.Cleanup(func(){ _ = app.Close() })
	chip, _ := app.Chips.CreateChip("width", ""); version, _ := app.Chips.CreateVersion(chip.ID, "")
	a, _ := app.Topology.CreateNode(version.ID, model.NodeInlet, "a", 0, 0); b, _ := app.Topology.CreateNode(version.ID, model.NodeOutlet, "b", 1, 0)
	e, err := app.Topology.CreateEdge(version.ID, &model.Edge{VersionID:version.ID, FromNodeID:a.ID, FromPort:model.PortA, ToNodeID:b.ID, ToPort:model.PortA, Direction:model.DirectionOneWay, Width:37}, 0)
	if err != nil { t.Fatal(err) }
	g, err := app.Topology.LoadGraph(version.ID); if err != nil { t.Fatal(err) }
	if g.EdgeByID(e.ID).Width != 37 { t.Fatalf("width=%v", g.EdgeByID(e.ID).Width) }
	h := app.Snapshot.TopologyHash(g); if h == "" { t.Fatal("empty hash") }
}
