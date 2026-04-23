package utils

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getEnvVar(key string) string {
	return os.Getenv(key)
}

func GetEnvVarString(key string, defaultValue string, logger *zap.Logger) string {
	value := getEnvVar(key)

	if value == "" {
		logger.Warn("Variable not set in env")
		return defaultValue
	}
	return value
}

func GetEnvVarInteger(key string, defaultValue int, logger *zap.Logger) int {
	value := getEnvVar(key)
	if value == "" {
		logger.Warn("Key not present in env variables")
		return defaultValue
	}

	res, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		logger.Error("Error converting env variable to int")
		res = int64(defaultValue)
	}
	return int(res)
}

func BuildLogger() *zap.Logger {
	loggerConfig := zap.NewDevelopmentConfig()

	err := os.MkdirAll("logs", 0755)
	if err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	logLevelFromConfig := getEnvVar("LOG_LEVEL")
	if logLevelFromConfig == "" {
		logLevelFromConfig = "info"
	}

	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(logLevelFromConfig)); err != nil {
		log.Fatalf("Invalid log level in env : %v", err)
	}

	loggerConfig.Level = zap.NewAtomicLevelAt(level.Level())
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

	lifetime := GetEnvVarInteger("DB_MAX_CONN_LIFETIME_SEC", 10, logger)
	db.SetMaxOpenConns(GetEnvVarInteger("DB_MAX_OPEN_CONNS", 25, logger))
	db.SetMaxIdleConns(GetEnvVarInteger("DB_MAX_IDLE_CONNS", 10, logger))
	db.SetConnMaxLifetime(time.Duration(lifetime) * time.Second)
	if err := db.Ping(); err != nil {
		logger.Fatal("db ping failed", zap.Error(err))
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

func GenerateID(prefix string) string {
	const charSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result strings.Builder
	result.WriteString(prefix)
	result.WriteString("-")

	for range 7 {
		result.WriteByte(charSet[rand.Intn(36)])
	}

	return result.String()
}
