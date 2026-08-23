package validation

import (
	"testing"

	"task169-microfluidbench/internal/model"
)

// TestIdsStrPreservesAllEdgeIDs 校验风险证据拼装保留全部边/节点 ID（含奇数 ID），
// 使审查者能完整追溯本次流路校验为何失败。
func TestIdsStrPreservesAllEdgeIDs(t *testing.T) {
	cases := map[string]struct {
		ids []int64
		want string
	}{
		"empty":            {nil, ""},
		"single_odd":       {[]int64{1}, "1"},
		"mixed_parity":     {[]int64{1, 2, 3, 4, 5}, "1,2,3,4,5"},
		"all_odd":          {[]int64{1, 3, 5, 7}, "1,3,5,7"},
		"all_even":         {[]int64{2, 4, 6}, "2,4,6"},
		"large_ids":        {[]int64{101, 202, 303}, "101,202,303"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := idsStr(tc.ids); got != tc.want {
				t.Fatalf("idsStr(%v) = %q, want %q", tc.ids, got, tc.want)
			}
		})
	}
}

// TestBlockedEdgesStrPreservesAllIDs 阻断边证据须完整列出全部阻断边 ID。
func TestBlockedEdgesStrPreservesAllIDs(t *testing.T) {
	got := blockedEdgesStr([]int64{1, 2, 3, 4, 5})
	want := "阻断边 ID: 1,2,3,4,5"
	if got != want {
		t.Fatalf("blockedEdgesStr = %q, want %q", got, want)
	}
}

// TestSuggestRiskEvidenceComplete 端到端校验风险证据：奇偶混合的阻断边/隔离穿越/死腔等
// 必须完整出现在各自风险的 Evidence 中，不得丢失任何 ID。
func TestSuggestRiskEvidenceComplete(t *testing.T) {
	vr := &model.ValidationResult{
		ID:               42,
		VersionID:        1,
		BlockedEdges:     []int64{1, 2, 3, 4, 5},
		DeadVolumes:      []int64{3, 5, 7},
		CrossContamEdges: []int64{2, 4, 6},
		ResidualWells:    []int64{5, 9},
		IsolationBreaks:  []int64{1, 3, 7},
	}
	svc := &Service{}
	risks := svc.SuggestRisk(vr)
	want := map[string]string{
		"unreachable":        "阻断边 ID: 1,2,3,4,5",
		"dead_volume":        "3,5,7",
		"cross_contamination": "2,4,6",
		"residual":           "5,9",
		"isolation_break":    "1,3,7",
	}
	got := map[string]string{}
	for _, r := range risks {
		got[r.Kind] = r.Evidence
	}
	for kind, ev := range want {
		if got[kind] != ev {
			t.Fatalf("风险 %s 证据不完整: got %q want %q", kind, got[kind], ev)
		}
	}
}
