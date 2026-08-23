package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"task169-microfluidbench/internal/httpapi"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

func TestServerCreatesChipAndServesWorkspace(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler := httpapi.New(service.New(db)).Handler()

	create := httptest.NewRequest(http.MethodPost, "/api/chips", bytes.NewBufferString(`{"name":"HTTP chip","description":"route test"}`))
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", created.Code, created.Body.String())
	}
	var chip struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(created.Body).Decode(&chip); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if chip.ID == 0 || chip.Name != "HTTP chip" {
		t.Fatalf("created chip = %+v", chip)
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !bytes.Contains(page.Body.Bytes(), []byte("拓扑")) {
		t.Fatalf("workspace status=%d body=%q", page.Code, page.Body.String())
	}
}

// newHandlerWithVersion 建立内存库并创建一枚芯片与一个版本，返回处理器与版本 ID。
func newHandlerWithVersion(t *testing.T) (http.Handler, int64) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler := httpapi.New(service.New(db)).Handler()

	create := httptest.NewRequest(http.MethodPost, "/api/chips", bytes.NewBufferString(`{"name":"risk chip","description":"risk test"}`))
	create.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create chip status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var chip struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&chip); err != nil {
		t.Fatalf("decode chip: %v", err)
	}

	vrec := httptest.NewRecorder()
	handler.ServeHTTP(vrec, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/chips/%d/versions", chip.ID), bytes.NewBufferString(`{"note":"v1"}`)))
	if vrec.Code != http.StatusCreated {
		t.Fatalf("create version status = %d, body=%s", vrec.Code, vrec.Body.String())
	}
	var ver struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(vrec.Body).Decode(&ver); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	return handler, ver.ID
}

func TestCreateRiskRejectsUnknownKind(t *testing.T) {
	handler, versionID := newHandlerWithVersion(t)

	body := fmt.Sprintf(`{"kind":"unknown","severity":"high","title":"t","evidence":"e"}`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/versions/%d/risks", versionID), bytes.NewBufferString(body)))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown kind status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}

	// 未定义类型不应写入存储：版本下不应存在任何风险。
	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/versions/%d/risks", versionID), nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list risks status = %d, body=%s", list.Code, list.Body.String())
	}
	if bytes.Contains(list.Body.Bytes(), []byte("unknown")) {
		t.Fatalf("未知风险类型不应被持久化: %s", list.Body.String())
	}
}

func TestCreateRiskAcceptsValidKind(t *testing.T) {
	handler, versionID := newHandlerWithVersion(t)

	body := fmt.Sprintf(`{"kind":"residual","severity":"medium","title":"t","evidence":"e"}`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/versions/%d/risks", versionID), bytes.NewBufferString(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("valid kind status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}
}
