package utils

import (
	"encoding/json"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"net/http"
	"os"
	"strings"
)

func getEnvVar(key string) string {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalln("Error loading .env file...")
	}
	return os.Getenv(key)
}

func BuildLogger() *zap.Logger {
	loggerConfig := zap.NewDevelopmentConfig()

	err := os.MkdirAll("logs", 0755)
	if err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	loggerConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	loggerConfig.Development = true
	loggerConfig.EncoderConfig.TimeKey = "ts"
	loggerConfig.EncoderConfig.MessageKey = "event"
	loggerConfig.EncoderConfig.CallerKey = "caller"
	loggerConfig.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	loggerConfig.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	loggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	loggerConfig.EncoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	loggerConfig.Encoding = "console"
	loggerConfig.OutputPaths = []string{"stdout", "logs/app.log"}
	loggerConfig.ErrorOutputPaths = []string{"stderr", "logs/app.log"}
	loggerConfig.DisableStacktrace = false

	logger, err := loggerConfig.Build(
		zap.AddStacktrace(zap.ErrorLevel),
	)
	if err != nil {
		log.Fatal("cannot build logger", err)
	}
	return logger
}

func GetDBPoolObject(logger *zap.Logger) *sqlx.DB {
	dsn := getEnvVar("DB_URL")
	if dsn == "" {
		logger.Fatal("DB_URL not set")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		logger.Fatal("db connect failed", zap.Error(err))
	}
	return db
}

func SendJSONError(w http.ResponseWriter, statusCode int, message string) {
	apiErr := APIError{
		Error:   http.StatusText(statusCode),
		Message: message,
	}

	if strings.TrimSpace(apiErr.Message) == "" {
		apiErr.Message = "Something went wrong"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(apiErr)
}

func SendInternalError(w http.ResponseWriter) {
	SendJSONError(w, http.StatusInternalServerError, "")
}
