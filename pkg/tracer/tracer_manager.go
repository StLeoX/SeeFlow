package tracer

import (
	"context"
	"fmt"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stleox/seeflow/pkg/config"
	sdktr "go.opentelemetry.io/otel/sdk/trace"
	"sync"
	"sync/atomic"
)

type TracerManager struct {
	numTracer atomic.Int32

	// cache: TraceID -> Tracer
	tracers *lru.Cache[string, *Tracer]

	// cache: SpanID -> flow
	bufFlow *lru.Cache[string, *observerpb.Flow]

	wgL7Consume sync.WaitGroup

	ShutdownCtx context.Context

	tracerProvider *sdktr.TracerProvider

	olap *Olap
}

func NewTracerManager(vp *viper.Viper) *TracerManager {
	var tm TracerManager
	tm.ShutdownCtx = context.Background()
	tm.tracers, _ = lru.New[string, *Tracer](config.MaxNumTracer)
	tm.bufFlow, _ = lru.New[string, *observerpb.Flow](config.MaxNumFlow)

	if vp == nil {
		tm.olap = nil // under testing
	} else {
		tm.olap = NewOlap(vp)
	}

	return &tm
}

func (tm *TracerManager) newTracer(traceID string) *Tracer {
	var a Tracer
	a.manager = tm
	a.number = int(tm.numTracer.Load())
	tm.numTracer.Add(1)
	a.traceID = traceID
	a.bufPreSpan = make([]*PreSpan, 0)
	a.mapService = make(map[uint32]*PostSpan, 0)

	a.tracer = tm.tracerProvider.Tracer(fmt.Sprintf("tracer#%d", a.number))

	a.debMapTraceID = make(map[string]string, 0)
	a.debMapSpanID = make(map[string]string, 0)
	a.debMapParent = make(map[string]string, 0)

	tm.tracers.Add(traceID, &a)
	// 淘汰之前肯定是要聚合的
	if tm.tracers.Len() == config.MaxNumTracer {
		_, evict, _ := tm.tracers.RemoveOldest()
		err := evict.Assemble(tm.ShutdownCtx)
		if err != nil {
			logrus.Error(err)
		}
	}

	logrus.Debugf("add new tracer#%d for trace: %s", a.number, traceID)
	return &a
}

// ConsumeFlow
// 消耗一条 Flow 数据，针对不同的 flow 类型进行分发
func (tm *TracerManager) ConsumeFlow(flow *observerpb.Flow) {
	// 调试模式下记录流量
	if config.Debug {
		config.Log4RawL7.Debug(flow)
	}

	// 分发处理流量
	switch flow.Type {
	case observerpb.FlowType_L3_L4:
		l34 := L34Flow{tm: tm}
		l34.Consume(flow)
	case observerpb.FlowType_SOCK:
		sock := SockFlow{tm: tm}
		sock.Consume(flow)
	case observerpb.FlowType_L7:
		l7 := L7Flow{tm: tm}
		l7.Consume(flow)
	default:
		logrus.Warnf("SeeFlow met unsupportted Cilium flow: %s", flow.Type.String())
	}
}

// These hooked on defer-point of observe cmd:

func (tm *TracerManager) Assemble() {
	// todo: 设计聚集的触发。目前是接收到全体 flow，并对全体 trace 聚合。
	tm.wgL7Consume.Wait()

	for _, a := range tm.tracers.Values() {
		// 验证收集结果
		logrus.Debugf("tracer#%d collected spans: %d", a.number, a.numSpan)
		err := a.Assemble(tm.ShutdownCtx)
		if err != nil {
			logrus.Error(err)
		}
		tm.tracers.Remove(a.traceID)
	}

}

func (tm *TracerManager) Flush() {
	tm.olap.l34Inserter.Flush()
	tm.olap.l7Inserter.Flush()
	tm.olap.sockInserter.Flush()
}

func (tm *TracerManager) Summary() {
	// 日志异常流量
	tm.olap.SummaryExFlows()
	// 日志插入数量，省略掉
}

func (tm *TracerManager) CheckSpansCount(trace_id string) bool {
	current := tm.olap.countL7Spans("trace_id", trace_id)

	history := 0
	tracer, hit := tm.tracers.Get(trace_id)
	if hit {
		history = tracer.numSpan
	}
	return current != history
}
