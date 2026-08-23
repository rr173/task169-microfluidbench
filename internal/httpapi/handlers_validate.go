package httpapi

import (
	"net/http"

	"task169-microfluidbench/internal/model"
)

// --- 校验处理器 ---

func (s *Server) handleValidateStep(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "stepID")
	if !ok {
		writeErr(w, model.NewInvalid("非法步骤 ID"))
		return
	}
	vr, err := s.app.Validation.ValidateStep(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vr)
}

func (s *Server) handleValidateAll(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	allPass, err := s.app.Validation.ValidateAll(versionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version_id": versionID,
		"all_passed": allPass,
	})
}

func (s *Server) handleListValidations(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	vs, err := s.app.Valid.ListByVersion(versionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vs)
}

func (s *Server) handleGetValidation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "validationID")
	if !ok {
		writeErr(w, model.NewInvalid("非法校验 ID"))
		return
	}
	vr, err := s.app.Valid.GetValidation(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vr)
}

// --- 风险处理器 ---

func (s *Server) handleCreateRisksFromValidation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "validationID")
	if !ok {
		writeErr(w, model.NewInvalid("非法校验 ID"))
		return
	}
	created, err := s.app.Risk.CreateFromValidation(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleListRisks(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	risks, err := s.app.Risk.ListByVersion(versionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, risks)
}

func (s *Server) handleGetRisk(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "riskID")
	if !ok {
		writeErr(w, model.NewInvalid("非法风险 ID"))
		return
	}
	rk, err := s.app.Risks.GetRisk(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rk)
}

// transitRiskReq 风险状态迁移请求。
type transitRiskReq struct {
	To         string `json:"to"`
	Owner      string `json:"owner"`
	Resolution string `json:"resolution"`
}

func (s *Server) handleTransitRisk(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "riskID")
	if !ok {
		writeErr(w, model.NewInvalid("非法风险 ID"))
		return
	}
	var req transitRiskReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	updated, err := s.app.Risk.Transit(id, req.To, req.Owner, req.Resolution)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// createRiskReq 手动创建风险请求。
type createRiskReq struct {
	ValidationID int64  `json:"validation_id"`
	Kind         string `json:"kind"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	Evidence     string `json:"evidence"`
}

func (s *Server) handleCreateRisk(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	var req createRiskReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	rk := &model.Risk{
		VersionID:    versionID,
		ValidationID: req.ValidationID,
		Kind:         req.Kind,
		Severity:     req.Severity,
		Title:        req.Title,
		Evidence:     req.Evidence,
	}
	created, err := s.app.Risk.Create(rk)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// --- 快照处理器 ---

// freezeSnapshotReq 冻结快照请求。
type freezeSnapshotReq struct {
	FrozenBy string `json:"frozen_by"`
}

func (s *Server) handleFreezeSnapshot(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	var req freezeSnapshotReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	snap, err := s.app.Snapshot.BuildAndFreeze(versionID, req.FrozenBy)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	snaps, err := s.app.Snapshot.ListByVersion(versionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

func (s *Server) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "snapshotID")
	if !ok {
		writeErr(w, model.NewInvalid("非法快照 ID"))
		return
	}
	snap, err := s.app.Snapshot.Get(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
