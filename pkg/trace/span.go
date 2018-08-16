package trace

import (
	"context"
	"github.com/opentracing/opentracing-go"
)

// InsideSpan runs f inside span named as operation name, runs opentracing.StartSpanFromContext internally
func InsideSpan(ctx context.Context, operationName string, f func(ctx context.Context, span opentracing.Span)) {
	span, ctx := opentracing.StartSpanFromContext(ctx, operationName)
	defer span.Finish()
	f(ctx, span)
}

// InsideSpanE same as InsideSpan but callable may return error, in such case it will be logged on span and returned
func InsideSpanE(ctx context.Context, operationName string, f func(ctx context.Context, span opentracing.Span) error) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, operationName)
	defer span.Finish()
	err = f(ctx, span)
	if err != nil {
		LogError(span, err)
	}
	return
}
