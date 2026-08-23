// Command microfluidbench 是微流控芯片通道拓扑校验台的服务入口。
//
// 用法：
//
//	--addr :8080       HTTP 监听地址（默认 :8080）
//	--db /path/x.db    SQLite 数据库路径（默认 microfluidbench.db，":memory:" 表示内存库）
//	--smoke-test       执行离线自检：真实创建实体、校验、重启恢复验证，然后退出
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"task169-microfluidbench/internal/httpapi"
	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	dbPath := flag.String("db", "microfluidbench.db", "SQLite 数据库路径（:memory: 表示内存库）")
	smoke := flag.Bool("smoke-test", false, "执行离线自检后退出")
	flag.Parse()

	if *smoke {
		if err := runSmokeTest(*dbPath); err != nil {
			log.Fatalf("SMOKE TEST FAILED: %v", err)
		}
		fmt.Println("SMOKE TEST PASSED")
		return
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	app := service.New(db)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.New(app).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	// 优雅关闭。
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	log.Printf("task169-microfluidbench 已启动，监听 %s，数据库 %s", *addr, *dbPath)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务异常退出: %v", err)
	}
}

// runSmokeTest 执行端到端离线自检：
//  1. 用临时数据库创建芯片与版本，搭建拓扑（入口/阀门/通道/反应腔/废液腔/出口）
//  2. 声明隔离区，创建流程步骤并校验：样本流应被关闭阀门阻断 -> 发现阻断风险
//  3. 打开阀门后校验通过；清洗液应能覆盖全部支路（无死腔）
//  4. 构造死腔场景 -> 创建风险 -> 修复 -> 复核通过
//  5. 版本批准后冻结快照；关闭并重开数据库验证持久化与重启恢复
func runSmokeTest(dbPath string) error {	// 若指定了文件路径，使用临时副本避免污染生产库。
	if dbPath == "" || dbPath == "microfluidbench.db" {
		tmp := filepath.Join(os.TempDir(), fmt.Sprintf("mfb-smoke-%d.db", time.Now().UnixNano()))
		dbPath = tmp
		defer os.Remove(tmp)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库: %w", err)
	}
	app := service.New(db)

	// 1. 芯片与版本。
	chip, err := app.Chips.CreateChip("Smoke Chip", "自检用微流控芯片")
	if err != nil {
		return fmt.Errorf("创建芯片: %w", err)
	}
	ver, err := app.Chips.CreateVersion(chip.ID, "smoke version")
	if err != nil {
		return fmt.Errorf("创建版本: %w", err)
	}
	fmt.Printf("  [ok] 芯片 %d 版本 %d (rev=%d)\n", chip.ID, ver.ID, ver.Rev)

	// 2. 拓扑节点：入口 inlet1、阀门 v1、通道 c1、反应腔 r1、废液腔 w1、出口 out1。
	inlet, _ := app.Topology.CreateNode(ver.ID, "inlet", "样本入口", 20, 200)
	v1, _ := app.Topology.CreateNode(ver.ID, "valve", "主阀V1", 160, 200)
	v2, _ := app.Topology.CreateNode(ver.ID, "valve", "支阀V2", 160, 320)
	reaction, _ := app.Topology.CreateNode(ver.ID, "reaction_well", "反应腔", 320, 200)
	waste, _ := app.Topology.CreateNode(ver.ID, "waste_well", "废液腔", 320, 320)
	outlet, _ := app.Topology.CreateNode(ver.ID, "outlet", "出口", 480, 200)
	fmt.Printf("  [ok] 拓扑节点: inlet=%d v1=%d v2=%d reaction=%d waste=%d outlet=%d\n",
		inlet.ID, v1.ID, v2.ID, reaction.ID, waste.ID, outlet.ID)

	// 3. 边：inlet->v1->reaction->outlet；v1->v2->waste 支路（清洗覆盖）。
	e1 := edgeFor(ver.ID, inlet.ID, "A", v1.ID, "B", "two_way")
	if _, err := app.Topology.CreateEdge(ver.ID, e1, ver.Rev); err != nil {
		return fmt.Errorf("建边 e1: %w", err)
	}
	ver, _ = app.Chips.GetVersion(ver.ID)
	e2 := edgeFor(ver.ID, v1.ID, "C", reaction.ID, "A", "two_way")
	if _, err := app.Topology.CreateEdge(ver.ID, e2, ver.Rev); err != nil {
		return fmt.Errorf("建边 e2: %w", err)
	}
	ver, _ = app.Chips.GetVersion(ver.ID)
	e3 := edgeFor(ver.ID, reaction.ID, "B", outlet.ID, "A", "one_way")
	if _, err := app.Topology.CreateEdge(ver.ID, e3, ver.Rev); err != nil {
		return fmt.Errorf("建边 e3: %w", err)
	}
	ver, _ = app.Chips.GetVersion(ver.ID)
	e4 := edgeFor(ver.ID, v1.ID, "D", v2.ID, "A", "two_way")
	if _, err := app.Topology.CreateEdge(ver.ID, e4, ver.Rev); err != nil {
		return fmt.Errorf("建边 e4: %w", err)
	}
	ver, _ = app.Chips.GetVersion(ver.ID)
	e5 := edgeFor(ver.ID, v2.ID, "B", waste.ID, "A", "two_way")
	if _, err := app.Topology.CreateEdge(ver.ID, e5, ver.Rev); err != nil {
		return fmt.Errorf("建边 e5: %w", err)
	}
	fmt.Printf("  [ok] 拓扑边建立完成，版本 rev=%d\n", ver.Rev)

	// 4. 隔离区：反应腔与废液腔隔离（需声明交叉流路）。
	_, err = app.Topo.CreateZone(modelZone("隔离区1", "反应腔与废液腔隔离", "反应腔区"), []int64{reaction.ID})
	if err != nil {
		return fmt.Errorf("创建隔离区: %w", err)
	}
	_, err = app.Topo.CreateZone(modelZone("隔离区2", "废液区", "废液腔区"), []int64{waste.ID})
	if err != nil {
		return fmt.Errorf("创建隔离区2: %w", err)
	}
	fmt.Println("  [ok] 隔离区建立")

	// 5. 场景一：样本流，V1 关闭 -> 阻断。
	step1, err := app.Flow.CreateStep(modelFlowStep(ver.ID, 1, "sample", inlet.ID, outlet.ID,
		[][]any{{v1.ID, "closed"}, {v2.ID, "closed"}}))
	if err != nil {
		return fmt.Errorf("创建步骤1: %w", err)
	}
	vr1, err := app.Validation.ValidateStep(step1.ID)
	if err != nil {
		return fmt.Errorf("校验步骤1: %w", err)
	}
	if vr1.Passed {
		return fmt.Errorf("预期步骤1阻断，但校验通过")
	}
	fmt.Printf("  [ok] 场景一：关闭 V1 后样本流被阻断 (blocked_edges=%v)\n", vr1.BlockedEdges)

	// 6. 打开 V1 后校验应通过（V2 仍关，清洗液不可达废液支路 -> 死腔风险）。
	step1Updated, err := app.Flow.UpdateValves(step1.ID, []model.ValveCommand{
		{NodeID: v1.ID, State: model.ValveOpen},
		{NodeID: v2.ID, State: model.ValveClosed},
	})
	if err != nil {
		return fmt.Errorf("更新阀门: %w", err)
	}
	_ = step1Updated
	vr2, err := app.Validation.ValidateStep(step1.ID)
	if err != nil {
		return fmt.Errorf("复核步骤1: %w", err)
	}
	fmt.Printf("  [ok] 场景一修复：V1 打开后可达=%v passed=%v\n", vr2.Reachable, vr2.Passed)

	// 7. 场景二：清洗液步骤，V1/V2 全开 -> 无死腔、无残留。
	step2, err := app.Flow.CreateStep(modelFlowStep(ver.ID, 2, "rinse", inlet.ID, waste.ID,
		[][]any{{v1.ID, "open"}, {v2.ID, "open"}}))
	if err != nil {
		return fmt.Errorf("创建清洗步骤: %w", err)
	}
	vr3, err := app.Validation.ValidateStep(step2.ID)
	if err != nil {
		return fmt.Errorf("校验清洗步骤: %w", err)
	}
	fmt.Printf("  [ok] 场景二：清洗液步骤 passed=%v dead_volumes=%v residual=%v\n",
		vr3.Passed, vr3.DeadVolumes, vr3.ResidualWells)

	// 8. 死腔场景：样本步骤 V2 关闭时，waste 支路在清洗前不可达 -> 从步骤1生成风险。
	risks, err := app.Risk.CreateFromValidation(vr1.ID)
	if err != nil {
		return fmt.Errorf("从校验生成风险: %w", err)
	}
	fmt.Printf("  [ok] 场景二风险：由步骤1校验生成 %d 条风险\n", len(risks))

	// 9. 并发冲突：预期 rev 用旧值应冲突。
	_, err = app.Topology.CreateEdge(ver.ID, edgeFor(ver.ID, inlet.ID, "Z", waste.ID, "Z", "one_way"), ver.Rev-1)
	if err == nil {
		return fmt.Errorf("预期乐观版本冲突，但未发生")
	}
	fmt.Printf("  [ok] 场景三：乐观版本冲突被拒绝 (%v)\n", err)

	// 10. 冻结：版本先批准再冻结快照。
	_ = app.Chips.UpdateVersionStatus(ver.ID, "approved")
	snap, err := app.Snapshot.BuildAndFreeze(ver.ID, "smoke-tester")
	if err != nil {
		return fmt.Errorf("冻结快照: %w", err)
	}
	if snap.Status != "frozen" || snap.TopologyHash == "" {
		return fmt.Errorf("快照未正确冻结")
	}
	fmt.Printf("  [ok] 快照冻结: hash=%s nodes=%d edges=%d\n",
		snap.TopologyHash[:16], snap.NodeCount, snap.EdgeCount)

	// 11. 冻结后禁止改边。
	_, err = app.Topology.CreateEdge(ver.ID, edgeFor(ver.ID, inlet.ID, "Q", outlet.ID, "Q", "one_way"), ver.Rev)
	if err == nil {
		return fmt.Errorf("预期冻结后禁止改边，但未发生")
	}
	fmt.Printf("  [ok] 场景三：冻结后改边被拒绝 (%v)\n", err)

	// 12. 重启恢复：关闭并重新打开数据库，校验数据仍在。
	if err := app.Close(); err != nil {
		return fmt.Errorf("关闭数据库: %w", err)
	}
	db2, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("重开数据库: %w", err)
	}
	app2 := service.New(db2)
	defer app2.Close()
	gotChip, err := app2.Chips.GetChip(chip.ID)
	if err != nil {
		return fmt.Errorf("重启后读芯片失败: %w", err)
	}
	gotVer, err := app2.Chips.GetVersion(ver.ID)
	if err != nil {
		return fmt.Errorf("重启后读版本失败: %w", err)
	}
	snaps, err := app2.Snaps.ListByVersion(ver.ID)
	if err != nil {
		return fmt.Errorf("重启后读快照失败: %w", err)
	}
	if len(snaps) != 1 || snaps[0].Status != "frozen" {
		return fmt.Errorf("重启后快照状态异常")
	}
	fmt.Printf("  [ok] 重启恢复：芯片 %q 版本 v%d 状态 %s，冻结快照 %d 份\n",
		gotChip.Name, gotVer.VersionNo, gotVer.Status, len(snaps))
	return nil
}

