package tracer

import (
	"context"
	"errors"
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

//nolint:revive
type TracerManager struct {
	numTracer atomic.Int32

	// cache: TraceID -> Tracer
	tracers *lru.Cache[string, *Tracer]

	// cache: SpanID -> flow
	// 被 ConsumeFlow 并发访问
	bufFlow *lru.Cache[string, *observerpb.Flow]
	// wg for ConsumeHttp
	wgConsumeHttp sync.WaitGroup
	// err for ConsumeHttp
	errConsumeHttp error
	// err for ConsumeL34
	errConsumeL34 error

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
	}
	tm.olap = NewOlap(vp)

	return &tm
}

func (tm *TracerManager) addTracer(traceID string) *Tracer {
	var a Tracer
	a.manager = tm
	a.number = int(tm.numTracer.Load())
	tm.numTracer.Add(1)
	a.traceID = traceID
	a.bufPreSpan = make([]*PreSpan, 0)
	a.mapService = make(map[string]*PostSpan, 0)

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
// 消耗一条 Flow 数据，针对 l7_flow 和 l34_flow 不同类型进行分发
func (tm *TracerManager) ConsumeFlow(flow *observerpb.Flow) {
	if config.Debug {
		config.LoggerRawL7Flow.Debug(flow)
	}

	if flow.GetType() == observerpb.FlowType_L7 {
		if flow.L7.GetHttp() != nil {
			tm.consumeHttp(flow)
		}
	} else if flow.GetType() == observerpb.FlowType_L3_L4 {
		tm.consumeTrace(flow)
	}

}

func (tm *TracerManager) consumeHttp(flow *observerpb.Flow) {
	tm.wgConsumeHttp.Add(1)
	go func() {
		defer tm.wgConsumeHttp.Done()

		_, err := tm.BuildPreSpan(flow)
		if err != nil {
			tm.errConsumeHttp = errors.Join(tm.errConsumeHttp, err)
		}
	}()
}

func (tm *TracerManager) consumeTrace(flow *observerpb.Flow) {
	// 异步处理，不需要 WG 同步
	go func() {
		_, err := tm.BuildL34Flow(flow)
		if err != nil {
			tm.errConsumeL34 = errors.Join(tm.errConsumeL34, err)
		}
	}()
}

func (tm *TracerManager) waitForAssemble() {
	tm.wgConsumeHttp.Wait()

	if tm.errConsumeHttp != nil {
		logrus.Error(tm.errConsumeHttp)
	}

}

func (tm *TracerManager) Assemble() {
	// todo: 设计聚集的触发。目前是接收到全体 flow，并对全体 trace 聚合。
	tm.waitForAssemble()

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
