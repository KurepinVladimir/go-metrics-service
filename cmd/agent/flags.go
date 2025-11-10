package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/caarlos0/env/v6"
)

// неэкспортированная переменная flagRunAddr содержит адрес и порт для запроса
var (
	flagRunAddr        string
	flagReportInterval int64
	flagPollInterval   int64
	flagKey            string
	flagRateLimit      int
)

type Config struct {
	RunAddr        string `env:"ADDRESS"`
	ReportInterval int    `env:"REPORT_INTERVAL"`
	PollInterval   int    `env:"POLL_INTERVAL"`
	Key            string `env:"KEY"`
	RateLimit      int    `env:"RATE_LIMIT"`
}

// parseFlags обрабатывает аргументы командной строки
func parseFlags() error {

	// регистрируем переменную flagRunAddr как аргумент -a со значением по умолчанию
	// Флаг -a=<ЗНАЧЕНИЕ> отвечает за адрес эндпоинта HTTP-сервера (по умолчанию localhost:8080). (":8080", "http://localhost:8080/update", "localhost:8080")
	flag.StringVar(&flagRunAddr, "a", "http://localhost:8080", "address and port")

	// Флаг -r=<ЗНАЧЕНИЕ> позволяет переопределять reportInterval — частоту отправки метрик на сервер (по умолчанию 10 секунд).
	flag.Int64Var(&flagReportInterval, "r", 10, "report interval in seconds") //10

	// Флаг -p=<ЗНАЧЕНИЕ> позволяет переопределять pollInterval — частоту опроса метрик из пакета runtime (по умолчанию 2 секунды).
	flag.Int64Var(&flagPollInterval, "p", 2, "poll interval in seconds") //2

	flag.StringVar(&flagKey, "k", "", "Key")

	flag.IntVar(&flagRateLimit, "l", 0, "max concurrent outbound requests (RATE_LIMIT)")

	// парсим переданные аргументы в зарегистрированные переменные
	flag.Parse()

	if !strings.HasPrefix(flagRunAddr, "http://") && !strings.HasPrefix(flagRunAddr, "https://") {
		flagRunAddr = "http://" + flagRunAddr
	}

	// проверка на неизвестные аргументы
	if len(flag.Args()) > 0 {
		return fmt.Errorf("неизвестные аргументы: %v", flag.Args())
	}

	// читаем переменные окружения и заполняем структуру Config
	// если переменные окружения не заданы, то будут использованы значения по умолчанию
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return err
	}

	if envRunAddr := cfg.RunAddr; envRunAddr != "" {
		flagRunAddr = envRunAddr
		if !strings.HasPrefix(flagRunAddr, "http://") && !strings.HasPrefix(flagRunAddr, "https://") {
			flagRunAddr = "http://" + flagRunAddr
		}
	}

	if envReportInterval := cfg.ReportInterval; envReportInterval != 0 {
		flagReportInterval = int64(envReportInterval)
	}

	if envPollInterval := cfg.PollInterval; envPollInterval != 0 {
		flagPollInterval = int64(envPollInterval)
	}

	if envKey := cfg.Key; envKey != "" {
		flagKey = envKey
	}

	// if envRateLimit := cfg.RateLimit; envRateLimit != 0 {
	// 	flagRateLimit = int(envRateLimit)
	// }

	// RATE_LIMIT из окружения — дефолт для -l
	if flagRateLimit == 0 && cfg.RateLimit > 0 {
		flagRateLimit = cfg.RateLimit
	}
	if flagRateLimit <= 0 {
		flagRateLimit = 1 // безопасный дефолт: без параллелизма
	}

	return nil

}
