package trace

import (
	"context"

	"go.uber.org/zap"

	"github.com/avnpl/go-march/utils"
)

func WithRequestID(ctx context.Context, logger *zap.Logger) *zap.Logger {
	if rid := utils.GetRequestID(ctx); rid != "" {
		return logger.With(zap.String("requestID", rid))
	}
	return logger
}

func Info(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithRequestID(ctx, logger).Info(msg, fields...)
}

func Error(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithRequestID(ctx, logger).Error(msg, fields...)
}

func Debug(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithRequestID(ctx, logger).Debug(msg, fields...)
}

func Warn(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithRequestID(ctx, logger).Warn(msg, fields...)
}