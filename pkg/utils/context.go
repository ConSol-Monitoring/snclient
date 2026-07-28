package utils

import (
	"context"

	"github.com/kdar/factorlog"
)

type loggerCtxKey struct{}

const (
	loggerCtxKey2 string = "github.com/consol-monitoring/snclient/pkg/utils.Logger"
)

//nolint:ireturn,staticcheck,revive,lll // context helpers must return context.Context by convention . If the context is used in a package that does not have access to loggerCtxKey, it can use the loggerCtxKey2
func ContextWithLogger(ctx context.Context, log *factorlog.FactorLog) context.Context {
	return context.WithValue(context.WithValue(ctx, loggerCtxKey{}, log),
		loggerCtxKey2, log)
}

func LoggerFromContext(ctx context.Context) *factorlog.FactorLog {
	if log, ok := ctx.Value(loggerCtxKey{}).(*factorlog.FactorLog); ok {
		return log
	}

	return nil
}
