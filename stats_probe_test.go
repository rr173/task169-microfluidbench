package microfluidbench_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"task169-microfluidbench/internal/httpapi"
	"task169-microfluidbench/internal/model"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

func TestStatsStableAfterPersistedRisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mfb_stats_probe.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	app := service.New(db)
	srv := httpapi.New(app)

	chip, err := app.Chips.CreateChip("probe", "x")
	if err != nil {
		t.Fatalf("create chip: %v", err)
	}
	ver, err := app.Chips.CreateVersion(chip.ID, "v1")
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	r, err := app.Risk.Create(&model.Risk{
		VersionID: ver.ID, Kind: "dead_volume", Severity: "low",
		Title: "dead volume", Evidence: "edge 2", Status: model.RiskNew,
	})
	if err != nil {
		t.Fatalf("create risk: %v", err)
	}
	t.Logf("persisted risk id=%d version=%d", r.ID, ver.ID)

	// 1) Stats must return total_risks as int == 1, without crashing.
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"total_risks":1`) {
		t.Fatalf("stats body missing total_risks:1: %s", body)
	}
	if strings.Contains(body, `risk_total`) {
		t.Fatalf("stats body still exposes stale risk_total key: %s", body)
	}

	// 2) Graph endpoint on the version with a persisted risk must not crash
	//    (previously ListByVersion returned nil entries -> rk.Evidence panic).
	greq := httptest.NewRequest(http.MethodGet, "/api/versions/1/graph", nil)
	grec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(grec, greq)
	if grec.Code != http.StatusOK {
		t.Fatalf("graph status=%d body=%s", grec.Code, grec.Body.String())
	}
	if !strings.Contains(grec.Body.String(), `"risk_count":1`) {
		t.Fatalf("graph body missing risk_count:1: %s", grec.Body.String())
	}
}
