package middleware

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KurepinVladimir/go-musthave-metrics-tpl.git/internal/cryptohelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecryptRSA_NoKey_PassThrough — проверяет, что при priv == nil middleware просто прокидывает тело дальше, ничего не меняя (важно, когда CRYPTO_KEY не задан).
// 1. Если ключ не задан (priv == nil) — middleware ничего не трогает.
func TestDecryptRSA_NoKey_PassThrough(t *testing.T) {
	body := []byte("plain body")

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	var got []byte

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		got = b
	})

	// priv == nil → должно просто пропустить тело как есть
	mw := DecryptRSA(nil)
	mw(next).ServeHTTP(rr, req)

	assert.Equal(t, body, got)
	// по умолчанию статус будет 200, если next не писал явный код
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestDecryptRSA_DecryptsBody — проверяет, что:
// - тело приходит в middleware зашифрованным (EncryptRSA),
// - внутри вызывается cryptohelpers.DecryptRSA,
// - в next-handler попадают расшифрованные байты.
// 2. Если ключ задан — тело должно расшифровываться.
func TestDecryptRSA_DecryptsBody(t *testing.T) {
	// Генерим тестовый ключ (не для продакшена, но для теста ок).
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	plain := []byte("secret body")

	// Шифруем так же, как это делает агент: EncryptRSA по публичному ключу.
	enc, err := cryptohelpers.EncryptRSA(&priv.PublicKey, plain)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(enc))
	rr := httptest.NewRecorder()

	var got []byte

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		got = b
	})

	mw := DecryptRSA(priv)
	mw(next).ServeHTTP(rr, req)

	assert.Equal(t, plain, got)
	assert.Equal(t, http.StatusOK, rr.Code)
}
