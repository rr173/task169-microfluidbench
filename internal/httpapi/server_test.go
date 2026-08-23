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
