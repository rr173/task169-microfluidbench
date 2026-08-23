package model

import (
	"errors"
	"fmt"
)

// 业务错误集合：HTTP 层通过 errors.Is 映射到 400/404/409/422。
var (
	// ErrNotFound 表示目标实体不存在。
	ErrNotFound = errors.New("not found")
	// ErrConflict 表示乐观版本冲突或状态冲突。
	ErrConflict = errors.New("version conflict")
	// ErrInvalid 表示输入校验失败。
	ErrInvalid = errors.New("invalid input")
	// ErrFrozen 表示版本已冻结，不可修改。
	ErrFrozen = errors.New("version frozen")
	// ErrDangling 表示边引用了不存在的节点。
	ErrDangling = errors.New("dangling edge endpoint")
	// ErrDuplicatePort 表示同一节点端口被重复使用且方向冲突。
	ErrDuplicatePort = errors.New("duplicate port direction")
	// ErrValveMissing 表示流程步骤未声明途经阀门的开关状态。
	ErrValveMissing = errors.New("valve state missing")
	// ErrIsolationCross 表示流路穿越隔离区但未声明。
	ErrIsolationCross = errors.New("undeclared isolation crossing")
	// ErrNoRoute 表示图上不存在可行流路。
	ErrNoRoute = errors.New("no route found")
	// ErrStateTransition 表示非法状态迁移。
	ErrStateTransition = errors.New("illegal state transition")
)

// NewNotFound 构造带资源的 not found 错误。
func NewNotFound(what string, id int64) error {
	return fmt.Errorf("%w: %s %d", ErrNotFound, what, id)
}

// NewConflict 构造带描述的冲突错误。
func NewConflict(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrConflict, fmt.Sprintf(format, args...))
}

// NewInvalid 构造输入校验错误。
func NewInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// IsNotFound 判断错误是否为实体不存在。
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsConflict 判断错误是否为并发/状态冲突。
func IsConflict(err error) bool { return errors.Is(err, ErrConflict) }

// IsInvalid 判断错误是否为输入非法。
func IsInvalid(err error) bool { return errors.Is(err, ErrInvalid) }
