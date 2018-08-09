package providers

import (
	"fmt"
	"github.com/opentracing/opentracing-go"
	jaeger "github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
)

// Tracer crates default jaeger tracer and set it as global tracer
func Tracer() (opentracing.Tracer, error) {
	cfg := &config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans: true,
		},
	}
	tracer, _, err := cfg.New("wallet-api", config.Logger(jaeger.StdLogger))
	if err != nil {
		return nil, fmt.Errorf("ERROR: cannot init Jaeger: %v\n", err)
	}
	opentracing.SetGlobalTracer(tracer)
	return tracer, nil
}
