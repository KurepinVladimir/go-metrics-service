package repository

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"sync"
	"time"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
)

// Storage описывает поведение хранилища метрик.
// Реализация должна быть потокобезопасной.
type Storage interface {
	// UpdateGauge устанавливает значение метрики типа gauge для указанного имени.
	UpdateGauge(ctx context.Context, name string, value float64)
	// UpdateCounter увеличивает значение метрики типа counter для указанного имени на value.
	UpdateCounter(ctx context.Context, name string, value int64)
	// GetGauge возвращает текущее значение gauge и признак наличия метрики.
	GetGauge(ctx context.Context, name string) (float64, bool)
	// GetCounter возвращает текущее значение counter и признак наличия метрики.
	GetCounter(ctx context.Context, name string) (int64, bool)
	// GetAllMetrics возвращает копии всех метрик gauge и counter.
	GetAllMetrics(ctx context.Context) (map[string]float64, map[string]int64)
}

// Опциональное расширение: если реализация его поддержит — применим батч атомарно.
// BatchUpdater описывает опциональное пакетное обновление метрик.
// Если реализация поддерживает интерфейс, сервер может применять батч атомарно.
type BatchUpdater interface {
	// UpdateBatch применяет список метрик batch (gauge/counter).
	// Отсутствующие значения (nil) пропускаются.
	UpdateBatch(ctx context.Context, batch []models.Metrics) error
}

// MemStorage — потокобезопасное in-memory хранилище метрик.
// Подходит для локального запуска, тестов и простых сценариев без внешней БД.
type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

// NewMemStorage создаёт и возвращает пустое in-memory хранилище метрик.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

// UpdateGauge устанавливает значение метрики типа gauge
func (s *MemStorage) UpdateGauge(_ context.Context, name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value
}

// UpdateCounter увеличивает значение метрики типа counter
func (s *MemStorage) UpdateCounter(_ context.Context, name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += value
}

// GetGauge возвращает значение метрики типа gauge и признак её наличия.
func (s *MemStorage) GetGauge(_ context.Context, name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.gauges[name]
	return val, ok
}

// GetCounter возвращает значение метрики типа counter и признак её наличия.
func (s *MemStorage) GetCounter(_ context.Context, name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.counters[name]
	return val, ok
}

// GetAllMetrics возвращает копии всех метрик gauge и counter.
// Возвращаются именно копии, чтобы потребитель не мог изменять внутреннее состояние.
func (s *MemStorage) GetAllMetrics(_ context.Context) (map[string]float64, map[string]int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// создаём копии, чтобы не отдавать оригинальные мапы
	gaugeCopy := make(map[string]float64, len(s.gauges))
	for k, v := range s.gauges {
		gaugeCopy[k] = v
	}
	counterCopy := make(map[string]int64, len(s.counters))
	for k, v := range s.counters {
		counterCopy[k] = v
	}
	return gaugeCopy, counterCopy
}

// SaveToFile сохраняет текущее состояние метрик в JSON-файл filename.
// Формат соответствует структуре models.Metrics.
func (s *MemStorage) SaveToFile(filename string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var metrics []models.Metrics
	for id, value := range s.gauges {
		val := value
		metrics = append(metrics, models.Metrics{
			ID:    id,
			MType: "gauge",
			Value: &val,
		})
	}
	for id, delta := range s.counters {
		d := delta
		metrics = append(metrics, models.Metrics{
			ID:    id,
			MType: "counter",
			Delta: &d,
		})
	}

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, fs.FileMode(0666))
}

// LoadFromFile загружает состояние метрик из JSON-файла filename.
// Если файл отсутствует, метод возвращает nil (это не считается ошибкой).
func (s *MemStorage) LoadFromFile(filename string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // если файла нет — это не ошибка
		}
		return err
	}

	var metrics []models.Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return err
	}

	for _, mtr := range metrics {
		switch mtr.MType {
		case "gauge":
			if mtr.Value != nil {
				s.gauges[mtr.ID] = *mtr.Value
			}
		case "counter":
			if mtr.Delta != nil {
				s.counters[mtr.ID] = *mtr.Delta
			}
		}
	}

	return nil
}

// PeriodicStore периодически сохраняет состояние хранилища в файл filename
// с заданным интервалом interval. Метод блокирует текущую горутину и должен
// запускаться отдельно (например, в goroutine).
func (s *MemStorage) PeriodicStore(filename string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		_ = s.SaveToFile(filename)
	}
}

// UpdateBatch применяет пакет обновлений batch к хранилищу.
// Пропускает элементы с отсутствующими значениями (nil).
func (s *MemStorage) UpdateBatch(ctx context.Context, batch []models.Metrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, met := range batch {
		switch met.MType {
		case "gauge":
			if met.Value == nil {
				continue
			}
			s.gauges[met.ID] = *met.Value
		case "counter":
			if met.Delta == nil {
				continue
			}

			s.counters[met.ID] += *met.Delta
		}
	}
	return nil
}
