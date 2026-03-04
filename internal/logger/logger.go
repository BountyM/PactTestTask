package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/BountyM/PactTestTask/internal/config"
)

// Config для логгера можно расширить позже
type Config struct {
	Level  string
	Format string // "json" или "text"
}

// New создаёт логгер на основе переменных окружения.
// Переменные:
//
//	LOG_LEVEL - debug, info, warn, error (по умолчанию info)
//	LOG_FORMAT - json или text (по умолчанию json)
func New(cfg config.Logger) *slog.Logger {
	level := parseLevel(cfg.Level)
	format := parseFormat(cfg.Format)

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
		// Можно добавить AddSource: true для разработки
		// AddSource: level == slog.LevelDebug,
	}

	switch format {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func parseLevel(levelStr string) slog.Level {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		if levelStr != "" {
			// Только предупреждаем, если значение было указано, но не распознано
			fmt.Fprintf(os.Stderr, "Unknown LOG_LEVEL %q, using INFO\n", levelStr)
		}
		return slog.LevelInfo
	}
}

func parseFormat(format string) string {

	if format != "text" && format != "json" {
		// По умолчанию json
		return "json"
	}
	return format
}
