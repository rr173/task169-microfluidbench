package model

// 合法枚举与状态迁移表：集中管理，避免非法值流入持久化层。

// ValidNodeKinds 返回所有合法的节点类型。
func ValidNodeKinds() []NodeKind {
	return []NodeKind{NodeInlet, NodeOutlet, NodeValve, NodeChannel, NodeReactionWell, NodeWasteWell}
}

// ValidNodeKind 判断节点类型是否合法。
func ValidNodeKind(k NodeKind) bool {
	for _, v := range ValidNodeKinds() {
		if v == k {
			return true
		}
	}
	return false
}

// ValidDirections 返回所有合法方向。
func ValidDirections() []Direction { return []Direction{DirectionOneWay, DirectionTwoWay} }

// ValidDirection 判断方向是否合法。
func ValidDirection(d Direction) bool {
	for _, v := range ValidDirections() {
		if v == d {
			return true
		}
	}
	return false
}

// ValidFluidTypes 返回所有合法流体类别。
func ValidFluidTypes() []FluidType {
	return []FluidType{FluidSample, FluidReagent, FluidRinse, FluidMixture}
}

// ValidFluidType 判断流体类别是否合法。
func ValidFluidType(f FluidType) bool {
	for _, v := range ValidFluidTypes() {
		if v == f {
			return true
		}
	}
	return false
}

// ValidValveState 判断阀门状态是否合法。
func ValidValveState(s ValveState) bool { return s == ValveOpen || s == ValveClosed }

// ValidRiskKinds 返回所有合法风险类型。
func ValidRiskKinds() []string {
	return []string{"dead_volume", "cross_contamination", "residual", "isolation_break", "unreachable"}
}

// ValidRiskKind 判断风险类型是否合法。
func ValidRiskKind(k string) bool {
	for _, v := range ValidRiskKinds() {
		if v == k {
			return true
		}
	}
	return false
}

// ValidSeverities 返回所有合法风险等级。
func ValidSeverities() []string { return []string{"high", "medium", "low"} }

// ValidSeverity 判断风险等级是否合法。
func ValidSeverity(s string) bool {
	for _, v := range ValidSeverities() {
		if v == s {
			return true
		}
	}
	return false
}

// VersionTransitions 定义芯片版本状态机：仅允许以下迁移。
var VersionTransitions = map[string][]string{
	VersionEditing:    {VersionPending},
	VersionPending:    {VersionRiskFound, VersionApproved, VersionEditing},
	VersionRiskFound:  {VersionEditing},
	VersionApproved:   {VersionSuperseded},
	VersionSuperseded: {},
}

// CanTransitVersion 判断版本状态迁移是否合法。
func CanTransitVersion(from, to string) bool {
	for _, v := range VersionTransitions[from] {
		if v == to {
			return true
		}
	}
	return false
}

// StepTransitions 定义流程步骤状态机。
var StepTransitions = map[string][]string{
	StepDraft:    {StepRunnable, StepBlocked},
	StepRunnable: {StepBlocked, StepVerified, StepDraft},
	StepBlocked:  {StepRunnable, StepDraft},
	StepVerified: {StepDraft},
}

// CanTransitStep 判断流程步骤状态迁移是否合法。
func CanTransitStep(from, to string) bool {
	for _, v := range StepTransitions[from] {
		if v == to {
			return true
		}
	}
	return false
}

// RiskTransitions 定义风险状态机。
var RiskTransitions = map[string][]string{
	RiskNew:      {RiskConfirmed, RiskResolved, RiskWaived},
	RiskConfirmed: {RiskResolved, RiskWaived},
	RiskResolved: {},
	RiskWaived:   {},
}

// CanTransitRisk 判断风险状态迁移是否合法。
func CanTransitRisk(from, to string) bool {
	for _, v := range RiskTransitions[from] {
		if v == to {
			return true
		}
	}
	return false
}

// IsFluidRinse 判断流体是否为清洗液。
func IsFluidRinse(f FluidType) bool { return f == FluidRinse }

// IsFluidContaminant 判断流体是否可能造成交叉污染（样本/试剂/混合液）。
func IsFluidContaminant(f FluidType) bool {
	return f == FluidSample || f == FluidReagent || f == FluidMixture
}
