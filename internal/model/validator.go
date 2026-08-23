package model

import "strings"

// 实体字段校验：在写入 store 之前由 service 层调用，避免脏数据入库。

// ValidateChipInput 校验芯片主体字段。
func ValidateChipInput(name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return NewInvalid("chip name 不能为空")
	}
	if len(name) > 64 {
		return NewInvalid("chip name 超过 64 字符上限")
	}
	if len(description) > 512 {
		return NewInvalid("chip description 超过 512 字符上限")
	}
	return nil
}

// ValidateNodeInput 校验节点字段。
func ValidateNodeInput(kind NodeKind, name string) error {
	if !ValidNodeKind(kind) {
		return NewInvalid("非法节点类型 %q", string(kind))
	}
	if strings.TrimSpace(name) == "" {
		return NewInvalid("node name 不能为空")
	}
	if len(name) > 64 {
		return NewInvalid("node name 超过 64 字符上限")
	}
	return nil
}

// ValidateEdgeInput 校验边字段（端点存在性由 service 层结合数据库校验）。
func ValidateEdgeInput(from, to int64, fromPort, toPort EdgePort, dir Direction, width float64) error {
	if from == to {
		return NewInvalid("边不能自环：from 与 to 相同")
	}
	if from <= 0 || to <= 0 {
		return NewInvalid("边端点必须为正 ID")
	}
	if fromPort == "" || toPort == "" {
		return NewInvalid("边端点端口不能为空")
	}
	if !ValidDirection(dir) {
		return NewInvalid("非法方向 %q", string(dir))
	}
	if width <= 0 {
		return NewInvalid("通道宽度必须大于 0")
	}
	return nil
}

// ValidateZoneInput 校验隔离区字段。
func ValidateZoneInput(name, reason string) error {
	if strings.TrimSpace(name) == "" {
		return NewInvalid("isolation zone name 不能为空")
	}
	if len(name) > 64 {
		return NewInvalid("isolation zone name 超过 64 字符上限")
	}
	if len(reason) > 256 {
		return NewInvalid("isolation zone reason 超过 256 字符上限")
	}
	return nil
}

// ValidateStepInput 校验流程步骤字段。
func ValidateStepInput(fluid FluidType, order int, inlet, outlet int64) error {
	if !ValidFluidType(fluid) {
		return NewInvalid("非法流体类别 %q", string(fluid))
	}
	if order <= 0 {
		return NewInvalid("步骤序号必须为正整数")
	}
	if inlet <= 0 || outlet <= 0 {
		return NewInvalid("步骤必须声明入口与出口")
	}
	return nil
}

// ValidateSnapshotTransition 校验快照状态迁移。
func ValidateSnapshotTransition(from, to string) error {
	if from == SnapshotBuilding && to == SnapshotFrozen {
		return nil
	}
	return NewInvalid("快照仅允许 building -> frozen 迁移，当前 %s -> %s", from, to)
}
