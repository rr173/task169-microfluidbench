// Package risk 管理风险记录：创建、确认、解决、豁免与处置跟踪。
package risk

import (
	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/store"
	"task169-microfluidbench/internal/validation"
)

// Service 封装风险领域操作。
type Service struct {
	risks *store.RiskStore
	valid *validation.Service
	chips *store.ChipsStore
}

// NewService 构造风险服务。
func NewService(risks *store.RiskStore, valid *validation.Service, chips *store.ChipsStore) *Service {
	return &Service{risks: risks, valid: valid, chips: chips}
}

// CreateFromValidation 根据校验结果自动创建风险，返回创建列表。
func (s *Service) CreateFromValidation(validationID int64) ([]*model.Risk, error) {
	vr, err := s.valid.GetValidation(validationID)
	if err != nil {
		return nil, err
	}
	suggestions := s.valid.SuggestRisk(vr)
	var created []*model.Risk
	for _, sug := range suggestions {
		r, err := s.risks.CreateRisk(&sug)
		if err != nil {
			return nil, err
		}
		created = append(created, r)
	}
	// 若当前版本是风险发现，无需额外动作；若还是待校验但出现风险，置为风险发现。
	if len(created) > 0 {
		v, err := s.chips.GetVersion(vr.VersionID)
		if err == nil && v.Status == model.VersionPending {
			_ = s.chips.UpdateVersionStatus(vr.VersionID, model.VersionRiskFound)
		}
	}
	return created, nil
}

// Create 手动创建风险。
func (s *Service) Create(r *model.Risk) (*model.Risk, error) {
	if !model.ValidSeverity(r.Severity) {
		return nil, model.NewInvalid("非法风险等级 %q", r.Severity)
	}
	if r.Status == "" {
		r.Status = model.RiskNew
	}
	return s.risks.CreateRisk(r)
}

// ListByVersion 列出版本的风险。
func (s *Service) ListByVersion(versionID int64) ([]*model.Risk, error) {
	return s.risks.ListByVersion(versionID)
}

// ListByValidation 列出校验结果的风险。
func (s *Service) ListByValidation(validationID int64) ([]*model.Risk, error) {
	return s.risks.ListByValidation(validationID)
}

// Transit 执行风险状态迁移（new -> confirmed/resolved/waived 等）。
func (s *Service) Transit(id int64, to, owner, resolution string) (*model.Risk, error) {
	r, err := s.risks.GetRisk(id)
	if err != nil {
		return nil, err
	}
	if !model.CanTransitRisk(r.Status, to) {
		return nil, model.NewConflict("风险状态不允许 %s -> %s", r.Status, to)
	}
	if err := s.risks.UpdateRiskStatus(id, to, owner, resolution); err != nil {
		return nil, err
	}
	return s.risks.GetRisk(id)
}

// CountOpen 统计版本未关闭风险数。
func (s *Service) CountOpen(versionID int64) (int, error) {
	return s.risks.CountOpenRisks(versionID)
}
