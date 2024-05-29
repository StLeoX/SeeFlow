package tracer

import (
	tr "go.opentelemetry.io/otel/trace"
)

type Tracer struct {
	// back link to manager
	manager *TracerManager

	// TraceID
	traceID string

	// historical span count
	numSpan int

	// preSpan buffer
	// 被 Assemble 单独访问
	bufPreSpan []*PreSpan

	// Pod DAG: dest_identity -> preSpan
	// 被 Assemble 单线程访问
	mapService map[uint32]*PostSpan

	// tracer
	tracer tr.Tracer

	// for debug
	// spanName -> traceID
	debMapTraceID map[string]string
	// spanName -> spanID
	debMapSpanID map[string]string
	// spanName -> parentId
	debMapParent map[string]string
}
