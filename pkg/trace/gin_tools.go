package trace

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

const SpanKey = "span"

var ErrMiddlewareMissing = errors.New("tracing: trace middleware is missing")

// StartSpanMiddleware starts span with handler full name as
func StartSpanMiddleware(opts ...opentracing.StartSpanOption) gin.HandlerFunc {
	return func(c *gin.Context) {
		tracer := opentracing.GlobalTracer()
		opName := c.Request.URL.String()

		var span opentracing.Span

		spanContext, extractErr := tracer.Extract(opentracing.TextMap, opentracing.HTTPHeadersCarrier(c.Request.Header))
		if extractErr != nil {
			// don't do anything on error, just create new orphan span, report it later
			span = opentracing.StartSpan(opName, opts...)
		} else {
			opts = append([]opentracing.StartSpanOption{opentracing.ChildOf(spanContext)}, opts...)
			span = tracer.StartSpan(opName, opts...)
		}
		defer span.Finish()

		if extractErr != nil && extractErr != opentracing.ErrSpanContextNotFound {
			// don't report span not found err because it's pointless
			span.LogKV("extract_span_err", extractErr)
		}
		// also report url which was used to access future handler
		span.SetTag("http.method", c.Request.Method)
		span.SetTag("http.url", c.Request.URL.String())

		// propagate span thought gin context
		c.Set(SpanKey, span)

		// call next
		c.Next()
	}
}

// GetSpan returns span associated with gin context, panics with ErrMiddlewareMissing if span not attached to a context
// (e.g. middleware missing)
func GetSpan(c *gin.Context) opentracing.Span {
	val, ok := c.Get(SpanKey)
	if !ok {
		panic(ErrMiddlewareMissing)
	}
	span, ok := val.(opentracing.Span)
	if !ok {
		panic(ErrMiddlewareMissing)
	}
	return span
}

// GetSpan creates contex with span embedded
func GetSpanWithCtx(c *gin.Context) (span opentracing.Span, ctx context.Context) {
	span = GetSpan(c)
	ctx = opentracing.ContextWithSpan(c, span)
	return
}
