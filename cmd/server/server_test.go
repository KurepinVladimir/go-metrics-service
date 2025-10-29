package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/audit"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/handler"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// --- Мок-приёмник аудита ---

type memSink struct {
	ch chan audit.Event
}

func newTestAuditor() (*audit.Auditor, <-chan audit.Event) {
	ch := make(chan audit.Event, 8)
	return audit.New(&memSink{ch: ch}), ch
}

func (m *memSink) Send(_ context.Context, ev audit.Event) error {
	select {
	case m.ch <- ev:
	default:
	}
	return nil
}

// ===================== СТАРЫЕ ТЕСТЫ  =====================

func TestUpdateHandler_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		url        string
		wantStatus int
		check      func(t *testing.T, storage *repository.MemStorage)
	}{
		{
			name:       "Valid Gauge",
			method:     http.MethodPost,
			url:        "/update/gauge/testGauge/42.5",
			wantStatus: http.StatusOK,
			check: func(t *testing.T, storage *repository.MemStorage) {
				val, ok := storage.GetGauge(context.Background(), "testGauge")
				assert.True(t, ok)
				assert.Equal(t, 42.5, val)
			},
		},
		{
			name:       "Valid Counter",
			method:     http.MethodPost,
			url:        "/update/counter/testCounter/5",
			wantStatus: http.StatusOK,
			check: func(t *testing.T, storage *repository.MemStorage) {
				val, ok := storage.GetCounter(context.Background(), "testCounter")
				assert.True(t, ok)
				assert.Equal(t, int64(5), val)
			},
		},
		{
			name:       "Invalid Metric Type",
			method:     http.MethodPost,
			url:        "/update/unknown/test/123",
			wantStatus: http.StatusBadRequest,
			check:      func(t *testing.T, _ *repository.MemStorage) {},
		},
		{
			name:       "Missing Metric Name",
			method:     http.MethodPost,
			url:        "/update/gauge//123",
			wantStatus: http.StatusNotFound,
			check:      func(t *testing.T, _ *repository.MemStorage) {},
		},
		{
			name:       "Invalid Gauge Value",
			method:     http.MethodPost,
			url:        "/update/gauge/test/invalid",
			wantStatus: http.StatusBadRequest,
			check:      func(t *testing.T, _ *repository.MemStorage) {},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			storage := repository.NewMemStorage()
			aud, _ := newTestAuditor()
			r := chi.NewRouter()
			r.Post("/update/{type}/{name}/{value}", updateHandler(storage, aud))

			req := httptest.NewRequest(tc.method, tc.url, nil)
			req.RemoteAddr = "192.0.2.10:12345"
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)
			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			tc.check(t, storage)
		})
	}
}

func TestGetValueHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateGauge(context.Background(), "myGauge", 42.5)
	storage.UpdateCounter(context.Background(), "myCounter", 5)

	r := chi.NewRouter()
	r.Get("/value/{type}/{name}", valueHandler(storage))

	tests := []struct {
		name       string
		url        string
		expected   string
		statusCode int
	}{
		{"Existing Gauge", "/value/gauge/myGauge", "42.5", http.StatusOK},
		{"Existing Counter", "/value/counter/myCounter", "5", http.StatusOK},
		{"Unknown Gauge", "/value/gauge/unknown", "", http.StatusNotFound},
		{"Unknown Counter", "/value/counter/unknown", "", http.StatusNotFound},
		{"Invalid Type", "/value/unknown/type", "", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()
			body, _ := io.ReadAll(res.Body)

			assert.Equal(t, tt.statusCode, res.StatusCode)
			if res.StatusCode == http.StatusOK {
				assert.Equal(t, tt.expected, string(body))
			}
		})
	}
}

func TestHTMLHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateGauge(context.Background(), "myGauge", 1.23)
	storage.UpdateCounter(context.Background(), "myCounter", 99)

	r := chi.NewRouter()
	r.Get("/", indexHandler(storage))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "myGauge")
	assert.Contains(t, string(body), "1.230")
	assert.Contains(t, string(body), "myCounter")
	assert.Contains(t, string(body), "99")
}

func TestUpdateHandlerJSON(t *testing.T) {
	storage := repository.NewMemStorage()
	aud, _ := newTestAuditor()
	h := updateHandlerJSON(storage, aud)

	tests := []struct {
		name       string
		input      string
		wantStatus int
		check      func() error
	}{
		{
			name:       "valid gauge metric",
			input:      `{"id":"TestGauge","type":"gauge","value":123.456}`,
			wantStatus: http.StatusOK,
			check: func() error {
				v, ok := storage.GetGauge(context.Background(), "TestGauge")
				if !ok || v != 123.456 {
					return fmt.Errorf("expected 123.456, got %v (ok=%v)", v, ok)
				}
				return nil
			},
		},
		{
			name:       "valid counter metric",
			input:      `{"id":"TestCounter","type":"counter","delta":5}`,
			wantStatus: http.StatusOK,
			check: func() error {
				v, ok := storage.GetCounter(context.Background(), "TestCounter")
				if !ok || v != 5 {
					return fmt.Errorf("expected 5, got %v (ok=%v)", v, ok)
				}
				return nil
			},
		},
		{
			name:       "missing value",
			input:      `{"id":"NoValue","type":"gauge"}`,
			wantStatus: http.StatusBadRequest,
			check:      func() error { return nil },
		},
		{
			name:       "unknown type",
			input:      `{"id":"BadType","type":"other","value":1.23}`,
			wantStatus: http.StatusNotImplemented,
			check:      func() error { return nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader(tt.input))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "192.0.2.20:5555"
			rr := httptest.NewRecorder()

			h(rr, req)
			res := rr.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d", res.StatusCode, tt.wantStatus)
			}
			if err := tt.check(); err != nil {
				t.Errorf("check failed: %v", err)
			}
		})
	}
}

