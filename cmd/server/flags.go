package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v6"
)

// глобальные переменные, которые использует main.go
var flagRunAddr string
var flagStoreInterval int64
var flagFileStoragePath string
var flagRestore bool
var flagDatabaseDSN string
var flagKey string
var flagAuditFile string
var flagAuditURL string
var flagCryptoKey string
var flagTrustedSubnet string
var flagGRPCAddr string

// Config — слой переменных окружения
type Config struct {
	RunAddr         string `env:"ADDRESS"`
	StoreInterval   int64  `env:"STORE_INTERVAL"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
	Restore         bool   `env:"RESTORE"`
	DatabaseDSN     string `env:"DATABASE_DSN"`
	Key             string `env:"KEY"`
	AuditFile       string `env:"AUDIT_FILE"`
	AuditURL        string `env:"AUDIT_URL"`
	CryptoKey       string `env:"CRYPTO_KEY"`
	ConfigPath      string `env:"CONFIG"` // путь к JSON-конфигу
	TrustedSubnet   string `env:"TRUSTED_SUBNET"`
	GRPCAddr        string `env:"GRPC_ADDRESS"`
}

// FileConfig — слой JSON-файла
//
//	{
//	  "address": "localhost:8080",
//	  "restore": true,
//	  "store_interval": "1s",
//	  "store_file": "/path/to/file.db",
//	  "database_dsn": "",
//	  "crypto_key": "/path/to/key.pem",
//	  "trusted_subnet": "192.168.0.0/24"я
//	  "grpc_address": "localhost:50051"
//	}
type FileConfig struct {
	Address       string `json:"address"`
	Restore       *bool  `json:"restore"`
	StoreInterval string `json:"store_interval"`
	StoreFile     string `json:"store_file"`
	DatabaseDSN   string `json:"database_dsn"`
	CryptoKey     string `json:"crypto_key"`
	TrustedSubnet string `json:"trusted_subnet"`
	GRPCAddr      string `json:"grpc_address"`
}

// parseFlags обрабатывает аргументы командной строки и JSON/ENV конфиг
// Приоритет: значения из файла < переменные окружения < флаги
func parseFlags() {
	// ----- 1. Базовые дефолты -----
	defaultRunAddr := ":8080"
	defaultStoreInterval := int64(300) // секунды
	defaultFileStoragePath := "/tmp/metrics-db.json"
	defaultRestore := false

	// промежуточные "текущие" значения, поверх них будем накладывать слои
	runAddr := defaultRunAddr
	storeInterval := defaultStoreInterval
	fileStoragePath := defaultFileStoragePath
	restore := defaultRestore
	databaseDSN := ""
	key := ""
	auditFile := ""
	auditURL := ""
	cryptoKey := ""
	trustedSubnet := ""
	grpcAddr := ""

	// ----- 2. Читаем CONFIG из env -----
	var envCfg Config
	_ = env.Parse(&envCfg) // пока важен только envCfg.ConfigPath и компания

	configPath := envCfg.ConfigPath

	// ----- 3. Смотрим -c / -config в os.Args ДО flag.Parse (флаги имеют больший приоритет для пути) -----
	if pathFromArgs := findConfigPathInArgs(os.Args[1:]); pathFromArgs != "" {
		configPath = pathFromArgs
	}

	// ----- 4. Если указали JSON-файл — читаем и накладываем слой "file" -----
	if configPath != "" {
		if fileCfg, err := readFileConfig(configPath); err == nil {
			// address
			if fileCfg.Address != "" {
				runAddr = fileCfg.Address
			}
			// store_interval: строка "1s" → секунды
			if fileCfg.StoreInterval != "" {
				if secs := parseDurationToSeconds(fileCfg.StoreInterval); secs > 0 {
					storeInterval = secs
				}
			}
			// store_file → FILE_STORAGE_PATH
			if fileCfg.StoreFile != "" {
				fileStoragePath = fileCfg.StoreFile
			}
			// restore
			if fileCfg.Restore != nil {
				restore = *fileCfg.Restore
			}
			// database_dsn
			if fileCfg.DatabaseDSN != "" {
				databaseDSN = fileCfg.DatabaseDSN
			}
			// crypto_key
			if fileCfg.CryptoKey != "" {
				cryptoKey = fileCfg.CryptoKey
			}
			if fileCfg.TrustedSubnet != "" {
				trustedSubnet = fileCfg.TrustedSubnet
			}
			if fileCfg.GRPCAddr != "" {
				grpcAddr = fileCfg.GRPCAddr
			}
		} else {
			log.Printf("cannot read config file %s: %v", configPath, err)
		}
	}

	// ----- 5. Накладываем слой "env" поверх файла -----
	// envCfg уже заполнен через env.Parse(&envCfg)

	if envCfg.RunAddr != "" {
		runAddr = envCfg.RunAddr
	}
	if v, ok := os.LookupEnv("STORE_INTERVAL"); ok && v != "" {
		// тут STORE_INTERVAL — в секундах (int64)
		storeInterval = envCfg.StoreInterval
	}
	if envCfg.FileStoragePath != "" {
		fileStoragePath = envCfg.FileStoragePath
	}
	if _, ok := os.LookupEnv("RESTORE"); ok {
		restore = envCfg.Restore
	}
	if envCfg.DatabaseDSN != "" {
		databaseDSN = envCfg.DatabaseDSN
	}
	if envCfg.Key != "" {
		key = envCfg.Key
	}
	if envCfg.AuditFile != "" {
		auditFile = envCfg.AuditFile
	}
	if envCfg.AuditURL != "" {
		auditURL = envCfg.AuditURL
	}
	if envCfg.CryptoKey != "" {
		cryptoKey = envCfg.CryptoKey
	}
	if envCfg.TrustedSubnet != "" {
		trustedSubnet = envCfg.TrustedSubnet
	}
	if envCfg.GRPCAddr != "" {
		grpcAddr = envCfg.GRPCAddr
	}

	// ----- 6. Регистрируем флаги с дефолтами из (file+env) -----
	flag.StringVar(&flagRunAddr, "a", runAddr, "address and port to run server")
	flag.Int64Var(&flagStoreInterval, "i", storeInterval, "storage interval in seconds")
	flag.StringVar(&flagFileStoragePath, "f", fileStoragePath, "file storage path")
	flag.BoolVar(&flagRestore, "r", restore, "restore from file storage")
	flag.StringVar(&flagDatabaseDSN, "d", databaseDSN, "database DSN for persistent storage")
	flag.StringVar(&flagKey, "k", key, "Key")
	flag.StringVar(&flagAuditFile, "audit-file", auditFile, "path to audit log file")
	flag.StringVar(&flagAuditURL, "audit-url", auditURL, "URL of remote audit log server")
	flag.StringVar(&flagCryptoKey, "crypto-key", cryptoKey, "path to RSA private key file")
	flag.StringVar(&flagTrustedSubnet, "t", trustedSubnet, "trusted subnet in CIDR (TRUSTED_SUBNET)")
	flag.StringVar(&flagGRPCAddr, "grpc", grpcAddr, "gRPC listen address (e.g. localhost:3200)")

	// чтобы -c/-config отображались в help, но реальное значение мы уже обработали выше
	var configDummy string
	flag.StringVar(&configDummy, "config", configPath, "path to JSON config file")
	flag.StringVar(&configDummy, "c", configPath, "path to JSON config file (shorthand)")

	// ----- 7. Парсим флаги (флаги — самый высокий приоритет) -----
	flag.Parse()

	// ----- 8. Проверка на неизвестные аргументы -----
	if len(flag.Args()) > 0 {
		log.Fatalf("Неизвестные аргументы: %v", flag.Args())
	}
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

// readFileConfig читает JSON-конфиг сервера
func readFileConfig(path string) (FileConfig, error) {
	var cfg FileConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config file %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("unmarshal config file %q: %w", path, err)
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
