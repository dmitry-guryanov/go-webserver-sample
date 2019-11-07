package log

import (
	"context"

	"github.com/sirupsen/logrus"
)

type key int

const (
	loggerKey key = iota
)

var defaultLogger *logrus.Entry = logrus.NewEntry(logrus.StandardLogger())

// WithLogger attaches given logger to the context.
func WithLogger(ctx context.Context, logger *logrus.Entry) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// L returns logrus.Entry for logger from given context.
func L(ctx context.Context) *logrus.Entry {
	logger, ok := ctx.Value(loggerKey).(*logrus.Entry)
	if !ok {
		logger = defaultLogger
	}

	return logger
}

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}
