// Package validation 执行流体可达性、死腔、交叉污染、清洗残留与隔离区校验。
package validation

import (
	"fmt"
	"sort"

	"task169-microfluidbench/internal/flow"
	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/topology"
)

// Engine 是校验引擎：对单个流程步骤执行完整分析。
type Engine struct{}

// NewEngine 构造校验引擎。
func NewEngine() *Engine { return &Engine{} }

// Result 是引擎分析产物（与持久化模型对应）。
type Result struct {
	Passed           bool
	Reachable        bool
	ReachablePath    []int64
	BlockedEdges     []int64
	DeadVolumes      []int64
	CrossContamEdges []int64
	ResidualWells    []int64
	IsolationBreaks  []int64
	Message          string
}

// ValidateStep 对流程步骤执行全量校验。
func (e *Engine) ValidateStep(g *topology.Graph, st *model.FlowStep) *Result {
	res := &Result{Passed: true}
	runnable := flow.RunnableEdges(g, st)
	adj := flow.Adjacency(g, runnable)

	// 1. 可达性：从入口沿可通行边 BFS 到出口。
	reachable, path := bfs(adj, st.InletID, st.OutletID)
	res.Reachable = reachable
	res.ReachablePath = path
	if !reachable {
		res.Passed = false
		res.Message = "样本流无法到达目标出口：存在阻断边或孤立路径"
		res.BlockedEdges = collectBlocked(g, st)
		return res
	}

	// 2. 死腔分析：清洗液从清洗入口出发，不能到达的“活”节点即为死腔。
	//    仅对非清洗步骤做死腔检查（清洗步骤本身覆盖全图）。
	if !model.IsFluidRinse(st.FluidType) {
		res.DeadVolumes = e.findDeadVolumes(g, adj, path)
		if len(res.DeadVolumes) > 0 {
			res.Passed = false
			res.Message = fmt.Sprintf("发现 %d 个死腔节点，清洗液无法覆盖", len(res.DeadVolumes))
		}
	}

	// 3. 交叉污染：样本/试剂路径与另一条入口路径共享非反应腔节点。
	if model.IsFluidContaminant(st.FluidType) {
		res.CrossContamEdges = e.findCrossContamination(g, adj, path, st)
		if len(res.CrossContamEdges) > 0 {
			res.Passed = false
			res.Message = fmt.Sprintf("发现 %d 条交叉污染路径", len(res.CrossContamEdges))
		}
	}

	// 4. 隔离区检查：路径必须被声明交叉流路覆盖，否则视为穿越隔离区。
	res.IsolationBreaks = e.findIsolationBreaks(g, path)
	if len(res.IsolationBreaks) > 0 {
		res.Passed = false
		res.Message = fmt.Sprintf("发现 %d 处未声明的隔离区穿越", len(res.IsolationBreaks))
	}

	// 5. 残留腔：反应腔与废液腔若不在清洗入口可达范围内则残留。
	res.ResidualWells = e.findResidualWells(g, adj, path)
	if len(res.ResidualWells) > 0 {
		res.Passed = false
		res.Message = fmt.Sprintf("发现 %d 个无法清洗的残留腔", len(res.ResidualWells))
	}

	if res.Passed {
		res.Message = "校验通过：路径可达、无死腔、无污染、无隔离穿越、无残留"
	}
	return res
}

// bfs 从 start 到 target 的广度优先搜索，返回是否可达与路径（节点 ID 序列）。
func bfs(adj map[int64][]struct {
	EdgeID   int64
	Neighbor int64
}, start, target int64) (bool, []int64) {
	type node struct {
		prev int64
	}
	visited := map[int64]bool{start: true}
	queue := []int64{start}
	prev := map[int64]int64{}
	found := false
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == target {
			found = true
			break
		}
		for _, hop := range adj[cur] {
			if visited[hop.Neighbor] {
				continue
			}
			visited[hop.Neighbor] = true
			prev[hop.Neighbor] = cur
			queue = append(queue, hop.Neighbor)
		}
	}
	if !found {
		return false, nil
	}
	// 回溯路径。
	var path []int64
	for cur := target; ; {
		path = append([]int64{cur}, path...)
		if cur == start {
			break
		}
		cur = prev[cur]
	}
	return true, path
}

