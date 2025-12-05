package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v6"
)

// неэкспортированная переменная flagRunAddr содержит адрес и порт для запроса
var (
	flagRunAddr        string
	flagReportInterval int64
	flagPollInterval   int64
	flagKey            string
	flagRateLimit      int
	flagCryptoKey      string
)

// Config — слой переменных окружения
type Config struct {
	RunAddr        string `env:"ADDRESS"`
	ReportInterval int    `env:"REPORT_INTERVAL"` // в секундах
	PollInterval   int    `env:"POLL_INTERVAL"`   // в секундах
	Key            string `env:"KEY"`
	RateLimit      int    `env:"RATE_LIMIT"`
	CryptoKey      string `env:"CRYPTO_KEY"`
	ConfigPath     string `env:"CONFIG"` // путь к JSON-конфигу
}

// AgentFileConfig — слой JSON-конфига
//
//	{
//	  "address": "localhost:8080",
//	  "report_interval": "1s",
//	  "poll_interval": "1s",
//	  "crypto_key": "/path/to/key.pem"
//	}
type AgentFileConfig struct {
	Address        string `json:"address"`
	ReportInterval string `json:"report_interval"`
	PollInterval   string `json:"poll_interval"`
	CryptoKey      string `json:"crypto_key"`
}

// parseFlags обрабатывает аргументы командной строки
// Приоритет: значения из файла < переменные окружения < флаги
func parseFlags() error {
	// ----- 1. Базовые дефолты (как раньше) -----
	defaultRunAddr := "http://localhost:8080"
	defaultReportInterval := int64(10) // секунд
	defaultPollInterval := int64(2)    // секунд
	defaultRateLimit := 1

	runAddr := defaultRunAddr
	reportInterval := defaultReportInterval
	pollInterval := defaultPollInterval
	key := ""
	rateLimit := defaultRateLimit
	cryptoKey := ""

	// ----- 2. Читаем CONFIG из окружения -----
	var envCfg Config
	_ = env.Parse(&envCfg) // пока интересен envCfg.ConfigPath и компания

	configPath := envCfg.ConfigPath

	// ----- 3. Смотрим -c / -config в os.Args (флаги перекрывают CONFIG для пути к файлу) -----
	if argPath := findConfigPathInArgs(os.Args[1:]); argPath != "" {
		configPath = argPath
	}

	// ----- 4. Если указан JSON-файл — читаем и накладываем слой "file" -----
	if configPath != "" {
		if fileCfg, err := readAgentFileConfig(configPath); err == nil {
			if fileCfg.Address != "" {
				runAddr = fileCfg.Address
			}
			if fileCfg.ReportInterval != "" {
				if secs := parseDurationToSeconds(fileCfg.ReportInterval); secs > 0 {
					reportInterval = secs
				}
			}
			if fileCfg.PollInterval != "" {
				if secs := parseDurationToSeconds(fileCfg.PollInterval); secs > 0 {
					pollInterval = secs
				}
			}
			if fileCfg.CryptoKey != "" {
				cryptoKey = fileCfg.CryptoKey
			}
		} else {
			fmt.Fprintf(os.Stderr, "cannot read agent config file %s: %v\n", configPath, err)
		}
	}

	// ----- 5. Накладываем слой "env" поверх файла -----

	if envCfg.RunAddr != "" {
		runAddr = envCfg.RunAddr
	}

	if envCfg.ReportInterval > 0 {
		reportInterval = int64(envCfg.ReportInterval)
	}

	if envCfg.PollInterval > 0 {
		pollInterval = int64(envCfg.PollInterval)
	}

	if envCfg.Key != "" {
		key = envCfg.Key
	}

	if envCfg.RateLimit > 0 {
		rateLimit = envCfg.RateLimit
	}

	if envCfg.CryptoKey != "" {
		cryptoKey = envCfg.CryptoKey
	}

	// ----- 6. Регистрируем флаги с дефолтами из (file+env) -----

	// Флаг -a=<ЗНАЧЕНИЕ> отвечает за адрес эндпоинта HTTP-сервера.
	flag.StringVar(&flagRunAddr, "a", runAddr, "address and port")

	// -r: частота отправки метрик на сервер, в секундах
	flag.Int64Var(&flagReportInterval, "r", reportInterval, "report interval in seconds")

	// -p: частота опроса метрик runtime, в секундах
	flag.Int64Var(&flagPollInterval, "p", pollInterval, "poll interval in seconds")

	flag.StringVar(&flagKey, "k", key, "Key")

	flag.IntVar(&flagRateLimit, "l", rateLimit, "max concurrent outbound requests (RATE_LIMIT)")

	flag.StringVar(&flagCryptoKey, "crypto-key", cryptoKey, "path to RSA public key file")

	// чтобы -c/-config отображались в help, но значение уже прочитано выше
	var configDummy string
	flag.StringVar(&configDummy, "config", configPath, "path to JSON config file")
	flag.StringVar(&configDummy, "c", configPath, "path to JSON config file (shorthand)")

	// ----- 7. Парсим переданные аргументы в зарегистрированные переменные (флаги — самый высокий приоритет) -----
	flag.Parse()

	// Нормализуем адрес: добавляем http:// если нужно
	if !strings.HasPrefix(flagRunAddr, "http://") && !strings.HasPrefix(flagRunAddr, "https://") {
		flagRunAddr = "http://" + flagRunAddr
	}

	// проверка на неизвестные аргументы
	if len(flag.Args()) > 0 {
		return fmt.Errorf("неизвестные аргументы: %v", flag.Args())
	}

	// RATE_LIMIT — защита от нуля/отрицательных значений
	if flagRateLimit <= 0 {
		flagRateLimit = 1
	}

	return nil
}

// findConfigPathInArgs ищет -c / -config в сыром os.Args (до flag.Parse)
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

// readAgentFileConfig читает JSON-конфиг агента
func readAgentFileConfig(path string) (AgentFileConfig, error) {
	var cfg AgentFileConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read agent config file %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("unmarshal agent config file %q: %w", path, err)
	}
	return cfg, nil
}

// parseDurationToSeconds парсит "1s", "5m", "2h" → секунды
func parseDurationToSeconds(s string) int64 {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return int64(d.Seconds())
}
