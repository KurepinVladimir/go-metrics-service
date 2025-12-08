package middleware

import (
	"bytes"
	"io"
	"net/http"

	"crypto/rsa"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/cryptohelpers"
	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/logger"
	"go.uber.org/zap"
)

func DecryptRSA(priv *rsa.PrivateKey) func(http.Handler) http.Handler {
	// если ключ не задан — просто пробрасываем дальше
	if priv == nil {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				logger.Log.Error("failed to read request body", zap.Error(err))
				//  клиенту — только стандартную ошибку без деталей
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			_ = r.Body.Close()

			if len(body) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			plain, err := cryptohelpers.DecryptRSA(priv, body)
			if err != nil {
				logger.Log.Warn("failed to decrypt request body", zap.Error(err))
				//  клиенту — только стандартную ошибку без деталей
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(plain))

			next.ServeHTTP(w, r)
		})
	}
}
