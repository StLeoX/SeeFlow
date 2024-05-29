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

	// set of active TraceIDs
	setActiveTraceID map[string]bool
	muActiveTraceID  sync.Mutex

	// cache: SpanID -> flow
	bufFlow *lru.Cache[string, *observerpb.Flow]

	// 每一个 TraceID 上有一个 WG 做同步，保证写数据库的操作完成。
	// todo map 改成 lru
	// map: TraceID -> WG
	mapWgL7Consume map[string]sync.WaitGroup

	ShutdownCtx context.Context

	tracerProvider *sdktr.TracerProvider

	olap *Olap
}

func NewTracerManager(vp *viper.Viper) *TracerManager {
	var tm TracerManager
	tm.ShutdownCtx = context.Background()
	tm.tracers, _ = lru.New[string, *Tracer](config.MaxNumTracer)
	tm.setActiveTraceID = make(map[string]bool, 0)
	tm.bufFlow, _ = lru.New[string, *observerpb.Flow](config.MaxNumFlow)
	tm.mapWgL7Consume = make(map[string]sync.WaitGroup, 0)

	if vp == nil {
		tm.olap = nil // under testing
	} else {
		tm.olap = NewOlap(vp)
	}

	return &tm
}

func (tm *TracerManager) Olap() *Olap {
	if tm.olap == nil {
		logrus.Error("SeeFlow couldn't use nil olap")
		return nil
	}
	return tm.olap
}

func (tm *TracerManager) newTracer(traceID string) *Tracer {
	var a Tracer
	a.manager = tm
	tm.numTracer.Add(1)
	a.traceID = traceID
	a.bufPreSpan = make([]*PreSpan, 0)
	a.mapService = make(map[uint32]*PostSpan, 0)

	a.tracer = tm.tracerProvider.Tracer(fmt.Sprintf("tracer#%d", a.traceID))

	a.debMapTraceID = make(map[string]string, 0)
	a.debMapSpanID = make(map[string]string, 0)
	a.debMapParent = make(map[string]string, 0)

	tm.tracers.Add(traceID, &a)
	// 淘汰之前肯定是要聚合的
	if tm.tracers.Len() == config.MaxNumTracer {
		_, evict, _ := tm.tracers.RemoveOldest()
		err := evict.Assemble(kAssemble_BasicAssemble, tm.ShutdownCtx)
		if err != nil {
			logrus.Warn(err)
		}
	}

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

func (tm *TracerManager) AssembleAll() {
	for at := range tm.setActiveTraceID {
		tm.Assemble(at)
	}
	// tm.setActiveTraceID.clear()

}

// 状态机控制在更加上层
func (tm *TracerManager) Assemble(traceID string) {
	// 不接受无效（空）TraceID。
	if !convertTraceID(traceID).IsValid() {
		return
	}

	wg, hit := tm.mapWgL7Consume[traceID]
	if !hit {
		// 不存在这个 WG，说明该 TraceID 下没消费过 Span。
		return
	}
	wg.Wait()

	t := tm.newTracer(traceID)
	// 直接从数据库拉取 span 到 t.bufPreSpan
	// 已按 StartTime 字段升序排序
	tm.olap.SelectL7Spans(&t.bufPreSpan, t.traceID)
	err := t.Assemble(kAssemble_BasicAssemble, tm.ShutdownCtx)
	if err != nil {
		logrus.Warn(err)
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
	// 日志插入数量（todo）
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
