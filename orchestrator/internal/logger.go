package internal

import (
    "os"
    "strings"
    "time"

    "github.com/rs/zerolog"
)

func NewLogger(levelStr string) zerolog.Logger {
    levelStr = strings.ToLower(strings.TrimSpace(levelStr))
    level := zerolog.InfoLevel
    switch levelStr {
    case "debug":
        level = zerolog.DebugLevel
    case "warn":
        level = zerolog.WarnLevel
    case "error":
        level = zerolog.ErrorLevel
    case "fatal":
        level = zerolog.FatalLevel
    case "panic":
        level = zerolog.PanicLevel
    case "trace":
        level = zerolog.TraceLevel
    case "info":
        fallthrough
    default:
        level = zerolog.InfoLevel
    }

    zerolog.TimeFieldFormat = time.RFC3339
    logger := zerolog.New(os.Stdout).With().
        Timestamp().
        Str("service", "orchestrator").
        Logger().
        Level(level)

    return logger
}
