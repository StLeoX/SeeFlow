package tracer

import (
	"context"
	"github.com/sirupsen/logrus"
	"github.com/stleox/seeflow/pkg/config"
	attr "go.opentelemetry.io/otel/attribute"
	sdktr "go.opentelemetry.io/otel/sdk/trace"
	tr "go.opentelemetry.io/otel/trace"
)

type PreSpan = L7FlowEntity

type PostSpan struct {
	preSpan *PreSpan
	ctx     context.Context
	spanID  tr.SpanID
}

func (t *Tracer) Assemble(amCtx context.Context) error {
	// todo: 策略模式
	// todo set the checkpoint for Assemble
	logrus.Debugf("call BasicAssemble() from #%d", t.number)
	return t.BasicAssemble(amCtx)
}

// BasicAssemble
// 离线算法，输入完备的 Span 数据。
// 返回 trace，返回的是该 Trace 的根 Span，提供给测试。
func (t *Tracer) BasicAssemble(amCtx context.Context) error {
	// 从 olap 拉取 span 到 t.bufPreSpan，已按 StartTime 升序排序
	t.manager.olap.SelectL7Spans(&t.bufPreSpan, t.traceID)

	if len(t.bufPreSpan) == 0 {
		return nil
	}

	// 遍历进行 parent 关联
	for _, preSpan := range t.bufPreSpan {
		var curPreSpan *PreSpan
		var curSpanID tr.SpanID
		var parentCtx context.Context
		// 检查缓存
		if hitPostSpan, hit := t.mapService[preSpan.SrcIdentity]; !hit {
			// 缺失，构建 tr.Span
			// 缺失的情况应该仅限于 Root Span
			curPreSpan = preSpan
			parentCtx, curSpanID = t.buildTrSpan(amCtx, curPreSpan, tr.SpanID{})
		} else {
			// 命中，构建 tr.Span
			curPreSpan = preSpan
			parentPostSpan := hitPostSpan
			parentCtx, curSpanID = t.buildTrSpan(parentPostSpan.ctx, curPreSpan, parentPostSpan.spanID)
			// 命中后不能清除 Span 因为可能再次命中，一个 parentSpan 有若干 childSpan。
		}

		// 命中或缺失，Span 都要入表
		t.mapService[preSpan.DestIdentity] = &PostSpan{
			preSpan: curPreSpan,
			ctx:     parentCtx,
			spanID:  curSpanID,
		}
	}
	return nil
}

func (t *Tracer) buildTrSpan(parentCtx context.Context, childSpan *PreSpan, parentSpanID tr.SpanID) (context.Context, tr.SpanID) {
	startOpts := make([]tr.SpanStartOption, 0)
	startOpts = append(startOpts, tr.WithTimestamp(childSpan.StartTime))
	startOpts = append(startOpts, tr.WithAttributes(attr.String("src", childSpan.SrcPod)))
	startOpts = append(startOpts, tr.WithAttributes(attr.String("dest", childSpan.DestPod)))

	// 暂时不知 TraceFlags 硬编码为 0x01 的后果，所以加个判断去除无效 SpanID
	traceFlags := tr.TraceFlags(0x01)
	if !parentSpanID.IsValid() {
		traceFlags = 0x00
	}

	parentSpanCtx := tr.NewSpanContext(tr.SpanContextConfig{
		TraceID:    convertTraceID(t.traceID),
		SpanID:     parentSpanID,
		TraceFlags: traceFlags,
	})

	parentCtx = tr.ContextWithSpanContext(parentCtx, parentSpanCtx)
	ctx, span := t.tracer.Start(parentCtx, constructSpanName(childSpan), startOpts...)
	span.End(tr.WithTimestamp(childSpan.EndTime))

	if config.Debug {
		// try to convert to sdktr.ReadOnlySpan
		switch span := span.(type) {
		case sdktr.ReadOnlySpan:
			logrus.Debugf("span name: %s, span ID: %s, parent span ID: %s",
				span.Name(), span.SpanContext().SpanID(), span.Parent().SpanID())
			t.debMapTraceID[span.Name()] = span.SpanContext().TraceID().String()
			t.debMapSpanID[span.Name()] = span.SpanContext().SpanID().String()
			t.debMapParent[span.Name()] = span.Parent().SpanID().String()
		default:
			logrus.Debugf("can't convert to ReadOnlySpan, span ID: %s", span.SpanContext().SpanID())
		}
	}

	return ctx, span.SpanContext().SpanID()
}
