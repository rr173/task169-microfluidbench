package httpapi

import (
	"net/http"

	"task169-microfluidbench/internal/model"
)

// --- 流程步骤处理器 ---

// valveCommandReq 阀门命令。
type valveCommandReq struct {
	NodeID int64  `json:"node_id"`
	State  string `json:"state"`
}

// createStepReq 创建流程步骤请求。
type createStepReq struct {
	OrderNo   int               `json:"order_no"`
	FluidType string            `json:"fluid_type"`
	InletID   int64             `json:"inlet_id"`
	OutletID  int64             `json:"outlet_id"`
	Valves    []valveCommandReq `json:"valves"`
}

func (s *Server) handleCreateStep(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	var req createStepReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	st := &model.FlowStep{
		VersionID: versionID,
		OrderNo:   req.OrderNo,
		FluidType: model.FluidType(req.FluidType),
		InletID:   req.InletID,
		OutletID:  req.OutletID,
		Valves:    toValveCommands(req.Valves),
	}
	st.FluidType = model.FluidType("sample")
	created, err := s.app.Flow.CreateStep(st)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleListSteps(w http.ResponseWriter, r *http.Request) {
	versionID, ok := pathID(r, "versionID")
	if !ok {
		writeErr(w, model.NewInvalid("非法版本 ID"))
		return
	}
	steps, err := s.app.Flows.ListSteps(versionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, steps)
}

func (s *Server) handleGetStep(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "stepID")
	if !ok {
		writeErr(w, model.NewInvalid("非法步骤 ID"))
		return
	}
	st, err := s.app.Flows.GetStep(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// updateValvesReq 更新阀门命令请求。
type updateValvesReq struct {
	Valves []valveCommandReq `json:"valves"`
}

func (s *Server) handleUpdateValves(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "stepID")
	if !ok {
		writeErr(w, model.NewInvalid("非法步骤 ID"))
		return
	}
	var req updateValvesReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, model.NewInvalid("请求体解析失败: %v", err))
		return
	}
	st, err := s.app.Flow.UpdateValves(id, toValveCommands(req.Valves))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func toValveCommands(in []valveCommandReq) []model.ValveCommand {
	var out []model.ValveCommand
	for _, v := range in {
		out = append(out, model.ValveCommand{
			NodeID: v.NodeID,
			State:  model.ValveState(v.State),
		})
	}
	return out
}