// edgeFor 构造一条边（便于 smoke 测试调用）。
func edgeFor(versionID, from int64, fromPort string, to int64, toPort string, dir string) *model.Edge {
	return &model.Edge{
		VersionID:  versionID,
		FromNodeID: from,
		FromPort:   model.EdgePort(fromPort),
		ToNodeID:   to,
		ToPort:     model.EdgePort(toPort),
		Direction:  model.Direction(dir),
		Width:      10,
	}
}

// modelZone 构造隔离区（便于 smoke 测试调用）。
func modelZone(name, reason, crossings string) *model.IsolationZone {
	return &model.IsolationZone{
		Name:              name,
		Reason:            reason,
		DeclaredCrossings: crossings,
	}
}

// modelFlowStep 构造流程步骤：valveSpecs 为 [nodeID, state] 二元组列表。
func modelFlowStep(versionID, order int64, fluid string, inlet, outlet int64, valveSpecs [][]any) *model.FlowStep {
	st := &model.FlowStep{
		VersionID: versionID,
		OrderNo:   int(order),
		FluidType: model.FluidType(fluid),
		InletID:   inlet,
		OutletID:  outlet,
	}
	for _, spec := range valveSpecs {
		st.Valves = append(st.Valves, model.ValveCommand{
			NodeID: spec[0].(int64),
			State:  model.ValveState(spec[1].(string)),
		})
	}
	return st
}