func TestValueHandlerJSON(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateGauge(context.Background(), "G1", 99.9)
	storage.UpdateCounter(context.Background(), "C1", 7)

	h := valueHandlerJSON(storage)

	tests := []struct {
		name       string
		input      string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "existing gauge",
			input:      `{"id":"G1","type":"gauge"}`,
			wantStatus: http.StatusOK,
			wantBody:   `"value":99.9`,
		},
		{
			name:       "existing counter",
			input:      `{"id":"C1","type":"counter"}`,
			wantStatus: http.StatusOK,
			wantBody:   `"delta":7`,
		},
		{
			name:       "not found",
			input:      `{"id":"none","type":"gauge"}`,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/value", strings.NewReader(tt.input))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			h(rr, req)
			res := rr.Result()
			defer res.Body.Close()

			body, _ := io.ReadAll(res.Body)

			if res.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d", res.StatusCode, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(string(body), tt.wantBody) {
				t.Errorf("expected body to contain %q, got %s", tt.wantBody, body)
			}
		})
	}
}

// Агент может отправить gzip-запрос
func TestUpdateHandlerJSON_GzipRequest(t *testing.T) {
	storage := repository.NewMemStorage()
	aud, _ := newTestAuditor()
	h := updateHandlerJSON(storage, aud)

	// JSON-метрика
	input := `{"id":"GZGauge","type":"gauge","value":3.14}`

	var buf strings.Builder
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write([]byte(input))
	assert.NoError(t, err)
	assert.NoError(t, gz.Close())

	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader(buf.String()))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.30:6000"

	rr := httptest.NewRecorder()

	// Оборачиваем хендлер миддлварой
	wrapped := gzipRequestMiddleware(h)
	wrapped.ServeHTTP(rr, req)

	res := rr.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)

	v, ok := storage.GetGauge(context.Background(), "GZGauge")
	assert.True(t, ok)
	assert.Equal(t, 3.14, v)
}

// Сервер сжимает ответ, если клиент просит gzip
func TestValueHandlerJSON_GzipResponse(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateGauge(context.Background(), "GZGauge", 2.718)

	h := valueHandlerJSON(storage)

	// JSON-запрос
	body := `{"id":"GZGauge","type":"gauge"}`
	req := httptest.NewRequest(http.MethodPost, "/value", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()

	// Оборачиваем хендлер миддлварой
	wrapped := gzipResponseMiddleware(h)
	wrapped.ServeHTTP(rr, req)

	res := rr.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "gzip", res.Header.Get("Content-Encoding"))

	// Распаковываем ответ
	gr, err := gzip.NewReader(res.Body)
	assert.NoError(t, err)
	defer gr.Close()

	uncompressed, err := io.ReadAll(gr)
	assert.NoError(t, err)

	assert.Contains(t, string(uncompressed), `"value":2.718`)
}

// ===================== НОВЫЕ ТЕСТЫ АУДИТА =====================

func TestAudit_OnUpdatePath(t *testing.T) {
	storage := repository.NewMemStorage()
	aud, ch := newTestAuditor()

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", updateHandler(storage, aud))

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/M1/1.5", nil)
	req.RemoteAddr = "198.51.100.1:1234"
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	res := rr.Result()
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	select {
	case ev := <-ch:
		assert.ElementsMatch(t, []string{"M1"}, ev.Metrics)
		assert.Equal(t, "198.51.100.1", ev.IPAddress)
		assert.InDelta(t, time.Now().Unix(), ev.TS, 5) // 5 секунд допуск
	default:
		t.Fatalf("expected an audit event")
	}
}

func TestAudit_OnUpdateJSON(t *testing.T) {
	storage := repository.NewMemStorage()
	aud, ch := newTestAuditor()

	h := updateHandlerJSON(storage, aud)
	req := httptest.NewRequest(http.MethodPost, "/update",
		strings.NewReader(`{"id":"A1","type":"counter","delta":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.9:7777"
	rr := httptest.NewRecorder()

	h(rr, req)
	res := rr.Result()
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	select {
	case ev := <-ch:
		assert.ElementsMatch(t, []string{"A1"}, ev.Metrics)
		assert.Equal(t, "203.0.113.9", ev.IPAddress)
	default:
		t.Fatalf("expected an audit event")
	}
}

func TestAudit_OnUpdatesBatch(t *testing.T) {
	storage := repository.NewMemStorage()
	aud, ch := newTestAuditor()

	r := chi.NewRouter()
	r.Post("/updates", handler.UpdatesHandler(storage, "", aud))

	batch := []models.Metrics{
		{ID: "G1", MType: "gauge", Value: ptrF(1.23)},
		{ID: "C1", MType: "counter", Delta: ptrI(5)},
	}
	b, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/updates", strings.NewReader(string(b)))
	// Если в UpdatesHandler проверка строгая, этот заголовок точно пройдёт
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.55:9090"

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	res := rr.Result()
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	select {
	case ev := <-ch:
		assert.ElementsMatch(t, []string{"G1", "C1"}, ev.Metrics)
		assert.Equal(t, "192.0.2.55", ev.IPAddress)
		// ts ~ сейчас
		assert.InDelta(t, time.Now().Unix(), ev.TS, 5)
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected an audit event")
	}
}

func ptrF(v float64) *float64 { return &v }
func ptrI(v int64) *int64     { return &v }
