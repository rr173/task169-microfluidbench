package topology_test

import (
	"testing"

	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/store"
	"task169-microfluidbench/internal/topology"
)

// TestEdgeWidthSurvivesLoad 验证通道宽度从数据库加载后贯穿画布与快照哈希链路：
// 创建时写入的宽度值不能在读取或冻结快照时丢失。
func TestEdgeWidthSurvivesLoad(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	chips := store.NewChipsStore(db)
	topoStore := store.NewTopologyStore(db)
	topo := topology.NewService(chips, topoStore)

	chip, err := chips.CreateChip("width-chip", "")
	if err != nil {
		t.Fatalf("create chip: %v", err)
	}
	version, err := chips.CreateVersion(chip.ID, "")
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	in, err := topo.CreateNode(version.ID, model.NodeInlet, "in", 0, 0)
	if err != nil {
		t.Fatalf("create inlet: %v", err)
	}
	out, err := topo.CreateNode(version.ID, model.NodeOutlet, "out", 1, 0)
	if err != nil {
		t.Fatalf("create outlet: %v", err)
	}

	const wantWidth = 42.5
	edge, err := topo.CreateEdge(version.ID, &model.Edge{
		VersionID: version.ID, FromNodeID: in.ID, FromPort: model.PortA,
		ToNodeID: out.ID, ToPort: model.PortB, Direction: model.DirectionOneWay,
		Width: wantWidth,
	}, 0)
	if err != nil {
		t.Fatalf("create edge: %v", err)
	}

	// 单边读取必须保留宽度。
	got, err := topoStore.GetEdge(edge.ID)
	if err != nil {
		t.Fatalf("get edge: %v", err)
	}
	if got.Width != wantWidth {
		t.Fatalf("GetEdge width lost: got %v want %v", got.Width, wantWidth)
	}

	// 列表读取必须保留宽度（画布与校验引擎均经此路径）。
	listed, err := topoStore.ListEdges(version.ID)
	if err != nil {
		t.Fatalf("list edges: %v", err)
	}
	if len(listed) != 1 || listed[0].Width != wantWidth {
		t.Fatalf("ListEdges width lost: %+v", listed)
	}

	// LoadGraph 是画布联动 (/graph) 与快照构建的统一入口，宽度必须贯穿。
	g, err := topo.LoadGraph(version.ID)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	if len(g.Edges) != 1 || g.Edges[0].Width != wantWidth {
		t.Fatalf("LoadGraph width lost: %+v", g.Edges)
	}
}
