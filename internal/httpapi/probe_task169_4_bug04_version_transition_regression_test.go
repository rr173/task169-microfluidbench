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

func TestVersionTransitionPersistsRequestedState(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := service.New(db)
	h := httpapi.New(app).Handler()
	chip, _ := app.Chips.CreateChip("transition", "")
	version, _ := app.Chips.CreateVersion(chip.ID, "")
	req := httptest.NewRequest(http.MethodPost, "/api/versions/"+itoa(version.ID)+"/transition", bytes.NewBufferString(`{"to":"pending"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("transition status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode transition: %v", err)
	}
	if got.Status != "pending" {
		t.Fatalf("response status=%q", got.Status)
	}
	persisted, err := app.Chips.GetVersion(version.ID)
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	if persisted.Status != "pending" {
		t.Fatalf("persisted status=%q", persisted.Status)
	}
}

func itoa(v int64) string {
	var b [20]byte
	i := len(b)
	if v == 0 {
		return "0"
	}
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
