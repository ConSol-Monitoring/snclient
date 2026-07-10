package utils

import (
	"context"

	"github.com/kdar/factorlog"
)

type loggerCtxKey struct{}

func ContextWithLogger(ctx context.Context, log *factorlog.FactorLog) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, log)
}

func LoggerFromContext(ctx context.Context) *factorlog.FactorLog {
	if log, ok := ctx.Value(loggerCtxKey{}).(*factorlog.FactorLog); ok {
		return log
	}

	return nil
}
