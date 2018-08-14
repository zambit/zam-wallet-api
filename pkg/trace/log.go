package trace

import (
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

const (
	stackKey     = "stack"
	msgKey       = "message"
	eventKey     = "event"
	errorKindKey = "error.kind"
)

// LogError logs error according to opentracing specs (https://github.com/opentracing/specification/blob/master/semantic_conventions.md)
// Also accepts optional kind argument
func LogError(span opentracing.Span, err error, kind ...string) {
	ext.Error.Set(span, true)
	span.LogKV(
		eventKey, "error",
		msgKey, err.Error(),
	)
	if len(kind) > 0 {
		span.LogKV(errorKindKey, kind[0])
	}
}

// LogErrorWithMsg same as LogError but provides additional error description
func LogErrorWithMsg(span opentracing.Span, err error, msg string, kind ...string) {
	ext.Error.Set(span, true)
	span.LogKV(
		eventKey, "error",
		msgKey, msg + ": " + err.Error(),
	)
	if len(kind) > 0 {
		span.LogKV(errorKindKey, kind[0])
	}
}

// LogMsg log message
func LogMsg(span opentracing.Span, msg string) {
	span.LogKV(msgKey, msg)
}

// LogMsgf
func LogMsgf(span opentracing.Span, msg string, args ...interface{}) {
	LogMsg(span, fmt.Sprintf(msg, args))
}
