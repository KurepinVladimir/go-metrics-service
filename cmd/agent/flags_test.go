package main

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------- Тест 1: значения берутся из JSON-конфига ----------

func TestParseFlags_Agent_ConfigFileOnly(t *testing.T) {
	// JSON-конфиг только с файлом, без env и флагов
	cfgJSON := `{
		"address": "localhost:9000",
		"report_interval": "5s",
		"poll_interval": "3s",
		"crypto_key": "/tmp/agent-file-key.pem"
	}`

	// Создаём временный файл с конфигом
	f, err := os.CreateTemp("", "agent-config-*.json")
	assert.NoError(t, err)
	defer os.Remove(f.Name())

	_, err = f.WriteString(cfgJSON)
	assert.NoError(t, err)
	assert.NoError(t, f.Close())

	// Чистим окружение, чтобы не мешало
	t.Setenv("CONFIG", "")
	t.Setenv("ADDRESS", "")
	t.Setenv("REPORT_INTERVAL", "")
	t.Setenv("POLL_INTERVAL", "")
	t.Setenv("CRYPTO_KEY", "")
	t.Setenv("RATE_LIMIT", "")

	// Подменяем os.Args так, будто агент запустили с -config
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"agent.test", "-config", f.Name()}

	// Сбрасываем флаговый пакет, чтобы parseFlags мог заново регистрировать флаги
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	err = parseFlags()
	assert.NoError(t, err)

	// Адрес из файла + автодобавление http://
	assert.Equal(t, "http://localhost:9000", flagRunAddr)
	// "5s" → 5 секунд
	assert.Equal(t, int64(5), flagReportInterval)
	// "3s" → 3 секунды
	assert.Equal(t, int64(3), flagPollInterval)
	// crypto_key
	assert.Equal(t, "/tmp/agent-file-key.pem", flagCryptoKey)
	// RATE_LIMIT должен быть безопасным дефолтом
	assert.Equal(t, 1, flagRateLimit)
}

// ---------- Тест 2: приоритет file < env < flags ----------

func TestParseFlags_Agent_Priority_FileEnvFlags(t *testing.T) {
	// Слой file
	cfgJSON := `{
		"address": "file-host:8000",
		"report_interval": "1s",
		"poll_interval": "2s",
		"crypto_key": "/tmp/agent-file-key.pem"
	}`

	f, err := os.CreateTemp("", "agent-config-*.json")
	assert.NoError(t, err)
	defer os.Remove(f.Name())

	_, err = f.WriteString(cfgJSON)
	assert.NoError(t, err)
	assert.NoError(t, f.Close())

	// Слой env: должен перекрыть file, но быть слабее флагов
	t.Setenv("CONFIG", "") // путь к файлу зададим флагом
	t.Setenv("ADDRESS", "env-host:8100")
	t.Setenv("REPORT_INTERVAL", "7") // секунд
	t.Setenv("POLL_INTERVAL", "8")   // секунд
	t.Setenv("CRYPTO_KEY", "/tmp/agent-env-key.pem")
	t.Setenv("RATE_LIMIT", "3")

	// Слой flags: самый приоритетный
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{
		"agent.test",
		"-config", f.Name(), // путь к JSON-файлу
		"-a", "flag-host:8200", // перекрывает ADDRESS
		"-r", "10", // перекрывает REPORT_INTERVAL
		"-p", "20", // перекрывает POLL_INTERVAL
		"-l", "5", // перекрывает RATE_LIMIT
	}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	err = parseFlags()
	assert.NoError(t, err)

	// ADDRESS: file ("file-host:8000") < env ("env-host:8100") < flag ("flag-host:8200")
	// плюс автодобавление "http://"
	assert.Equal(t, "http://flag-host:8200", flagRunAddr)

	// REPORT_INTERVAL: file ("1s" → 1) < env (7) < flag (10)
	assert.Equal(t, int64(10), flagReportInterval)

	// POLL_INTERVAL: file ("2s" → 2) < env (8) < flag (20)
	assert.Equal(t, int64(20), flagPollInterval)

	// CRYPTO_KEY: file < env, флага нет → env
	assert.Equal(t, "/tmp/agent-env-key.pem", flagCryptoKey)

	// RATE_LIMIT: file (дефолт 1) < env (3) < flag (5)
	assert.Equal(t, 5, flagRateLimit)
}

// ---------- Тест 3: RATE_LIMIT не может быть <= 0 ----------

func TestParseFlags_Agent_RateLimit_DefaultSafe(t *testing.T) {
	// Без env и без флага -l
	t.Setenv("CONFIG", "")
	t.Setenv("ADDRESS", "")
	t.Setenv("REPORT_INTERVAL", "")
	t.Setenv("POLL_INTERVAL", "")
	t.Setenv("CRYPTO_KEY", "")
	t.Setenv("RATE_LIMIT", "")

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"agent.test"} // без флагов

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	err := parseFlags()
	assert.NoError(t, err)

	// Должен быть безопасный дефолт
	assert.Equal(t, 1, flagRateLimit)
}
