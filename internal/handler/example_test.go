// file: internal/handler/example_test.go
package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/handler"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/repository"
)

// ExampleUpdatesHandler_success показывает успешное пакетное обновление метрик.
func ExampleUpdatesHandler_success() {
	st := repository.NewMemStorage()
	// аудит в примере не используем -> nil
	h := handler.UpdatesHandler(st /* key */, "" /* aud */, nil)

	batch := []models.Metrics{
		{ID: "g1", MType: models.Gauge, Value: ptrF(1.23)},
		{ID: "c1", MType: models.Counter, Delta: ptrI(5)},
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/updates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Ответ формата {"status":"ok"}; проверим код и поле status.
	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	fmt.Println(rr.Code, resp["status"])
	// Output:
	// 200 ok
}

// ExampleUpdatesHandler_badContentType демонстрирует 415 при неверном Content-Type.
func ExampleUpdatesHandler_badContentType() {
	st := repository.NewMemStorage()
	h := handler.UpdatesHandler(st, "", nil)

	batch := []models.Metrics{
		{ID: "g1", MType: models.Gauge, Value: ptrF(1.0)},
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/updates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain") // должно быть application/json
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	fmt.Println(rr.Code)
	// Output:
	// 415
}

// ExampleUpdatesHandler_emptyBatch демонстрирует 400 при пустом массиве.
func ExampleUpdatesHandler_emptyBatch() {
	st := repository.NewMemStorage()
	h := handler.UpdatesHandler(st, "", nil)

	body := []byte(`[]`)
	req := httptest.NewRequest(http.MethodPost, "/updates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	fmt.Println(rr.Code)
	// Output:
	// 400
}

// ExampleUpdatesHandler_missingValue демонстрирует 400, если для gauge нет Value.
func ExampleUpdatesHandler_missingValue() {
	st := repository.NewMemStorage()
	h := handler.UpdatesHandler(st, "", nil)

	// gauge без Value -> 400
	batch := []models.Metrics{
		{ID: "g2", MType: models.Gauge /* Value: nil */},
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/updates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	fmt.Println(rr.Code)
	// Output:
	// 400
}

// --- helpers ---

func ptrF(v float64) *float64 { return &v }
func ptrI(v int64) *int64     { return &v }
