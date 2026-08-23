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

func TestRiskDispositionPersistsOwnerAndResolution(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := service.New(db)
	h := httpapi.New(app).Handler()
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	chip, _ := app.Chips.CreateChip("risk-disposition", "")
	version, _ := app.Chips.CreateVersion(chip.ID, "")
	w := request(http.MethodPost, "/api/versions/"+itoa(version.ID)+"/risks", `{"kind":"unreachable","severity":"high","title":"blocked","evidence":"edge 1"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create risk status=%d body=%s", w.Code, w.Body.String())
	}
	var created struct{ ID int64 `json:"id"` }
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode risk: %v", err)
	}
	w = request(http.MethodPut, "/api/risks/"+itoa(created.ID)+"/transit", `{"to":"confirmed","owner":"alice","resolution":"rerouted channel"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("transit status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Owner string `json:"owner"`
		Resolution string `json:"resolution"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode transit: %v", err)
	}
	if got.Owner != "alice" || got.Resolution != "rerouted channel" {
		t.Fatalf("disposition = %+v, want owner and resolution preserved", got)
	}
}

func itoa(v int64) string {
	if v == 0 { return "0" }
	var b [20]byte
	i := len(b)
	for v > 0 { i--; b[i] = byte('0' + v%10); v /= 10 }
	return string(b[i:])
}
