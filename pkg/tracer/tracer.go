package tracer

import (
	tr "go.opentelemetry.io/otel/trace"
	"sync"
)

type Tracer struct {
	// back link to manager
	manager *TracerManager

	// identifier number
	number int

	// TraceID
	traceID string

	// historical span count
	numSpan uint64

	// preSpan buffer
	// 被 ConsumeFlow 并发访问，被 Assemble 单独访问
	bufPreSpan []*PreSpan
	muPreSpan  sync.Mutex

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

// 入表
func (t *Tracer) addPreSpan(preSpan *PreSpan) {
	t.muPreSpan.Lock()
	t.bufPreSpan = append(t.bufPreSpan, preSpan)
	t.numSpan++
	t.muPreSpan.Unlock()
	// inserter 自身有锁
	t.manager.olap.InsertL7Span(preSpan)
}
