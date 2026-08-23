package model

import "testing"

func TestVersionTransitions(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{VersionEditing, VersionPending, true},
		{VersionPending, VersionApproved, true},
		{VersionPending, VersionRiskFound, true},
		{VersionRiskFound, VersionEditing, true},
		{VersionApproved, VersionSuperseded, true},
		{VersionApproved, VersionEditing, false},
		{VersionEditing, VersionApproved, false},
		{VersionSuperseded, VersionEditing, false},
	}
	for _, c := range cases {
		if got := CanTransitVersion(c.from, c.to); got != c.want {
			t.Errorf("CanTransitVersion(%s->%s) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestStepTransitions(t *testing.T) {
	if !CanTransitStep(StepDraft, StepRunnable) {
		t.Error("draft -> runnable 应合法")
	}
	if !CanTransitStep(StepRunnable, StepVerified) {
		t.Error("runnable -> verified 应合法")
	}
	if CanTransitStep(StepVerified, StepRunnable) {
		t.Error("verified -> runnable 应非法")
	}
}

func TestRiskTransitions(t *testing.T) {
	if !CanTransitRisk(RiskNew, RiskConfirmed) {
		t.Error("new -> confirmed 应合法")
	}
	if !CanTransitRisk(RiskConfirmed, RiskWaived) {
		t.Error("confirmed -> waived 应合法")
	}
	if CanTransitRisk(RiskWaived, RiskNew) {
		t.Error("waived -> new 应非法")
	}
}

func TestValidEnums(t *testing.T) {
	if !ValidNodeKind(NodeValve) || ValidNodeKind(NodeKind("ghost")) {
		t.Error("节点类型校验错误")
	}
	if !ValidFluidType(FluidRinse) || ValidFluidType(FluidType("oil")) {
		t.Error("流体类型校验错误")
	}
	if !ValidDirection(DirectionTwoWay) || ValidDirection(Direction("diag")) {
		t.Error("方向校验错误")
	}
	if !IsFluidContaminant(FluidSample) || IsFluidContaminant(FluidRinse) {
		t.Error("污染流体判定错误")
	}
}

func TestValidateInputs(t *testing.T) {
	if err := ValidateChipInput("", "x"); err == nil {
		t.Error("空名称应报错")
	}
	if err := ValidateChipInput("ok", "desc"); err != nil {
		t.Errorf("合法输入应通过: %v", err)
	}
	if err := ValidateEdgeInput(1, 1, "A", "B", DirectionOneWay, 10); err == nil {
		t.Error("自环应报错")
	}
	if err := ValidateEdgeInput(1, 2, "A", "B", DirectionOneWay, -1); err == nil {
		t.Error("负宽度应报错")
	}
	if err := ValidateStepInput(FluidReagent, 1, 10, 20); err != nil {
		t.Errorf("合法步骤应通过: %v", err)
	}
	if err := ValidateStepInput(FluidType("oil"), 1, 10, 20); err == nil {
		t.Error("非法流体类型应报错")
	}
	if err := ValidateStepInput("", 1, 10, 20); err == nil {
		t.Error("空流体类型应报错")
	}
	if err := ValidateSnapshotTransition(SnapshotBuilding, SnapshotFrozen); err != nil {
		t.Errorf("合法快照迁移应通过: %v", err)
	}
	if err := ValidateSnapshotTransition(SnapshotFrozen, SnapshotBuilding); err == nil {
		t.Error("非法快照迁移应报错")
	}
}

func TestTimeRoundTrip(t *testing.T) {
	ti := NowUTC()
	raw, err := ti.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	var got Time
	if err := got.Scan(raw); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got.IsZero() {
		t.Fatal("扫描后不应为零值")
	}
	b, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var back Time
	if err := back.UnmarshalJSON(b); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
}
