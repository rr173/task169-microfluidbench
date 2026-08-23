package httpapi_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"task169-microfluidbench/internal/httpapi"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

func TestCreateStepRejectsUnknownFluidType(t *testing.T) {
	db, err := store.Open(":memory:"); if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = db.Close() })
	app := service.New(db); h := httpapi.New(app).Handler()
	chip, _ := app.Chips.CreateChip("fluid", ""); version, _ := app.Chips.CreateVersion(chip.ID, "")
	inlet, _ := app.Topology.CreateNode(version.ID, "inlet", "in", 0, 0)
	outlet, _ := app.Topology.CreateNode(version.ID, "outlet", "out", 1, 0)
	body := `{"order_no":1,"fluid_type":"unknown","inlet_id":` + itoa(inlet.ID) + `,"outlet_id":` + itoa(outlet.ID) + `,"valves":[]}`
	r := httptest.NewRequest(http.MethodPost, "/api/versions/"+itoa(version.ID)+"/steps", bytes.NewBufferString(body)); r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder(); h.ServeHTTP(w, r)
	if w.Code != http.StatusUnprocessableEntity { t.Fatalf("status=%d body=%s", w.Code, w.Body.String()) }
}

func itoa(v int64) string { if v == 0 { return "0" }; var b [20]byte; i:=len(b); for v>0 { i--; b[i]=byte('0'+v%10); v/=10 }; return string(b[i:]) }
