package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/audit"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/handler"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/logger"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/middleware"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/repository"
	"github.com/go-chi/chi/v5"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

// handler обрабатывает POST-запросы на /update/{type}/{name}/{value}
func updateHandler(storage repository.Storage, aud *audit.Auditor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		metricType := chi.URLParam(r, "type")
		name := chi.URLParam(r, "name")
		valueStr := chi.URLParam(r, "value")

		if name == "" {
			http.Error(w, "Missing metric name", http.StatusNotFound)
			return
		}

		switch metricType {
		case "gauge":
			value, err := strconv.ParseFloat(valueStr, 64)
			if err != nil {
				http.Error(w, "Invalid gauge value", http.StatusBadRequest)
				return
			}
			storage.UpdateGauge(r.Context(), name, value)

			if aud != nil && aud.Enabled() {
				aud.Notify(r.Context(), []string{name}, handler.ClientIP(r), time.Now)
			}

		case "counter":
			value, err := strconv.ParseInt(valueStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid counter value", http.StatusBadRequest)
				return
			}
			storage.UpdateCounter(r.Context(), name, value)

			if aud != nil && aud.Enabled() {
				aud.Notify(r.Context(), []string{name}, handler.ClientIP(r), time.Now)
			}

		default:
			http.Error(w, "Invalid metric type", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	}
}

func updateHandlerJSON(storage repository.Storage, aud *audit.Auditor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// десериализуем запрос в структуру модели
		logger.Log.Debug("decoding request")
		var m models.Metrics
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&m); err != nil {
			logger.Log.Debug("cannot decode request JSON body", zap.Error(err))
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		switch m.MType {
		case "gauge":
			if m.Value == nil {
				http.Error(w, "missing gauge value", http.StatusBadRequest)
				return
			}
			storage.UpdateGauge(r.Context(), m.ID, *m.Value)
			if aud != nil && aud.Enabled() {
				ip := handler.ClientIP(r)
				aud.Notify(r.Context(), []string{m.ID}, ip, time.Now)
			}
		case "counter":
			if m.Delta == nil {
				http.Error(w, "missing counter delta", http.StatusBadRequest)
				return
			}
			storage.UpdateCounter(r.Context(), m.ID, *m.Delta)
			if aud != nil && aud.Enabled() {
				ip := handler.ClientIP(r)
				aud.Notify(r.Context(), []string{m.ID}, ip, time.Now)
			}
		default:
			http.Error(w, "unknown metric type", http.StatusNotImplemented)
			return
		}

		if err := handler.WriteSignedJSONResponse(w, m, flagKey); err != nil {
			logger.Log.Debug("error writing signed response", zap.Error(err))
		}

		logger.Log.Debug("sending HTTP 200 response")
	}
}

func valueHandlerJSON(storage repository.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var m models.Metrics
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		//w.Header().Set("Content-Type", "application/json")
		switch m.MType {
		case "gauge":
			val, ok := storage.GetGauge(r.Context(), m.ID)
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			m.Value = &val
		case "counter":
			val, ok := storage.GetCounter(r.Context(), m.ID)
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			m.Delta = &val
		default:
			http.Error(w, "unknown metric type", http.StatusNotImplemented)
			return
		}

		_ = handler.WriteSignedJSONResponse(w, m, flagKey)
	}
}

// GET /value/{type}/{name}
func valueHandler(storage repository.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metricType := chi.URLParam(r, "type")
		name := chi.URLParam(r, "name")

		switch metricType {
		case "gauge":
			val, ok := storage.GetGauge(r.Context(), name)
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, strconv.FormatFloat(val, 'f', -1, 64))

		case "counter":
			val, ok := storage.GetCounter(r.Context(), name)
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%d", val)

		default:
			http.Error(w, "invalid metric type", http.StatusBadRequest)
		}
	}
}

