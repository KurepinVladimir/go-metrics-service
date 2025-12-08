package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Глобальные переменные, которые дальше использует main.go
var (
	flagRunAddr        string
	flagReportInterval int64
	flagPollInterval   int64
	flagKey            string
	flagRateLimit      int
	flagCryptoKey      string
)

// agentConfig — "склейка" конфигурации из файла + env.
// Viper будет мапить сюда значения по ключам:
//   - address         ← JSON "address" или env ADDRESS
//   - report_interval ← JSON "report_interval" или env REPORT_INTERVAL
//   - poll_interval   ← JSON "poll_interval" или env POLL_INTERVAL
//   - key             ← JSON "key" или env KEY
//   - rate_limit      ← JSON "rate_limit" или env RATE_LIMIT
//   - crypto_key      ← JSON "crypto_key" или env CRYPTO_KEY
type agentConfig struct {
	Address        string `mapstructure:"address"`
	ReportInterval string `mapstructure:"report_interval"`
	PollInterval   string `mapstructure:"poll_interval"`
	Key            string `mapstructure:"key"`
	RateLimit      int    `mapstructure:"rate_limit"`
	CryptoKey      string `mapstructure:"crypto_key"`
}

// parseFlags обрабатывает конфигурацию агента.
// Приоритет источников:
//
//  1. JSON-файл (CONFIG или -c/-config)
//  2. переменные окружения
//  3. флаги командной строки
//
// Viper объединяет (1) и (2) → даёт нам "базовый" конфиг.
// Потом мы регистрируем флаги с дефолтами из этого конфига,
// и флаги уже имеют самый высокий приоритет.
func parseFlags() error {
	// -------- 1. Определяем откуда брать путь к JSON-конфигу --------

	// Базово: из переменной окружения CONFIG
	configPath := os.Getenv("CONFIG")

	// Но флаги -c / -config должны иметь больший приоритет
	// Сначала вытащим их "вручную" из сырого os.Args, до flag.Parse.
	if argPath := findConfigPathInArgs(os.Args[1:]); argPath != "" {
		configPath = argPath
	}

	// -------- 2. Загружаем базовый конфиг (файл + env) через Viper --------

	cfg, err := loadAgentConfigWithViper(configPath)
	if err != nil {
		// Для учебного проекта — не падаем, а логируем на stderr и продолжаем
		// с дефолтами Viper (cfg будет нулём, но Viper нам уже подставил
		// дефолты при Unmarshal).
		fmt.Fprintf(os.Stderr, "agent: cannot load config %q: %v\n", configPath, err)
	}

	// report_interval и poll_interval у нас в конфиге хранятся как строка:
	//  - из файла: "1s", "5s", "2m"
	//  - из env: "10" (секунды), "10s" и т.п.
	// Приводим их к секундам для глобальных флагов.
	flagReportInterval = parseDurationToSeconds(cfg.ReportInterval, 10) // дефолт 10s
	flagPollInterval = parseDurationToSeconds(cfg.PollInterval, 2)      // дефолт 2s

	// -------- 3. Регистрируем флаги с дефолтами из (file+env) --------

	// Адрес сервера: дефолт берём из cfg.Address (файл/ENV).
	// Если в конфиге он пустой — Viper уже подставил "http://localhost:8080".
	flag.StringVar(&flagRunAddr, "a", cfg.Address, "address and port")

	// Частота отправки метрик, в секундах
	flag.Int64Var(&flagReportInterval, "r", flagReportInterval, "report interval in seconds")

	// Частота опроса runtime-метрик, в секундах
	flag.Int64Var(&flagPollInterval, "p", flagPollInterval, "poll interval in seconds")

	// Ключ для подписи HMAC
	flag.StringVar(&flagKey, "k", cfg.Key, "Key")

	// Ограничение числа параллельных запросов
	flag.IntVar(&flagRateLimit, "l", cfg.RateLimit, "max concurrent outbound requests (RATE_LIMIT)")

	// Путь к публичному RSA-ключу
	flag.StringVar(&flagCryptoKey, "crypto-key", cfg.CryptoKey, "path to RSA public key file")

	// Чтобы -config / -c были в help, но значение мы уже учитываем выше
	var configDummy string
	flag.StringVar(&configDummy, "config", configPath, "path to JSON config file")
	flag.StringVar(&configDummy, "c", configPath, "path to JSON config file (shorthand)")

	// -------- 4. Парсим флаги (они имеют высший приоритет) --------

	flag.Parse()

	// Проверка на неизвестные позиционные аргументы
	if len(flag.Args()) > 0 {
		return fmt.Errorf("неизвестные аргументы: %v", flag.Args())
	}

	// Нормализуем адрес: если пользователь указал "localhost:8080",
	// добавим префикс "http://".
	if !strings.HasPrefix(flagRunAddr, "http://") && !strings.HasPrefix(flagRunAddr, "https://") {
		flagRunAddr = "http://" + flagRunAddr
	}

	// RATE_LIMIT защитим от нуля и отрицательных значений.
	if flagRateLimit <= 0 {
		flagRateLimit = 1
	}

	return nil
}

