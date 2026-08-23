package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task169-microfluidbench/internal/httpapi"
	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

func TestStatsIncludesPersistedRiskCount(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	app := service.New(db)
	t.Cleanup(func() { _ = app.Close() })
	chip, err := app.Chips.CreateChip("stats", "")
	if err != nil {
		t.Fatal(err)
	}
	version, err := app.Chips.CreateVersion(chip.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Risk.Create(&model.Risk{
		VersionID: version.ID,
		Kind:      "dead_volume",
		Severity:  "high",
		Title:     "persisted risk",
		Evidence:  "probe",
	}); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	httpapi.New(app).Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if got, ok := body["total_risks"].(float64); !ok || int(got) != 1 {
		t.Fatalf("total_risks=%v body=%s", body["total_risks"], rr.Body.String())
	}
}
