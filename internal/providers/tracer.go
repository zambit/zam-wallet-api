package providers

import (
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	jconfig "github.com/uber/jaeger-client-go/config"
)

// logrusJaegerLogAdapter adapts logrus logger to jaeger interfaces
type logrusJaegerLogAdapter struct {
	logger logrus.FieldLogger
}

func (l logrusJaegerLogAdapter) Error(msg string) {
	l.logger.Error(msg)
}

func (l logrusJaegerLogAdapter) Infof(msg string, args ...interface{}) {
	l.logger.Infof(msg, args)
}

// Tracer crates default jaeger tracer and set it as global tracer
func Tracer(cfg jconfig.Configuration, logger logrus.FieldLogger) (opentracing.Tracer, error) {
	tracer, _, err := cfg.NewTracer(jconfig.Logger(logrusJaegerLogAdapter{logger}))
	if err != nil {
		return nil, errors.Wrap(err, "jaeger provider")
	}
	opentracing.SetGlobalTracer(tracer)
	return tracer, nil
}