// loadAgentConfigWithViper загружает конфиг агента:
//
// - задаёт дефолты (address, report_interval, poll_interval, rate_limit, key, crypto_key)
// - читает JSON-файл (если задан configPath)
// - читает переменные окружения (ADDRESS, REPORT_INTERVAL, ...)
// - возвращает склеенный agentConfig
func loadAgentConfigWithViper(configPath string) (agentConfig, error) {
	v := viper.New()

	// ----- Дефолты -----
	// Здесь мы задаём начальные значения, которые будут использованы,
	// если ни файл, ни ENV ничего не переопределили.
	v.SetDefault("address", "http://localhost:8080")
	v.SetDefault("report_interval", "10s") // строки "10s", "2s" и т.п.
	v.SetDefault("poll_interval", "2s")
	v.SetDefault("rate_limit", 1)
	v.SetDefault("key", "")
	v.SetDefault("crypto_key", "")

	// ----- ENV -----
	// Преобразуем ключи вида "report_interval" → "REPORT_INTERVAL"
	// чтобы Viper мог найти значение в переменных окружения.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Теперь:
	//   "address"         ← окружение ADDRESS
	//   "report_interval" ← окружение REPORT_INTERVAL
	//   "poll_interval"   ← окружение POLL_INTERVAL
	//   "rate_limit"      ← окружение RATE_LIMIT
	//   "key"             ← окружение KEY
	//   "crypto_key"      ← окружение CRYPTO_KEY

	// ----- JSON-файл (если указан) -----
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			// Оборачиваем ошибку с контекстом.
			return agentConfig{}, fmt.Errorf("read config file %q: %w", configPath, err)
		}
	}

	// ----- Unmarshal в структуру -----
	var cfg agentConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return agentConfig{}, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}

// parseDurationToSeconds принимает строку из конфига/env и приводит её к секундам.
//
// Поддерживает оба варианта:
//   - "10s", "1m", "2h"  → парсим через time.ParseDuration
//   - "10"               → трактуем как "10 секунд"
//
// Если строка пустая или нераспознаваемая — возвращаем дефолт.
func parseDurationToSeconds(s string, def int64) int64 {
	if s == "" {
		return def
	}

	// Сначала пробуем как duration ("1s", "2m", "500ms")
	if d, err := time.ParseDuration(s); err == nil {
		secs := int64(d.Seconds())
		if secs > 0 {
			return secs
		}
	}

	// Если не получилось — пробуем как целое число секунд ("10")
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
		return n
	}

	// В крайнем случае возвращаем дефолт
	return def
}

// findConfigPathInArgs ищет -c / -config в сырых аргументах os.Args.
// Это нужно сделать до flag.Parse, чтобы знать,
// какой файл конфигурации читать в loadAgentConfigWithViper.
func findConfigPathInArgs(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-c" || arg == "-config":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(arg, "-c="):
			return strings.TrimPrefix(arg, "-c=")
		case strings.HasPrefix(arg, "-config="):
			return strings.TrimPrefix(arg, "-config=")
		}
	}
	return ""
}
