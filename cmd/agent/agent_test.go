package main

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
)

func TestSendMetricJSON(t *testing.T) {

	// на всякий случай сбрасываем глобальные настройки, чтобы другие тесты не влияли
	rsaPublicKey = nil
	flagKey = ""

	// Ожидаемая метрика, которую будем отправлять
	expectedMetric := models.Metrics{
		ID:    "TestMetric",
		MType: "gauge", // тип метрики — gauge
	}
	value := float64(42.0)
	expectedMetric.Value = &value // присваиваем значение

	// Создаём тестовый HTTP-сервер, который примет запрос от клиента
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверка метода и пути запроса
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/update", r.URL.Path)

		// Проверка заголовков запроса
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding")) // обязательно: ожидаем gzip
		assert.Equal(t, "203.0.113.10", r.Header.Get("X-Real-IP"))

		// Создаём gzip-ридер для чтения сжатого тела
		gr, err := gzip.NewReader(r.Body)
		assert.NoError(t, err)
		defer gr.Close()

		// Читаем распакованное тело запроса
		body, err := io.ReadAll(gr)
		assert.NoError(t, err)

		// Десериализуем JSON в структуру models.Metrics
		var received models.Metrics
		err = json.Unmarshal(body, &received)
		assert.NoError(t, err)

		// Проверяем, что метрика пришла корректно
		assert.Equal(t, expectedMetric.ID, received.ID)
		assert.Equal(t, expectedMetric.MType, received.MType)
		assert.NotNil(t, received.Value)
		assert.Equal(t, *expectedMetric.Value, *received.Value)

		// Подготовим JSON-ответ (можно пустой или с каким-то сообщением)
		response := map[string]string{"status": "ok"}
		respBody, err := json.Marshal(response)
		assert.NoError(t, err)

		// Проверим, поддерживает ли клиент gzip
		if r.Header.Get("Accept-Encoding") == "gzip" {
			// Установим заголовки ответа
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Type", "application/json")

			// Оборачиваем writer в gzip
			gz := gzip.NewWriter(w)
			defer gz.Close()

			_, err := gz.Write(respBody)
			assert.NoError(t, err)
		} else {
			// Отдаём обычный (несжатый) ответ
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(respBody)
			assert.NoError(t, err)
		}

	}))
	defer ts.Close() // Отключаем сервер после завершения теста

	// Создаём resty-клиент
	client := resty.New()

	// Вызываем тестируемую функцию: она должна отправить метрику в gzip
	agent := &Agent{
		Client:    client,
		ServerURL: ts.URL,
		RealIP:    "203.0.113.10",
	}
	err := agent.sendMetricJSON(expectedMetric)

	// Проверяем, что ошибок не было
	assert.NoError(t, err)
}

func TestCollectRuntimeMetrics(t *testing.T) {
	agent := NewAgent("http://localhost")
	agent.collectMetrics()

	expectedKeys := []string{"Alloc", "TotalAlloc", "NextGC", "NumGC"}
	for _, key := range expectedKeys {
		_, ok := agent.Metrics[key]
		assert.True(t, ok, "Expected metric %s not found", key)
	}

	assert.GreaterOrEqual(t, agent.RandomValue, 0.0)
	assert.LessOrEqual(t, agent.RandomValue, 1.0)
	assert.GreaterOrEqual(t, agent.Metrics["NumGC"], 0.0)
}
