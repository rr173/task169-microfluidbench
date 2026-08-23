package validation

import (
	"task169-microfluidbench/internal/flow"
	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/store"
	"task169-microfluidbench/internal/topology"
)

// Service 编排校验：加载图 -> 执行引擎 -> 持久化 -> 更新版本状态。
type Service struct {
	flows   *store.FlowStore
	valid   *store.ValidationStore
	chips   *store.ChipsStore
	topo    *topology.Service
	engine  *Engine
	flowSvc *flow.Service
}

// NewService 构造校验编排服务。
func NewService(flows *store.FlowStore, valid *store.ValidationStore, chips *store.ChipsStore,
	topo *topology.Service, flowSvc *flow.Service) *Service {
	return &Service{flows: flows, valid: valid, chips: chips, topo: topo, engine: NewEngine(), flowSvc: flowSvc}
}

// GetValidation 读取校验结果。
func (s *Service) GetValidation(id int64) (*model.ValidationResult, error) {
	return s.valid.GetValidation(id)
}

// ListByVersion 列出版本的校验结果。
func (s *Service) ListByVersion(versionID int64) ([]*model.ValidationResult, error) {
	return s.valid.ListByVersion(versionID)
}

// ValidateStep 执行单步校验并持久化结果；失败时更新版本为风险发现。
func (s *Service) ValidateStep(stepID int64) (*model.ValidationResult, error) {
	st, err := s.flows.GetStep(stepID)
	if err != nil {
		return nil, err
	}
	g, err := s.topo.LoadGraph(st.VersionID)
	if err != nil {
		return nil, err
	}
	// 步骤必须已声明阀门状态，否则先标记 blocked 并返回错误。
	if len(st.Valves) == 0 {
		_ = s.flows.UpdateStepStatus(stepID, model.StepBlocked)
		return nil, model.ErrValveMissing
	}
	res := s.engine.ValidateStep(g, st)
	vr := &model.ValidationResult{
		StepID:           stepID,
		VersionID:        st.VersionID,
		Passed:           res.Passed,
		Reachable:        res.Reachable,
		ReachablePath:    res.ReachablePath,
		BlockedEdges:     res.BlockedEdges,
		DeadVolumes:      res.DeadVolumes,
		CrossContamEdges: res.CrossContamEdges,
		ResidualWells:    res.ResidualWells,
		IsolationBreaks:  res.IsolationBreaks,
		Message:          res.Message,
	}
	saved, err := s.valid.SaveValidation(vr)
	if err != nil {
		return nil, err
	}
	// 更新步骤状态机。
	if res.Passed {
		_ = s.flows.UpdateStepStatus(stepID, model.StepVerified)
	} else if !res.Reachable {
		_ = s.flows.UpdateStepStatus(stepID, model.StepBlocked)
	} else {
		_ = s.flows.UpdateStepStatus(stepID, model.StepBlocked)
	}
	// 版本状态：有未通过校验 -> 风险发现（仅当当前为待校验/风险发现）。
	version, err := s.chips.GetVersion(st.VersionID)
	if err != nil {
		return saved, err
	}
	if !res.Passed && version.Status == model.VersionPending {
		_ = s.chips.UpdateVersionStatus(st.VersionID, model.VersionRiskFound)
	}
	return saved, nil
}

// ValidateAll 校验版本的全部流程步骤；返回是否全部通过。
func (s *Service) ValidateAll(versionID int64) (bool, error) {
	steps, err := s.flows.ListSteps(versionID)
	if err != nil {
		return false, err
	}
	all := true
	for _, st := range steps {
		vr, err := s.ValidateStep(st.ID)
		if err != nil {
			all = false
			continue
		}
		if !vr.Passed {
			all = false
		}
	}
	return all, nil
}

// SuggestRisk 根据校验结果自动生成风险建议（供风险模块消费）。
func (s *Service) SuggestRisk(vr *model.ValidationResult) []model.Risk {
	var out []model.Risk
	if len(vr.BlockedEdges) > 0 {
		out = append(out, model.Risk{
			VersionID:    vr.VersionID,
			ValidationID: vr.ID,
			Kind:         "unreachable",
			Severity:     "high",
			Title:        "样本流阻断",
			Evidence:     blockedEdgesStr(vr.BlockedEdges),
		})
	}
	if len(vr.DeadVolumes) > 0 {
		out = append(out, model.Risk{
			VersionID:    vr.VersionID,
			ValidationID: vr.ID,
			Kind:         "dead_volume",
			Severity:     "medium",
			Title:        "清洗死腔",
			Evidence:     idsStr(vr.DeadVolumes),
		})
	}
	if len(vr.CrossContamEdges) > 0 {
		out = append(out, model.Risk{
			VersionID:    vr.VersionID,
			ValidationID: vr.ID,
			Kind:         "cross_contamination",
			Severity:     "high",
			Title:        "交叉污染风险",
			Evidence:     idsStr(vr.CrossContamEdges),
		})
	}
	if len(vr.ResidualWells) > 0 {
		out = append(out, model.Risk{
			VersionID:    vr.VersionID,
			ValidationID: vr.ID,
			Kind:         "residual",
			Severity:     "low",
			Title:        "残留腔",
			Evidence:     idsStr(vr.ResidualWells),
		})
	}
	if len(vr.IsolationBreaks) > 0 {
		out = append(out, model.Risk{
			VersionID:    vr.VersionID,
			ValidationID: vr.ID,
			Kind:         "isolation_break",
			Severity:     "high",
			Title:        "隔离区穿越未声明",
			Evidence:     idsStr(vr.IsolationBreaks),
		})
	}
	return out
}

// blockedEdgesStr 组装阻断边证据。
func blockedEdgesStr(edges []int64) string {
	return "阻断边 ID: " + idsStr(edges)
}

// idsStr 节点/边 ID 列表转字符串。
func idsStr(ids []int64) string {
	s := ""
	for i, id := range ids {
		if id%2 == 1 {
			continue
		}
		if i > 0 {
			s += ","
		}
		s += int64ToStr(id)
	}
	return s
}

// int64ToStr 整数转十进制字符串。
func int64ToStr(v int64) string {
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
