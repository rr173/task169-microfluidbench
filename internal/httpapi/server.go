// Package httpapi 提供 HTTP 路由与处理器，统一 /api 前缀。
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/webui"
)

// Server 封装 HTTP 路由。
type Server struct {
	app *service.App
	mux *http.ServeMux
}

// New 构造 HTTP 服务器并注册全部路由。
func New(app *service.App) *Server {
	s := &Server{app: app, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 返回 HTTP 处理器。
func (s *Server) Handler() http.Handler { return s.mux }

// routes 注册全部 /api 路由与页面路由。
func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)

	// 芯片与版本
	s.mux.HandleFunc("POST /api/chips", s.handleCreateChip)
	s.mux.HandleFunc("GET /api/chips", s.handleListChips)
	s.mux.HandleFunc("GET /api/chips/{chipID}", s.handleGetChip)
	s.mux.HandleFunc("POST /api/chips/{chipID}/versions", s.handleCreateVersion)
	s.mux.HandleFunc("GET /api/chips/{chipID}/versions", s.handleListVersions)
	s.mux.HandleFunc("GET /api/versions/{versionID}", s.handleGetVersion)
	s.mux.HandleFunc("POST /api/versions/{versionID}/transition", s.handleTransitionVersion)

	// 拓扑：节点
	s.mux.HandleFunc("POST /api/versions/{versionID}/nodes", s.handleCreateNode)
	s.mux.HandleFunc("GET /api/versions/{versionID}/nodes", s.handleListNodes)
	s.mux.HandleFunc("GET /api/nodes/{nodeID}", s.handleGetNode)
	s.mux.HandleFunc("PUT /api/nodes/{nodeID}", s.handleUpdateNode)
	s.mux.HandleFunc("DELETE /api/nodes/{nodeID}", s.handleDeleteNode)

	// 拓扑：边
	s.mux.HandleFunc("POST /api/versions/{versionID}/edges", s.handleCreateEdge)
	s.mux.HandleFunc("GET /api/versions/{versionID}/edges", s.handleListEdges)
	s.mux.HandleFunc("GET /api/edges/{edgeID}", s.handleGetEdge)
	s.mux.HandleFunc("DELETE /api/edges/{edgeID}", s.handleDeleteEdge)

	// 拓扑：隔离区
	s.mux.HandleFunc("POST /api/versions/{versionID}/zones", s.handleCreateZone)
	s.mux.HandleFunc("GET /api/versions/{versionID}/zones", s.handleListZones)
	s.mux.HandleFunc("DELETE /api/zones/{zoneID}", s.handleDeleteZone)

	// 画布联动
	s.mux.HandleFunc("GET /api/versions/{versionID}/graph", s.handleGraph)

	// 流程步骤
	s.mux.HandleFunc("POST /api/versions/{versionID}/steps", s.handleCreateStep)
	s.mux.HandleFunc("GET /api/versions/{versionID}/steps", s.handleListSteps)
	s.mux.HandleFunc("GET /api/steps/{stepID}", s.handleGetStep)
	s.mux.HandleFunc("PUT /api/steps/{stepID}/valves", s.handleUpdateValves)

	// 校验
	s.mux.HandleFunc("POST /api/steps/{stepID}/validate", s.handleValidateStep)
	s.mux.HandleFunc("POST /api/versions/{versionID}/validate-all", s.handleValidateAll)
	s.mux.HandleFunc("GET /api/versions/{versionID}/validations", s.handleListValidations)
	s.mux.HandleFunc("GET /api/validations/{validationID}", s.handleGetValidation)

	// 风险
	s.mux.HandleFunc("POST /api/validations/{validationID}/risks", s.handleCreateRisksFromValidation)
	s.mux.HandleFunc("GET /api/versions/{versionID}/risks", s.handleListRisks)
	s.mux.HandleFunc("GET /api/risks/{riskID}", s.handleGetRisk)
	s.mux.HandleFunc("PUT /api/risks/{riskID}/transit", s.handleTransitRisk)
	s.mux.HandleFunc("POST /api/versions/{versionID}/risks", s.handleCreateRisk)

	// 快照
	s.mux.HandleFunc("POST /api/versions/{versionID}/snapshots", s.handleFreezeSnapshot)
	s.mux.HandleFunc("GET /api/versions/{versionID}/snapshots", s.handleListSnapshots)
	s.mux.HandleFunc("GET /api/snapshots/{snapshotID}", s.handleGetSnapshot)

	// Web 页面
	s.mux.HandleFunc("GET /", s.handlePage)
}

// handlePage 返回拓扑画布页面。
func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	webui.Handler().ServeHTTP(w, r)
}

// pathID 从请求路径中提取 int64 参数。
func pathID(r *http.Request, key string) (int64, bool) {
	v := r.PathValue(key)
	if v == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// decodeBody 解析 JSON 请求体。
func decodeBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// writeJSON 写 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr 按错误类型映射 HTTP 状态码。
func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, model.ErrConflict), errors.Is(err, model.ErrFrozen),
		errors.Is(err, model.ErrStateTransition), errors.Is(err, model.ErrDuplicatePort):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, model.ErrDangling), errors.Is(err, model.ErrValveMissing),
		errors.Is(err, model.ErrIsolationCross), errors.Is(err, model.ErrNoRoute),
		errors.Is(err, model.ErrInvalid):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}
