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
