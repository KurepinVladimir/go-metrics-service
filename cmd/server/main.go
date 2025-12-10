package main

import (
	"context"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/audit"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/buildinfo"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/cryptohelpers"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/handler"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/logger"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/middleware"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/repository"
	"github.com/go-chi/chi/v5"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

var rsaPrivateKey *rsa.PrivateKey

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
				aud.Notify(r.Context(), []string{name}, handler.ClientIP(r))
			}

		case "counter":
			value, err := strconv.ParseInt(valueStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid counter value", http.StatusBadRequest)
				return
			}
			storage.UpdateCounter(r.Context(), name, value)

			if aud != nil && aud.Enabled() {
				aud.Notify(r.Context(), []string{name}, handler.ClientIP(r))
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
				aud.Notify(r.Context(), []string{m.ID}, ip)
			}
		case "counter":
			if m.Delta == nil {
				http.Error(w, "missing counter delta", http.StatusBadRequest)
				return
			}
			storage.UpdateCounter(r.Context(), m.ID, *m.Delta)
			if aud != nil && aud.Enabled() {
				ip := handler.ClientIP(r)
				aud.Notify(r.Context(), []string{m.ID}, ip)
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

	buildinfo.Print()

	// обрабатываем аргументы командной строки
	parseFlags()

	if err := run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}

}

// функция run будет полезна при инициализации зависимостей сервера перед запуском
func run() error {

	if flagCryptoKey != "" {
		key, err := cryptohelpers.LoadPrivateKeyFromFile(flagCryptoKey)
		if err != nil {
			log.Fatalf("failed to load RSA private key from %s: %v", flagCryptoKey, err)
		}
		rsaPrivateKey = key
	}

	var trustedNet *net.IPNet
	if flagTrustedSubnet != "" {
		_, network, err := net.ParseCIDR(flagTrustedSubnet)
		if err != nil {
			return fmt.Errorf("invalid trusted_subnet %q: %w", flagTrustedSubnet, err)
		}
		trustedNet = network
	}

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

	// Сжимаем ответы, если клиент поддерживает gzip
	r.Use(gzipResponseMiddleware)

	// middleware для подписи и расшифровки
	hashMiddleware := middleware.ValidateHashSHA256(flagKey)
	decryptMiddleware := middleware.DecryptRSA(rsaPrivateKey)
	trustedSubnetMiddleware := middleware.ValidateTrustedSubnet(trustedNet)

	// --- "текстовые" ручки без шифрования/HMAC ---
	r.Post("/update/{type}/{name}/{value}", updateHandler(storage, aud)) // Регистрируем маршрут с параметрами
	r.Get("/value/{type}/{name}", valueHandler(storage))
	r.Get("/", indexHandler(storage))
	if db != nil {
		r.Get("/ping", pingHandler(db)) //проверяет соединение с базой данных.
	}

	// --- JSON-эндпоинты, куда стучится агент: RSA → gzip-распаковка → проверка HMAC ---
	r.Group(func(r chi.Router) {
		// 0) проверяем, что агентский IP входит в доверенную подсеть
		r.Use(trustedSubnetMiddleware)
		// 1) сначала расшифровываем тело (если есть приватный ключ)
		r.Use(decryptMiddleware)
		// 2) потом, если Content-Encoding: gzip, распаковываем
		r.Use(gzipRequestMiddleware)
		// 3) потом проверяем подпись по JSON
		r.Use(hashMiddleware)

		r.Post("/update", updateHandlerJSON(storage, aud))
		r.Post("/update/", updateHandlerJSON(storage, aud))

		r.Post("/updates", handler.UpdatesHandler(storage, flagKey, aud))
		r.Post("/updates/", handler.UpdatesHandler(storage, flagKey, aud))
	})

	// JSON-ручка /value оставлена без шифрования/HMAC (для удобства внешних клиентов)
	r.Post("/value", valueHandlerJSON(storage))
	r.Post("/value/", valueHandlerJSON(storage))

	// Готовим http.Server, чтобы уметь делать Shutdown
	srv := &http.Server{
		Addr:    flagRunAddr,
		Handler: r,
	}

	// Канал для ошибок сервера
	errCh := make(chan error, 1)

	// Запускаем сервер в отдельной горутине
	go func() {
		logger.Log.Info("Running server", zap.String("address", flagRunAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	// Канал для сигналов ОС
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case sig := <-stop:
		logger.Log.Info("Received shutdown signal", zap.String("signal", sig.String()))

		// Даём активным запросам возможность доработать
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Log.Error("HTTP server shutdown error", zap.Error(err))
			return err
		}

		// Финально сохраняем метрики, если работаем с MemStorage и настроен файл
		if memStorage, ok := storage.(*repository.MemStorage); ok && flagFileStoragePath != "" {
			if err := memStorage.SaveToFile(flagFileStoragePath); err != nil {
				logger.Log.Warn("Failed to save metrics on shutdown", zap.Error(err))
			}
		}

		// При желании можно закрыть соединение с БД
		if db != nil {
			if err := db.Close(); err != nil {
				logger.Log.Warn("Failed to close DB on shutdown", zap.Error(err))
			}
		}

		return nil

	case err := <-errCh:
		// Сервер упал сам по себе, не через Shutdown
		if err != nil {
			return err
		}
		return nil
	}
}
