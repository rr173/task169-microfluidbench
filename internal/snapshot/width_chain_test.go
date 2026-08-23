package snapshot_test

import (
	"testing"

	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/snapshot"
	"task169-microfluidbench/internal/store"
	"task169-microfluidbench/internal/topology"
)

// TestSnapshotHashEmbedsEdgeWidth 验证通道宽度贯穿快照哈希链路：
// 冻结时宽度必须进入拓扑哈希，且不同宽度产生不同哈希；冻结不改变内存图宽度。
func TestSnapshotHashEmbedsEdgeWidth(t *testing.T) {
	mk := func(width float64) (string, float64) {
		db, err := store.Open(":memory:")
		if err != nil {
			t.Fatalf("open database: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })
		chips := store.NewChipsStore(db)
		topo := topology.NewService(chips, store.NewTopologyStore(db))
		snaps := snapshot.NewService(store.NewSnapshotStore(db), chips, topo, store.NewFlowStore(db))

		chip, err := chips.CreateChip("hash-chip", "")
		if err != nil {
			t.Fatalf("create chip: %v", err)
		}
		ver, err := chips.CreateVersion(chip.ID, "")
		if err != nil {
			t.Fatalf("create version: %v", err)
		}
		in, err := topo.CreateNode(ver.ID, model.NodeInlet, "in", 0, 0)
		if err != nil {
			t.Fatalf("create inlet: %v", err)
		}
		out, err := topo.CreateNode(ver.ID, model.NodeOutlet, "out", 1, 0)
		if err != nil {
			t.Fatalf("create outlet: %v", err)
		}
		if _, err := topo.CreateEdge(ver.ID, &model.Edge{
			VersionID: ver.ID, FromNodeID: in.ID, FromPort: model.PortA,
			ToNodeID: out.ID, ToPort: model.PortB, Direction: model.DirectionOneWay,
			Width: width,
		}, 0); err != nil {
			t.Fatalf("create edge: %v", err)
		}
		if err := chips.UpdateVersionStatus(ver.ID, model.VersionApproved); err != nil {
			t.Fatalf("approve version: %v", err)
		}
		snap, err := snaps.BuildAndFreeze(ver.ID, "tester")
		if err != nil {
			t.Fatalf("build and freeze: %v", err)
		}
		// 冻结后重新加载图，确认宽度未被冻结过程改写。
		g, err := topo.LoadGraph(ver.ID)
		if err != nil {
			t.Fatalf("reload graph: %v", err)
		}
		if len(g.Edges) != 1 || g.Edges[0].Width != width {
			t.Fatalf("width lost after freeze: %+v", g.Edges)
		}
		return snap.TopologyHash, width
	}

	h1, _ := mk(10)
	h2, _ := mk(50)
	if h1 == "" {
		t.Fatal("topology hash should not be empty")
	}
	if h1 == h2 {
		t.Fatalf("不同宽度应产生不同快照哈希: h1=%s h2=%s", h1, h2)
	}
}
