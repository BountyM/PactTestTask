package config

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
)

type Config struct {
	GRPC        GRPCConfig        `envPrefix:"GRPC_"`
	TelegramApp TelegramAppConfig `envPrefix:"TELEGRAM_APP_"`
	Logger      Logger            `envPrefix:"LOGGER_"`
}

type GRPCConfig struct {
	Port string `env:"PORT" envDefault:"50051"`
}

type TelegramAppConfig struct {
	ID   int    `env:"ID" envDefault:"12345"`
	Hash string `env:"HASH" envDefault:"abcde"`
}

// Config для логгера
type Logger struct {
	Level  string `env:"LEVEL" envDefault:"INFO"`
	Format string `env:"FORMAT" envDefault:"json"` // "json" или "text"
}

func Load() (*Config, error) {
	// Определяем путь к текущему файлу (config.go)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("failed to get current file path")
	}
	basepath := filepath.Dir(filename) // internal/config
	envPath := filepath.Join(basepath, ".env")

	// Загружаем .env файл (если есть)
	err := godotenv.Load(envPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load env")
	}

	// Парсим переменные окружения в структуру Config
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}
