package httpapi

import (
	"net/http"
	"strings"

	"task169-microfluidbench/internal/model"
)

// --- 拓扑处理器：节点、边、隔离区、画布 ---

// createNodeReq 创建节点请求。
type createNodeReq struct {
	Kind string  `json:"kind"`
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

func (s *Server) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	var req createNodeReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	n, err := s.app.Topology.CreateNode(versionID, model.NodeKind(req.Kind), req.Name, req.X, req.Y)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	nodes, err := s.app.Topo.ListNodes(versionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "nodeID")
	if !ok {
		writeErr(w, model.NewInvalid("非法节点 ID"))
		return
	}
	n, err := s.app.Topo.GetNode(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// updateNodeReq 更新节点请求。
type updateNodeReq struct {
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

func (s *Server) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "nodeID")
	if !ok {
		writeErr(w, model.NewInvalid("非法节点 ID"))
		return
	}
	n, err := s.app.Topo.GetNode(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if _, err := s.app.Topology.EnsureEditable(n.VersionID); err != nil {
		writeErr(w, err)
		return
	}
	var req updateNodeReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	if err := s.app.Topo.UpdateNode(id, req.Name, req.X, req.Y); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "updated": true})
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "nodeID")
	if !ok {
		writeErr(w, model.NewInvalid("非法节点 ID"))
		return
	}
	n, err := s.app.Topo.GetNode(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if _, err := s.app.Topology.EnsureEditable(n.VersionID); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.app.Topo.DeleteNode(id); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

// createEdgeReq 创建边请求。
type createEdgeReq struct {
	FromNodeID int64   `json:"from_node_id"`
	FromPort   string  `json:"from_port"`
	ToNodeID   int64   `json:"to_node_id"`
	ToPort     string  `json:"to_port"`
	Direction  string  `json:"direction"`
	Width      float64 `json:"width"`
	Comment    string  `json:"comment"`
	ExpectedRev int    `json:"expected_rev"`
}

func (s *Server) handleCreateEdge(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	var req createEdgeReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	e := &model.Edge{
		VersionID:  versionID,
		FromNodeID: req.FromNodeID,
		FromPort:   model.EdgePort(req.FromPort),
		ToNodeID:   req.ToNodeID,
		ToPort:     model.EdgePort(req.ToPort),
		Direction:  model.Direction(req.Direction),
		Width:      req.Width,
		Comment:    req.Comment,
	}
	ne, err := s.app.Topology.CreateEdge(versionID, e, req.ExpectedRev)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ne)
}

func (s *Server) handleListEdges(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	edges, err := s.app.Topo.ListEdges(versionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, edges)
}

func (s *Server) handleGetEdge(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "edgeID")
	if !ok {
		writeErr(w, model.NewInvalid("非法边 ID"))
		return
	}
	e, err := s.app.Topo.GetEdge(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

// deleteEdgeReq 删除边请求（乐观锁）。
type deleteEdgeReq struct {
	ExpectedRev int `json:"expected_rev"`
}

func (s *Server) handleDeleteEdge(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "edgeID")
	if !ok {
		writeErr(w, model.NewInvalid("非法边 ID"))
		return
	}
	e, err := s.app.Topo.GetEdge(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	var req deleteEdgeReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	if err := s.app.Topology.DeleteEdge(e.VersionID, id, req.ExpectedRev); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

// createZoneReq 创建隔离区请求。
type createZoneReq struct {
	Name              string  `json:"name"`
	Reason            string  `json:"reason"`
	MemberNodeIDs     []int64 `json:"member_node_ids"`
	DeclaredCrossings string  `json:"declared_crossings"`
}

func (s *Server) handleCreateZone(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	if _, err := s.app.Topology.EnsureEditable(versionID); err != nil {
		writeErr(w, err)
		return
	}
	var req createZoneReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	if err := model.ValidateZoneInput(req.Name, req.Reason); err != nil {
		writeErr(w, err)
		return
	}
	z, err := s.app.Topo.CreateZone(&model.IsolationZone{
		VersionID:         versionID,
		Name:              req.Name,
		Reason:            req.Reason,
		DeclaredCrossings: req.DeclaredCrossings,
	}, req.MemberNodeIDs)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, z)
}

func (s *Server) handleListZones(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	zones, err := s.app.Topo.ListZones(versionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, zones)
}

func (s *Server) handleDeleteZone(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "zoneID")
	if !ok {
		writeErr(w, model.NewInvalid("非法隔离区 ID"))
		return
	}
	z, err := s.app.Topo.GetZone(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if _, err := s.app.Topology.EnsureEditable(z.VersionID); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.app.Topo.DeleteZone(id); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

// handleGraph 返回画布联动数据：节点、边、隔离区与风险节点。
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	g, err := s.app.Topology.LoadGraph(versionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	risks, err := s.app.Risks.ListByVersion(versionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	// 把风险涉及的边两端节点标记为风险节点，供前端联动高亮。
	riskNodeIDs := map[int64]bool{}
	for _, rk := range risks {
		for _, e := range g.Edges {
			if riskEdgeReferenced(rk.Evidence, e.ID) {
				riskNodeIDs[e.FromNodeID] = true
				riskNodeIDs[e.ToNodeID] = true
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version_id": g.VersionID,
		"nodes":      g.Nodes,
		"edges":      g.Edges,
		"zones":      g.Zones,
		"risk_count": len(risks),
		"risk_nodes": riskNodeIDs,
	})
}

// riskEdgeReferenced 判断风险证据文本是否引用了指定边 ID。
func riskEdgeReferenced(evidence string, edgeID int64) bool {
	if evidence == "" {
		return false
	}
	return strings.Contains(evidence, intToStr(edgeID))
}

// intToStr 整数转十进制字符串。
func intToStr(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
