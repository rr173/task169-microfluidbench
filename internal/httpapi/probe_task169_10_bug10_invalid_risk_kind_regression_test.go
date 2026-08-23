package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task169-microfluidbench/internal/httpapi"
	"task169-microfluidbench/internal/service"
	"task169-microfluidbench/internal/store"
)

func TestInvalidRiskKindIsRejected(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	app := service.New(db)
	t.Cleanup(func() { _ = app.Close() })
	chip, err := app.Chips.CreateChip("risk-validation", "")
	if err != nil {
		t.Fatal(err)
	}
	version, err := app.Chips.CreateVersion(chip.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	body := `{"kind":"unknown","severity":"high","title":"bad","evidence":"probe"}`
	req := httptest.NewRequest(http.MethodPost, "/api/versions/"+itoa(version.ID)+"/risks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	httpapi.New(app).Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	buf := [32]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
