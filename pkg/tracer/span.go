package tracer

import (
	"fmt"
	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/stleox/seeflow/pkg/config"
	tr "go.opentelemetry.io/otel/trace"
	"strings"
	"time"
)

type PreSpan struct {
	ID        string    `db:"id"`       // UUID32
	TraceID   string    `db:"trace_id"` // UUID16, currently from X-B3-Traceid
	SrcPod    string    `db:"src_pod"`
	SrcSvc    string    `db:"src_svc"`
	DestPod   string    `db:"dest_pod"`
	DestSvc   string    `db:"dest_svc"`
	StartTime time.Time `db:"start_time"`
	EndTime   time.Time `db:"end_time"`
	// 其他属性
	// http.method
	// http.url
	// http.status_code
}

// BuildPreSpan 构建 PreSpan
// 构建成功，入队并返回；构建失败，返回 nil
func (tm *TracerManager) BuildPreSpan(flow *observerpb.Flow) (*PreSpan, error) {
	xreqID, err := extractXreqID(flow)
	if err != nil {
		return nil, err
	}

	// 检查缓存
	hitFlow, hit := tm.bufFlow.Get(xreqID)
	if !hit {
		// 缺失，入表 flow
		tm.bufFlow.Add(xreqID, flow)
		// 缓存满，淘汰最旧项，并为其构造 BrokenSpan
		if tm.bufFlow.Len() == config.MaxNumFlow {
			_, evict, _ := tm.bufFlow.RemoveOldest()
			if evict.IsReply.Value {
				return tm.buildPreSpanForSingleResponse(evict)
			}
			return tm.buildPreSpanForSingleRequest(evict)
		}
		return nil, nil
	}
	// 命中，构造 span
	spanReq, spanResp := flow, hitFlow
	if flow.IsReply.Value {
		spanReq, spanResp = hitFlow, flow
	}

	traceID, err := extractTraceID(spanReq)
	if err != nil {
		return nil, err
	}

	retPreSpan := &PreSpan{
		ID:        xreqID,
		TraceID:   traceID,
		SrcPod:    extractPodName(spanReq.Source),
		SrcSvc:    extractSvcName(spanReq.Source),
		DestPod:   extractPodName(spanReq.Destination),
		DestSvc:   extractSvcName(spanReq.Destination),
		StartTime: spanReq.Time.AsTime(),
		// fixme: EndTime 是否涉及到 latencyNs 字段
		EndTime: spanResp.Time.AsTime(),
	}
	// 命中后清除
	tm.bufFlow.Remove(xreqID)

	// 找到相应的 Tracer
	a, hit := tm.tracers.Get(traceID)
	if !hit {
		a = tm.addTracer(traceID)
	}
	a.addPreSpan(retPreSpan)
	return retPreSpan, nil
}

// 针对溢出的 flow，我们构造其配对规则（注意，这会影响 tracing 算法）：
func (tm *TracerManager) buildPreSpanForSingleRequest(flow *observerpb.Flow) (*PreSpan, error) {
	traceID, err := extractTraceID(flow)
	if err != nil {
		return nil, err
	}
	retPreSpan := &PreSpan{
		ID:        flow.Uuid,
		TraceID:   traceID,
		SrcPod:    extractPodName(flow.Source),
		SrcSvc:    extractSvcName(flow.Source),
		DestPod:   config.NameWorld,
		DestSvc:   config.NameUnknown,
		StartTime: flow.Time.AsTime(),
		EndTime:   config.MaxSpanTimestamp, // 如果是请求，其响应时间是 MaxSpanTimestamp
	}
	// 对于 SingleRequest 可知其 TraceID，故入表。
	a, hit := tm.tracers.Get(traceID)
	if !hit {
		a = tm.addTracer(traceID)
	}
	a.addPreSpan(retPreSpan)
	return retPreSpan, nil
}

func (tm *TracerManager) buildPreSpanForSingleResponse(flow *observerpb.Flow) (*PreSpan, error) {
	retPreSpan := &PreSpan{
		ID:        flow.Uuid,
		TraceID:   "",
		SrcPod:    config.NameWorld,
		SrcSvc:    config.NameUnknown,
		DestPod:   extractPodName(flow.Destination),
		DestSvc:   extractSvcName(flow.Destination),
		StartTime: config.MinSpanTimestamp, // 如果是响应，其请求时间是 MinSpanTimestamp
		EndTime:   flow.Time.AsTime(),
	}
	// 对于 SingleResponse 不可知其 TraceID，故不入表。
	return retPreSpan, nil
}

// utils

// TraceID 选用首个非空字段，优先级顺序：`x-client-trace-ID`(128 bits) > `X-B3-Traceid`(64 bits)
// 测试数据中可能 response 没有 TraceID
func extractTraceID(flow *observerpb.Flow) (string, error) {
	headers := flow.L7.GetHttp().Headers
	for i := 0; i < len(headers); i++ {
		if strings.EqualFold(headers[i].Key, "x-client-trace-ID") ||
			strings.EqualFold(headers[i].Key, "x-b3-traceid") {
			return headers[i].Value, nil
		}
	}
	return "", fmt.Errorf("no field like TraceID in HTTP headers")
}

// convert to UUID32
// demo input: "000000000000000a", usually extractTraceID's output.
// demo output: "0000000000004000800000000000000a", zero if error.
// https://stackoverflow.com/questions/7905929/how-to-test-valid-uuid-guid
func convertTraceID(uuid string) (tr.TraceID, error) {
	if len(uuid) == 16 {
		validMiddle := "0000400080000000"
		uuid = uuid[:8] + validMiddle + uuid[8:]
	}
	return tr.TraceIDFromHex(uuid)
}

func extractXreqID(flow *observerpb.Flow) (string, error) {
	headers := flow.L7.GetHttp().Headers
	// 假设 X-Request-Id 字段是最后一个 Header，所以从后往前遍历
	for i := len(headers) - 1; i > -1; i-- {
		if strings.EqualFold(headers[i].Key, "x-request-ID") {
			return headers[i].Value, nil
		}
	}
	return "", fmt.Errorf("X-Request-Id not in HTTP headers")
}

// convert from UUID32 to UUID16
// demo input: "00000000-0000-0000-0000-00000000000a", usually extractXreqID's output.
// demo output: "000000000000000a", zero if error.
func convertSpanID(uuid string) (tr.SpanID, error) {
	if len(uuid) == 36 {
		uuid = uuid[:8] + uuid[28:]
	}
	return tr.SpanIDFromHex(uuid)
}

func extractPodName(endpoint *flowpb.Endpoint) string {
	if endpoint == nil {
		return config.NameUnknown
	}
	// convert "" to "unknown"
	if endpoint.PodName == "" {
		return config.NameUnknown
	}
	return endpoint.PodName
}

// 当然有多种方式析取 SvcName，包括：
// - 从 labels 通常是首个，以"k8s:app"开头；
// - 从 workloads[0].name，但是 workloads 本身是个数组；
// - 从 PodName 中析取，但是 Pod 名称规则又不确定；
func extractSvcName(endpoint *flowpb.Endpoint) string {
	if endpoint == nil {
		return config.NameUnknown
	}
	if len(endpoint.Labels) == 0 {
		return config.NameUnknown
	}
	for _, label := range endpoint.Labels {
		if strings.HasPrefix(label, "k8s:app") {
			return label[8:]
		}
	}
	return config.NameUnknown
}

func structureSpanName(preSpan *PreSpan) string {
	return preSpan.SrcSvc + "-" + preSpan.DestSvc
}