// collectBlocked 返回导致路径阻断的边（关闭阀门所在边）。
func collectBlocked(g *topology.Graph, st *model.FlowStep) []int64 {
	closed := map[int64]bool{}
	for _, nid := range flow.ClosedValves(st) {
		closed[nid] = true
	}
	var out []int64
	for _, e := range g.Edges {
		if closed[e.FromNodeID] || closed[e.ToNodeID] {
			out = append(out, e.ID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// findDeadVolumes 寻找无法被清洗液覆盖的节点（非入口出口、且不在路径上）。
func (e *Engine) findDeadVolumes(g *topology.Graph, adj map[int64][]struct {
	EdgeID   int64
	Neighbor int64
}, path []int64) []int64 {
	// 以所有入口为源做全图可达性：清洗液从任意入口可覆盖的节点集合。
	sources := map[int64]bool{}
	for _, n := range g.Nodes {
		if n.Kind == model.NodeInlet {
			sources[n.ID] = true
		}
	}
	covered := reachableFromAll(g, adj, sources)
	var out []int64
	for _, n := range g.Nodes {
		if n.Kind == model.NodeInlet || n.Kind == model.NodeOutlet {
			continue
		}
		if !covered[n.ID] {
			out = append(out, n.ID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// reachableFromAll 计算从任一源可达的节点集合。
func reachableFromAll(g *topology.Graph, adj map[int64][]struct {
	EdgeID   int64
	Neighbor int64
}, sources map[int64]bool) map[int64]bool {
	visited := map[int64]bool{}
	queue := []int64{}
	for s := range sources {
		if !visited[s] {
			visited[s] = true
			queue = append(queue, s)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, hop := range adj[cur] {
			if visited[hop.Neighbor] {
				continue
			}
			visited[hop.Neighbor] = true
			queue = append(queue, hop.Neighbor)
		}
	}
	return visited
}

// findCrossContamination 检查污染路径：路径上非反应腔节点若同时出现在其他路径可达范围内则交叉污染。
func (e *Engine) findCrossContamination(g *topology.Graph, adj map[int64][]struct {
	EdgeID   int64
	Neighbor int64
}, path []int64, st *model.FlowStep) []int64 {
	onPath := map[int64]bool{}
	for _, nid := range path {
		onPath[nid] = true
	}
	var out []int64
	for _, n := range g.Nodes {
		if n.Kind != model.NodeReactionWell && onPath[n.ID] {
			// 该节点被当前样本流经过，若它还能被其他入口流到达，则存在共享通道。
			for _, other := range g.Nodes {
				if other.Kind != model.NodeInlet || other.ID == st.InletID {
					continue
				}
				_, p := bfs(adj, other.ID, n.ID)
				if len(p) > 0 {
					for _, e := range g.Edges {
						if e.FromNodeID == n.ID || e.ToNodeID == n.ID {
							out = append(out, e.ID)
						}
					}
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return dedupe(out)
}

// findIsolationBreaks 返回路径中穿越隔离区但未声明的边。
func (e *Engine) findIsolationBreaks(g *topology.Graph, path []int64) []int64 {
	if len(path) < 2 || len(g.Zones) == 0 {
		return nil
	}
	onPath := map[int64]bool{}
	for _, nid := range path {
		onPath[nid] = true
	}
	var out []int64
	for _, e := range g.Edges {
		if !onPath[e.FromNodeID] || !onPath[e.ToNodeID] {
			continue
		}
		zA := g.ZoneOfNode[e.FromNodeID]
		zB := g.ZoneOfNode[e.ToNodeID]
		for zidA := range zA {
			for zidB := range zB {
				if zidA == zidB {
					continue
				}
				if !declaredByZones(g, zidA, e.FromNodeID, e.ToNodeID) {
					out = append(out, e.ID)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return dedupe(out)
}

// declaredByZones 检查某个隔离区的声明交叉流路是否包含该边。
func declaredByZones(g *topology.Graph, zoneID, a, b int64) bool {
	for _, z := range g.Zones {
		if z.ID != zoneID {
			continue
		}
		for _, pair := range splitPairs(z.DeclaredCrossings) {
			if (pair[0] == a && pair[1] == b) || (pair[0] == b && pair[1] == a) {
				return true
			}
		}
	}
	return false
}

// splitPairs 解析 "1|2,3|4" 声明。
func splitPairs(s string) [][2]int64 {
	var out [][2]int64
	var cur [2]int64
	idx := 0
	for _, r := range s {
		switch r {
		case '|':
			idx = 1
		case ',':
			out = append(out, cur)
			cur = [2]int64{}
			idx = 0
		default:
			cur[idx] = cur[idx]*10 + int64(r-'0')
		}
	}
	if cur[0] != 0 || cur[1] != 0 {
		out = append(out, cur)
	}
	return out
}

// findResidualWells 检查无法清洗的腔体：反应腔与废液腔若不在清洗入口可达范围内则残留。
func (e *Engine) findResidualWells(g *topology.Graph, adj map[int64][]struct {
	EdgeID   int64
	Neighbor int64
}, path []int64) []int64 {
	sources := map[int64]bool{}
	for _, n := range g.Nodes {
		if n.Kind == model.NodeInlet {
			sources[n.ID] = true
		}
	}
	covered := reachableFromAll(g, adj, sources)
	var out []int64
	for _, n := range g.Nodes {
		if n.Kind == model.NodeReactionWell || n.Kind == model.NodeWasteWell {
			if !covered[n.ID] {
				out = append(out, n.ID)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// dedupe 去重并保持有序。
func dedupe(in []int64) []int64 {
	if len(in) < 2 {
		return in
	}
	out := in[:0]
	var last int64
	for i, v := range in {
		if i == 0 || v != last {
			out = append(out, v)
			last = v
		}
	}
	return out
}
