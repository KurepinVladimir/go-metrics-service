package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/buildinfo"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/cryptohelpers"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/logger"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/proto"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/retry"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var httpDelays = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}
var rsaPublicKey *rsa.PublicKey

// Agent инкапсулирует состояние и поведение агента для сбора и отправки метрик на сервер
type Agent struct {
	PollCount   int64              // счётчик обновлений метрик
	RandomValue float64            // случайное значение метрики
	Metrics     map[string]float64 // метрики типа gauge из runtime
	Client      *resty.Client      // HTTP-клиент
	ServerURL   string             // адрес сервера
	RealIP      string             // IP-адрес хоста агента
}

// NewAgent создаёт и возвращает новый экземпляр агента
func NewAgent(serverURL string) *Agent {
	return &Agent{
		Metrics:   make(map[string]float64), // инициализируем хранилище метрик
		Client:    resty.New(),              // Создаём HTTP-клиент resty
		ServerURL: serverURL,                // Адрес сервера, куда будем отправлять метрики
		RealIP:    detectAgentIP(),          // Пытаемся определить реальный IP-адрес агента
	}
}

func detectAgentIP() string {
	ifaces, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range ifaces {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}

	return ""
}

func (a *Agent) realIP() string {
	if a.RealIP == "" {
		a.RealIP = detectAgentIP()
		if a.RealIP == "" {
			a.RealIP = "127.0.0.1"
		}
	}
	return a.RealIP
}

// sendMetricJSON отправляет одну метрику на сервер в формате JSON, сжатом через gzip
func (a *Agent) sendMetricJSON(metric models.Metrics) error {

	// Сериализуем метрику в JSON
	var jsonBuf bytes.Buffer
	if err := json.NewEncoder(&jsonBuf).Encode(metric); err != nil {
		return fmt.Errorf("encode metric %q to JSON: %w", metric.ID, err)
	}

	// Сжимаем JSON в gzip
	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)

	if _, err := gz.Write(jsonBuf.Bytes()); err != nil {
		return fmt.Errorf("gzip write metric %q: %w", metric.ID, err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("gzip close metric %q: %w", metric.ID, err)
	}

	// Отправляем сжатый (и, возможно, зашифрованный) JSON
	return retry.DoIf(context.Background(), httpDelays, func(ctx context.Context) error {
		bodyBytes := gzBuf.Bytes()

		if rsaPublicKey != nil {
			encBody, err := cryptohelpers.EncryptRSA(rsaPublicKey, bodyBytes)
			if err != nil {
				return fmt.Errorf("encrypt metric %q with RSA: %w", metric.ID, err)
			}
			bodyBytes = encBody
		}

		req := a.Client.R().
			SetHeader("Content-Type", "application/json").
			SetHeader("Content-Encoding", "gzip").
			SetHeader("Accept-Encoding", "gzip").
			SetHeader("X-Real-IP", a.realIP()).
			SetBody(bodyBytes)

		if flagKey != "" {
			hashStr := cryptohelpers.Sign(jsonBuf.Bytes(), flagKey)
			req.SetHeader("HashSHA256", hashStr)
		}

		resp, err := req.Post(a.ServerURL + "/update")
		if err != nil {
			// сетевой/транспортный сбой — ретраибл, вернём err
			return fmt.Errorf("send metric %q to %s/update: %w", metric.ID, a.ServerURL, err)
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
		logger.Log.Debug("metric sent",
			zap.String("id", metric.ID),
			zap.String("type", metric.MType))

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
	buildinfo.Print()

	// обрабатываем аргументы командной строки
	if err := parseFlags(); err != nil {
		log.Fatal(err)
	}

	if flagCryptoKey != "" {
		key, err := cryptohelpers.LoadPublicKeyFromFile(flagCryptoKey)
		if err != nil {
			log.Fatalf("failed to load RSA public key from %s: %v", flagCryptoKey, err)
		}
		rsaPublicKey = key
	}

	// интервалы работы агента
	reportInterval := time.Duration(flagReportInterval) * time.Second
	pollInterval := time.Duration(flagPollInterval) * time.Second

	agent := NewAgent(flagRunAddr)

	// gRPC-клиент, если указан адрес
	var grpcConn *grpc.ClientConn
	var grpcClient proto.MetricsClient
	if flagGRPCAddr != "" {
		conn, err := grpc.NewClient(flagGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("failed to connect to gRPC server %s: %v", flagGRPCAddr, err)
		}
		grpcConn = conn
		grpcClient = proto.NewMetricsClient(conn)
		defer grpcConn.Close()
	}

	// Канал заданий на отправку
	jobs := make(chan models.Metrics, 2048)

	// Контекст, который завершится по сигналу SIGINT/SIGTERM/SIGQUIT
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	// --- продюсеры (те, кто пишет в jobs) ---
	var prodWG sync.WaitGroup

	// (а) Сбор runtime по pollInterval
	prodWG.Add(1)
	go func() {
		defer prodWG.Done()
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
	prodWG.Add(1)
	go func() {
		defer prodWG.Done()
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

	// (в) Системные метрики через gopsutil
	prodWG.Add(1)
	go func() {
		defer prodWG.Done()
		collectSysLoop(ctx, 5*time.Second, jobs)
	}()

	// Пул воркеров ограничивает число одновременных исходящих запросов
	var wg *sync.WaitGroup
	if grpcClient != nil {
		wg = startGRPCDispatcher(ctx, reportInterval, jobs, grpcClient, agent.realIP())
	} else {
		wg = startWorkers(ctx, flagRateLimit, jobs, agent)
	}

	// ---- graceful shutdown ----
	<-ctx.Done()
	log.Println("agent: received shutdown signal")

	// ждём, пока продюсеры перестанут писать в канал
	prodWG.Wait()

	// теперь безопасно закрываем jobs — новых записей уже не будет
	close(jobs)

	// ждём, пока все воркеры дочитают и отправят оставшиеся метрики
	wg.Wait()
}