// GET /
func indexHandler(storage repository.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gauges, counters := storage.GetAllMetrics(r.Context())

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintln(w, "<html><body><h1>Metrics</h1><ul>")
		for name, val := range gauges {
			fmt.Fprintf(w, "<li>gauge %s = %f</li>\n", name, val)
		}
		for name, val := range counters {
			fmt.Fprintf(w, "<li>counter %s = %d</li>\n", name, val)
		}
		fmt.Fprintln(w, "</ul></body></html>")
	}
}

// GET /
func pingHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "DB not configured", http.StatusInternalServerError)
			return
		}
		if err := db.PingContext(r.Context()); err != nil {
			http.Error(w, "DB not available", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "pong")
	}
}

func initPostgres(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, nil // режим без БД
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB: %w", err)
	}
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping DB: %w", err)
	}
	logger.Log.Info("Connected to PostgreSQL successfully")
	return db, nil
}

func main() {

	// обрабатываем аргументы командной строки
	parseFlags()

	if err := run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}

}

// функция run будет полезна при инициализации зависимостей сервера перед запуском
func run() error {

	if err := logger.Initialize("INFO"); err != nil {
		return err
	}

	db, err := initPostgres(flagDatabaseDSN)
	if err != nil {
		return err
	}

	if db != nil {
		if err := repository.RunMigrations(flagDatabaseDSN); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	var storage repository.Storage
	if db != nil {
		storage = repository.NewPostgresStorage(db)
	} else {
		storage = repository.NewMemStorage()
	}

	// --- Инициализация аудитора (Observer sinks) ---
	var sinks []audit.Sink
	if flagAuditFile != "" {
		sinks = append(sinks, audit.NewFileSink(flagAuditFile))
	}
	if flagAuditURL != "" {
		sinks = append(sinks, audit.NewHTTPSink(flagAuditURL))
	}
	aud := audit.New(sinks...)

	// загружаем метрики из файла, если включено
	if memStorage, ok := storage.(*repository.MemStorage); ok && flagRestore && flagFileStoragePath != "" {
		if err := memStorage.LoadFromFile(flagFileStoragePath); err != nil {
			logger.Log.Warn("Failed to restore metrics", zap.Error(err))
		}
	}

	// запуск периодического сохранения, если установлен интервал > 0
	if memStorage, ok := storage.(*repository.MemStorage); ok && flagStoreInterval > 0 && flagFileStoragePath != "" {
		go memStorage.PeriodicStore(flagFileStoragePath, time.Duration(flagStoreInterval)*time.Second)
	}

	r := chi.NewRouter()

	//Use добавляет middleware ко всем маршрутам, зарегистрированным через chi.Router.
	r.Use(logger.RequestLogger)
	// Добавляем middleware для обработки gzip-запросов и ответов
	r.Use(gzipRequestMiddleware)
	r.Use(gzipResponseMiddleware)

	r.Post("/update/{type}/{name}/{value}", updateHandler(storage, aud)) // Регистрируем маршрут с параметрами

	hashMiddleware := middleware.ValidateHashSHA256(flagKey)

	r.With(hashMiddleware).Post("/update", updateHandlerJSON(storage, aud))
	r.With(hashMiddleware).Post("/update/", updateHandlerJSON(storage, aud))

	r.With(hashMiddleware).Post("/updates", handler.UpdatesHandler(storage, flagKey, aud))
	r.With(hashMiddleware).Post("/updates/", handler.UpdatesHandler(storage, flagKey, aud))

	r.Post("/value", valueHandlerJSON(storage))
	r.Post("/value/", valueHandlerJSON(storage))

	r.Get("/value/{type}/{name}", valueHandler(storage))
	r.Get("/", indexHandler(storage))

	if db != nil {
		r.Get("/ping", pingHandler(db)) //проверяет соединение с базой данных.
	}

	logger.Log.Info("Running server", zap.String("address", flagRunAddr))

	return http.ListenAndServe(flagRunAddr, r)
}
