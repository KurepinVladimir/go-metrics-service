package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/cryptohelpers"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/logger"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/retry"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

var httpDelays = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

// Agent инкапсулирует состояние и поведение агента для сбора и отправки метрик на сервер
type Agent struct {
	PollCount   int64              // счётчик обновлений метрик
	RandomValue float64            // случайное значение метрики
	Metrics     map[string]float64 // метрики типа gauge из runtime
	Client      *resty.Client      // HTTP-клиент
	ServerURL   string             // адрес сервера
}

// NewAgent создаёт и возвращает новый экземпляр агента
func NewAgent(serverURL string) *Agent {
	return &Agent{
		Metrics:   make(map[string]float64), // инициализируем хранилище метрик
		Client:    resty.New(),              // Создаём HTTP-клиент resty
		ServerURL: serverURL,                // Адрес сервера, куда будем отправлять метрики
	}
}

// sendMetricJSON отправляет одну метрику на сервер в формате JSON, сжатом через gzip
func (a *Agent) sendMetricJSON(metric models.Metrics) error {

	// Сериализуем метрику в JSON
	var jsonBuf bytes.Buffer
	if err := json.NewEncoder(&jsonBuf).Encode(metric); err != nil {
		logger.Log.Debug("json encode error:", zap.Error(err))
		return err
	}

	// Сжимаем JSON в gzip
	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	if _, err := gz.Write(jsonBuf.Bytes()); err != nil {
		logger.Log.Debug("gzip write error:", zap.Error(err))
		return err
	}
	if err := gz.Close(); err != nil {
		logger.Log.Debug("gzip close error:", zap.Error(err))
		return err
	}

	// Отправляем сжатый JSON
	return retry.DoIf(context.Background(), httpDelays, func(ctx context.Context) error {

		req := a.Client.R().
			SetHeader("Content-Type", "application/json").
			SetHeader("Content-Encoding", "gzip").
			SetHeader("Accept-Encoding", "gzip"). // Говорим серверу: "Я поддерживаю сжатые ответы"
			SetBody(gzBuf.Bytes())

		if flagKey != "" {
			hashStr := cryptohelpers.Sign(jsonBuf.Bytes(), flagKey) // Вычисляем HMAC-SHA256 от JSON
			req.SetHeader("HashSHA256", hashStr)
		}

		resp, err := req.Post(a.ServerURL + "/update")
		if err != nil {
			// сетевой/транспортный сбой — считаем ретраибл, вернём err
			logger.Log.Debug("send error", zap.Error(err))
			return err
		}
		// 502/503/504 — ретраим
		if resp.StatusCode() == http.StatusBadGateway ||
			resp.StatusCode() == http.StatusServiceUnavailable ||
			resp.StatusCode() == http.StatusGatewayTimeout {
			return fmt.Errorf("temporary server error %d", resp.StatusCode())
		}
		// 4xx — НЕ ретраим, сразу фейл
		if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
			return fmt.Errorf("client error %d: %s", resp.StatusCode(), resp.String())
		}
		// успех
		logger.Log.Debug("metric sent", zap.String("id", metric.ID), zap.String("type", metric.MType))
		return nil
	}, func(err error) bool {
		// retryIf: ретраим только сетевые ошибки (err != nil)
		if err == nil {
			return false
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return true
		}
		// обрыв соединения/временная недоступность — тоже ретраим
		return true
	})

}

// collectMetrics собирает метрики из runtime и обновляет состояние агента
func (a *Agent) collectMetrics() {

	var m runtime.MemStats   // Считываем текущие значения метрик
	runtime.ReadMemStats(&m) // Обновляем метрики в агенте

	// обновляем runtime метрики
	a.Metrics["Alloc"] = float64(m.Alloc)
	a.Metrics["BuckHashSys"] = float64(m.BuckHashSys)
	a.Metrics["Frees"] = float64(m.Frees)
	a.Metrics["GCCPUFraction"] = m.GCCPUFraction
	a.Metrics["GCSys"] = float64(m.GCSys)
	a.Metrics["HeapAlloc"] = float64(m.HeapAlloc)
	a.Metrics["HeapIdle"] = float64(m.HeapIdle)
	a.Metrics["HeapInuse"] = float64(m.HeapInuse)
	a.Metrics["HeapObjects"] = float64(m.HeapObjects)
	a.Metrics["HeapReleased"] = float64(m.HeapReleased)
	a.Metrics["HeapSys"] = float64(m.HeapSys)
	a.Metrics["LastGC"] = float64(m.LastGC)
	a.Metrics["Lookups"] = float64(m.Lookups)
	a.Metrics["MCacheInuse"] = float64(m.MCacheInuse)
	a.Metrics["MCacheSys"] = float64(m.MCacheSys)
	a.Metrics["MSpanInuse"] = float64(m.MSpanInuse)
	a.Metrics["MSpanSys"] = float64(m.MSpanSys)
	a.Metrics["Mallocs"] = float64(m.Mallocs)
	a.Metrics["NextGC"] = float64(m.NextGC)
	a.Metrics["NumForcedGC"] = float64(m.NumForcedGC)
	a.Metrics["NumGC"] = float64(m.NumGC)
	a.Metrics["OtherSys"] = float64(m.OtherSys)
	a.Metrics["PauseTotalNs"] = float64(m.PauseTotalNs)
	a.Metrics["StackInuse"] = float64(m.StackInuse)
	a.Metrics["StackSys"] = float64(m.StackSys)
	a.Metrics["Sys"] = float64(m.Sys)
	a.Metrics["TotalAlloc"] = float64(m.TotalAlloc)

	a.RandomValue = rand.Float64() // Обновляем случайное значение метрики
	a.PollCount++                  // Увеличиваем счётчик обновлений
}

func main() {

	// обрабатываем аргументы командной строки
	if err := parseFlags(); err != nil {
		log.Fatal(err)
	}

	// запускаем агента
	reportInterval := time.Duration(flagReportInterval) * time.Second // Интервал отправки метрик на сервер, по умолчанию 10 секунд
	pollInterval := time.Duration(flagPollInterval) * time.Second     // Интервал обновления метрик, по умолчанию 2 секунды

	agent := NewAgent(flagRunAddr) // Создаём нового агента с адресом сервера

	// Канал заданий на отправку
	jobs := make(chan models.Metrics, 2048)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// (а) Сбор runtime по pollInterval — только обновляет состояние агентa
	go func() {
		t := time.NewTicker(pollInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				agent.collectMetrics()
			}
		}
	}()

	// (б) Формирование заданий для отправки по reportInterval
	go func() {
		t := time.NewTicker(reportInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				// gauge из карты
				for name, val := range agent.Metrics {
					v := val
					jobs <- models.Metrics{ID: name, MType: "gauge", Value: &v}
				}
				// RandomValue как gauge
				rv := agent.RandomValue
				jobs <- models.Metrics{ID: "RandomValue", MType: "gauge", Value: &rv}
				// PollCount как counter
				pc := agent.PollCount
				jobs <- models.Metrics{ID: "PollCount", MType: "counter", Delta: &pc}
			}
		}
	}()

	// (в) Системные метрики через gopsutil (каждые 5s)
	go collectSysLoop(ctx, 5*time.Second, jobs)

	// Пул воркеров ограничивает число одновременных исходящих запросов
	_ = startWorkers(ctx, flagRateLimit, jobs, agent)

	// Блокируемся (упрощённо). Для graceful shutdown можно ловить SIGINT/SIGTERM.
	select {}

}
