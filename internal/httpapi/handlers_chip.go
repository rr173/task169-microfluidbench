package httpapi

import (
	"net/http"

	"task169-microfluidbench/internal/model"
)

// --- 芯片与版本处理器 ---

// createChipReq 创建芯片请求。
type createChipReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleCreateChip(w http.ResponseWriter, r *http.Request) {
	var req createChipReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	if err := model.ValidateChipInput(req.Name, req.Description); err != nil {
		writeErr(w, err)
		return
	}
	chip, err := s.app.Chips.CreateChip(req.Name, req.Description)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, chip)
}

func (s *Server) handleListChips(w http.ResponseWriter, r *http.Request) {
	chips, err := s.app.Chips.ListChips()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, chips)
}

func (s *Server) handleGetChip(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "chipID")
	if !ok {
		writeErr(w, model.NewInvalid("非法芯片 ID"))
		return
	}
	chip, err := s.app.Chips.GetChip(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, chip)
}

// createVersionReq 创建版本请求。
type createVersionReq struct {
	Note string `json:"note"`
}

func (s *Server) handleCreateVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "chipID")
	if !ok {
		writeErr(w, model.NewInvalid("非法芯片 ID"))
		return
	}
	var req createVersionReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	v, err := s.app.Chips.CreateVersion(id, req.Note)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "chipID")
	if !ok {
		writeErr(w, model.NewInvalid("非法芯片 ID"))
		return
	}
	vs, err := s.app.Chips.ListVersions(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vs)
}

func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	v, err := s.app.Chips.GetVersion(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// transitionVersionReq 版本状态迁移请求。
type transitionVersionReq struct {
	To string `json:"to"`
}

func (s *Server) handleTransitionVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	var req transitionVersionReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	v, err := s.app.Chips.GetVersion(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if !model.CanTransitVersion(v.Status, req.To) {
		writeErr(w, model.NewConflict("版本状态不允许 %s -> %s", v.Status, req.To))
		return
	}
	if err := s.app.Chips.UpdateVersionStatus(id, req.To); err != nil {
		writeErr(w, err)
		return
	}
	// 重新读取，确保返回的是已持久化的目标状态，而非请求值。
	updated, err := s.app.Chips.GetVersion(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":     id,
		"status": updated.Status,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Health())
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.app.Stats()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
