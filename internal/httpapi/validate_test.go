package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task169-microfluidbench/internal/httpapi"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

// TestValidateStepReturnsBlockedEdges 校验 POST /api/steps/{id}/validate 在路径被阻断时
// 完整返回 blocked_edges 证据，使审查者能追溯本次流路校验为何失败（契约：返回 blocked_edges）。
func TestValidateStepReturnsBlockedEdges(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	srv := httpapi.New(service.New(db)).Handler()
	postJSON := func(path string, body string, into any) {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code >= 400 {
			t.Fatalf("POST %s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
		if into != nil {
			if err := json.NewDecoder(rec.Body).Decode(into); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
		}
	}

	var chip struct {
		ID int64 `json:"id"`
	}
	postJSON("/api/chips", `{"name":"blk chip","description":""}`, &chip)
	var ver struct {
		ID int64 `json:"id"`
	}
	postJSON("/api/chips/"+itoa(chip.ID)+"/versions", `{"note":"v1"}`, &ver)

	// 节点：inlet / valve / outlet。
	var inlet, valve, outlet struct {
		ID int64 `json:"id"`
	}
	postJSON("/api/versions/"+itoa(ver.ID)+"/nodes", `{"kind":"inlet","name":"in","x":0,"y":0}`, &inlet)
	postJSON("/api/versions/"+itoa(ver.ID)+"/nodes", `{"kind":"valve","name":"v","x":1,"y":0}`, &valve)
	postJSON("/api/versions/"+itoa(ver.ID)+"/nodes", `{"kind":"outlet","name":"out","x":2,"y":0}`, &outlet)

	// 边：inlet-valve-valve-outlet（两条双向边）。
	postJSON("/api/versions/"+itoa(ver.ID)+"/edges",
		`{"from_node_id":`+itoa(inlet.ID)+`,"from_port":"A","to_node_id":`+itoa(valve.ID)+`,"to_port":"B","direction":"two_way","width":10,"expected_rev":0}`,
		nil)
	postJSON("/api/versions/"+itoa(ver.ID)+"/edges",
		`{"from_node_id":`+itoa(valve.ID)+`,"from_port":"C","to_node_id":`+itoa(outlet.ID)+`,"to_port":"A","direction":"two_way","width":10,"expected_rev":1}`,
		nil)

	// 流程步骤：关闭主阀 -> 样本流被阻断。
	var step struct {
		ID int64 `json:"id"`
	}
	postJSON("/api/versions/"+itoa(ver.ID)+"/steps",
		`{"order_no":1,"fluid_type":"sample","inlet_id":`+itoa(inlet.ID)+`,"outlet_id":`+itoa(outlet.ID)+
			`,"valves":[{"node_id":`+itoa(valve.ID)+`,"state":"closed"}]}`, &step)

	var vr struct {
		Passed       bool    `json:"passed"`
		Reachable    bool    `json:"reachable"`
		BlockedEdges []int64 `json:"blocked_edges"`
		Message      string  `json:"message"`
	}
	postJSON("/api/steps/"+itoa(step.ID)+"/validate", `{}`, &vr)

	if vr.Passed {
		t.Fatalf("关闭主阀后应不通过校验: %+v", vr)
	}
	if vr.Reachable {
		t.Fatalf("关闭主阀后应不可达")
	}
	if len(vr.BlockedEdges) == 0 {
		t.Fatalf("blocked_edges 必须非空，供审查者追溯失败原因: %+v", vr)
	}
	// 两条边均连接到关闭的阀门节点，均应被记为阻断边。
	if len(vr.BlockedEdges) != 2 {
		t.Fatalf("应记录全部阻断边（2 条），got %v", vr.BlockedEdges)
	}
}

// TestValidateAllAndListValidationsPreserveBlockedEdges 校验 GET /api/validations 与
// POST /api/versions/{id}/validate-all 链路上阻断边证据同样完整保留（含落库回读）。
func TestValidateAllAndListValidationsPreserveBlockedEdges(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	srv := httpapi.New(service.New(db)).Handler()
	postJSON := func(path string, body string, into any) {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code >= 400 {
			t.Fatalf("POST %s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
		if into != nil {
			if err := json.NewDecoder(rec.Body).Decode(into); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
		}
	}

	var chip struct {
		ID int64 `json:"id"`
	}
	postJSON("/api/chips", `{"name":"blk2","description":""}`, &chip)
	var ver struct {
		ID int64 `json:"id"`
	}
	postJSON("/api/chips/"+itoa(chip.ID)+"/versions", `{"note":"v1"}`, &ver)

	var inlet, valve, outlet struct {
		ID int64 `json:"id"`
	}
	postJSON("/api/versions/"+itoa(ver.ID)+"/nodes", `{"kind":"inlet","name":"in","x":0,"y":0}`, &inlet)
	postJSON("/api/versions/"+itoa(ver.ID)+"/nodes", `{"kind":"valve","name":"v","x":1,"y":0}`, &valve)
	postJSON("/api/versions/"+itoa(ver.ID)+"/nodes", `{"kind":"outlet","name":"out","x":2,"y":0}`, &outlet)
	postJSON("/api/versions/"+itoa(ver.ID)+"/edges",
		`{"from_node_id":`+itoa(inlet.ID)+`,"from_port":"A","to_node_id":`+itoa(valve.ID)+`,"to_port":"B","direction":"two_way","width":10,"expected_rev":0}`, nil)
	postJSON("/api/versions/"+itoa(ver.ID)+"/edges",
		`{"from_node_id":`+itoa(valve.ID)+`,"from_port":"C","to_node_id":`+itoa(outlet.ID)+`,"to_port":"A","direction":"two_way","width":10,"expected_rev":1}`, nil)
	postJSON("/api/versions/"+itoa(ver.ID)+"/steps",
		`{"order_no":1,"fluid_type":"sample","inlet_id":`+itoa(inlet.ID)+`,"outlet_id":`+itoa(outlet.ID)+
			`,"valves":[{"node_id":`+itoa(valve.ID)+`,"state":"closed"}]}`, nil)

	// validate-all。
	var all struct {
		AllPassed bool `json:"all_passed"`
	}
	postJSON("/api/versions/"+itoa(ver.ID)+"/validate-all", `{}`, &all)
	if all.AllPassed {
		t.Fatalf("阻断场景 all_passed 应为 false")
	}

	// GET /api/versions/{id}/validations 落库回读，阻断边证据须完整。
	req := httptest.NewRequest(http.MethodGet, "/api/versions/"+itoa(ver.ID)+"/validations", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list validations status=%d body=%s", rec.Code, rec.Body.String())
	}
	var list []struct {
		BlockedEdges []int64 `json:"blocked_edges"`
		Passed       bool    `json:"passed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode validations: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("应有 1 条校验记录, got %d", len(list))
	}
	if len(list[0].BlockedEdges) != 2 {
		t.Fatalf("落库回读阻断边证据不完整: got %v want 2 条", list[0].BlockedEdges)
	}
	if list[0].Passed {
		t.Fatalf("落库记录应为未通过")
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
