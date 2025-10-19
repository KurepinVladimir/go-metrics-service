package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/audit"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/models"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/repository"
)

func UpdatesHandler(storage repository.Storage, key string, aud *audit.Auditor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if ct := r.Header.Get("Content-Type"); ct != "" && ct != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}

		// небольшая защита от больших тел
		limited := io.LimitReader(r.Body, 10<<20) // 10MB

		var batch []models.Metrics
		if err := json.NewDecoder(limited).Decode(&batch); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if len(batch) == 0 {
			http.Error(w, "empty batch", http.StatusBadRequest)
			return
		}

		// Если хранилище умеет атомарный батч — используем его
		if bu, ok := storage.(repository.BatchUpdater); ok {
			if err := bu.UpdateBatch(r.Context(), batch); err != nil {
				http.Error(w, "storage error", http.StatusInternalServerError)
				return
			}
		} else {
			// Фолбэк: поштучно
			for _, m := range batch {
				switch m.MType {
				case "gauge":
					if m.Value == nil {
						http.Error(w, "gauge without value", http.StatusBadRequest)
						return
					}
					storage.UpdateGauge(r.Context(), m.ID, *m.Value)
				case "counter":
					if m.Delta == nil {
						http.Error(w, "counter without delta", http.StatusBadRequest)
						return
					}
					storage.UpdateCounter(r.Context(), m.ID, *m.Delta)
				default:
					http.Error(w, "unknown mtype", http.StatusBadRequest)
					return
				}
			}
		}

		// собираем имена метрик ИЗ batch
		names := make([]string, 0, len(batch))
		for _, m := range batch {
			names = append(names, m.ID)
		}

		// аудит после успеха
		if aud != nil && aud.Enabled() {
			aud.Notify(r.Context(), names, ClientIP(r), time.Now)
		}

		//_ = WriteSignedJSONResponse(w, []byte(`{"status":"ok"}`), key)
		_ = WriteSignedJSONResponse(w, map[string]string{"status": "ok"}, key)
	}
}
