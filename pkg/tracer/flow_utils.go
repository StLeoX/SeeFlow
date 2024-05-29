package tracer

import (
	"fmt"
	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/stleox/seeflow/pkg/config"
	tr "go.opentelemetry.io/otel/trace"
	"strings"
)

// TraceID 选用首个非空字段，优先级顺序：`x-client-trace-ID`(128 bits) > `X-B3-Traceid`(64 bits)
// 数据可能缺少 TraceID 字段，返回空
func extractTraceID(flow *observerpb.Flow) (string, error) {
	headers := flow.L7.GetHttp().Headers
	for i := 0; i < len(headers); i++ {
		if strings.EqualFold(headers[i].Key, "x-client-trace-ID") ||
			strings.EqualFold(headers[i].Key, "x-b3-traceid") {
			return headers[i].Value, nil
		}
	}
	return "", fmt.Errorf("flow#%s doesn't have TraceID in HTTP headers", flow.Uuid)
}

// 返回空 ID，比如在 WG 中作为键，并且不检查
func extractTraceIDOrZero(flow *observerpb.Flow) (string, error) {
	traceID, err := extractTraceID(flow)
	if traceID == "" {
		return tr.TraceID{}.String(), nil
	}
	return traceID, err
}

// convert to UUID32
// demo input: "000000000000000a", usually extractTraceID's output.
// demo output: "0000000000004000800000000000000a", `zero` if fail to convert.
// https://stackoverflow.com/questions/7905929/how-to-test-valid-uuid-guid
func convertTraceID(uuid string) tr.TraceID {
	if len(uuid) == 16 {
		validMiddle := "0000400080000000"
		uuid = uuid[:8] + validMiddle + uuid[8:]
	}
	traceID, err := tr.TraceIDFromHex(uuid)
	if err != nil {
		return tr.TraceID{}
	}
	return traceID
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

// 析取 Namespace 字段，存在“或”逻辑
func extractNamespace(flow *observerpb.Flow) string {
	if flow == nil {
		return config.NameUnknown
	}
	if flow.Source.Namespace != "" {
		return flow.Source.Namespace
	}
	if flow.Destination.Namespace != "" {
		return flow.Destination.Namespace
	}
	return config.NameUnknown
}

// convert from PodName to SvcName
// 仅限于测试
func convertPodName(podName string) string {
	return strings.Split(podName, "-")[0]
}

// SpanName = {{SrcSvc}}-{{DestSvc}}
func constructSpanName(l7 *L7FlowEntity) string {
	return fmt.Sprintf("%s-%s", convertPodName(l7.SrcPod), convertPodName(l7.DestPod))
}

func checkSrcDest(flow *flowpb.Flow) error {
	if flow.Source == nil ||
		flow.Destination == nil {
		return fmt.Errorf("flow#%s doesn't have Source or Destination", flow.Uuid)
	}
	return nil
}
