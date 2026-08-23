// Package service 编排各业务包，向 HTTP 层提供统一门面。
package service

import (
	"task169-microfluidbench/internal/flow"
	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/risk"
	"task169-microfluidbench/internal/snapshot"
	"task169-microfluidbench/internal/store"
	"task169-microfluidbench/internal/topology"
	"task169-microfluidbench/internal/validation"
)

// App 是领域服务的组合根。
type App struct {
	Store      *store.DB
	Chips      *store.ChipsStore
	Topo       *store.TopologyStore
	Flows      *store.FlowStore
	Valid      *store.ValidationStore
	Risks      *store.RiskStore
	Snaps      *store.SnapshotStore
	Topology   *topology.Service
	Flow       *flow.Service
	Validation *validation.Service
	Risk       *risk.Service
	Snapshot   *snapshot.Service
}

// New 构造 App，注入全部存储与服务。
func New(db *store.DB) *App {
	chips := store.NewChipsStore(db)
	topoStore := store.NewTopologyStore(db)
	flows := store.NewFlowStore(db)
	valid := store.NewValidationStore(db)
	risks := store.NewRiskStore(db)
	snaps := store.NewSnapshotStore(db)

	topoSvc := topology.NewService(chips, topoStore)
	flowSvc := flow.NewService(flows, topoSvc)
	validSvc := validation.NewService(flows, valid, chips, topoSvc, flowSvc)
	riskSvc := risk.NewService(risks, validSvc, chips)
	snapSvc := snapshot.NewService(snaps, chips, topoSvc, flows)

	return &App{
		Store:      db,
		Chips:      chips,
		Topo:       topoStore,
		Flows:      flows,
		Valid:      valid,
		Risks:      risks,
		Snaps:      snaps,
		Topology:   topoSvc,
		Flow:       flowSvc,
		Validation: validSvc,
		Risk:       riskSvc,
		Snapshot:   snapSvc,
	}
}

// Close 关闭数据库。
func (a *App) Close() error { return a.Store.Close() }

// Health 返回系统健康信息。
func (a *App) Health() map[string]any {
	return map[string]any{
		"status": "ok",
		"module": "task169-microfluidbench",
	}
}

// Stats 返回全局统计（版本数、风险数、快照数）。
//
// 统计面向只读展示，必须保证稳定可用：任意子查询失败时不应整体崩溃，
// 仅以缺省值（0）参与聚合，使调用方始终拿到 total_risks 等计数字段。
func (a *App) Stats() (map[string]any, error) {
	versions, err := a.Chips.ListVersionsAll()
	if err != nil {
		return nil, err
	}
	snapCount := 0
	riskCount := 0
	for _, v := range versions {
		if rs, err := a.Risks.ListByVersion(v.ID); err == nil {
			riskCount += len(rs)
		}
		if snaps, err := a.Snaps.ListByVersion(v.ID); err == nil {
			snapCount += len(snaps)
		}
	}
	return map[string]any{
		"chip_versions":    len(versions),
		"frozen_snapshots": snapCount,
		"total_risks":      riskCount,
	}, nil
}

// EnsureType 编译期类型检查占位（保证 model 包被编排层引用）。
var _ model.FluidType = model.FluidSample
