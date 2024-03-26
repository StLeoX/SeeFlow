package tracer

import (
	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stleox/seeflow/pkg/config"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"

	r "github.com/stretchr/testify/require"
)

func TestTracer_BuildPreSpan_1(t *testing.T) {
	// right span: sequential flows, f1 happens-before f2
	tm := mockNewTracerManager()

	f1 := mockFlow(uuid1, time.Unix(1, 0), false, "bar", "foo")
	span, err := tm.BuildPreSpan(f1)
	r.NoError(t, err)
	r.Nil(t, span)

	f2 := mockFlow(uuid1, time.Unix(10, 0), true, "foo", "bar")
	span, err = tm.BuildPreSpan(f2)
	r.NoError(t, err)
	r.NotNil(t, span)

	r.Equal(t, uuid1, span.ID)
	r.Equal(t, time.Unix(1, 0).UTC(), span.StartTime)
	r.Equal(t, time.Unix(10, 0).UTC(), span.EndTime)

}

func TestTracer_BuildPreSpan_2(t *testing.T) {
	// right span: disordered flows, f2 happens-before f1
	tm := mockNewTracerManager()

	f1 := mockFlow(uuid2, time.Unix(10, 0), true, "foo", "bar")
	span, err := tm.BuildPreSpan(f1)
	r.NoError(t, err)
	r.Nil(t, span)

	f2 := mockFlow(uuid2, time.Unix(1, 0), false, "bar", "foo")
	span, err = tm.BuildPreSpan(f2)
	r.NoError(t, err)
	r.NotNil(t, span)

	r.Equal(t, uuid2, span.ID)
	r.Equal(t, time.Unix(1, 0).UTC(), span.StartTime)
	r.Equal(t, time.Unix(10, 0).UTC(), span.EndTime)

}

func TestTracer_BuildBrokenPreSpan_1(t *testing.T) {
	// broken span: f1 is request
	tm := mockNewTracerManager()

	f1 := mockFlow(uuid1, time.Unix(1, 0), false, "bar", "foo")
	span, err := tm.buildPreSpanForSingleRequest(f1)
	r.NoError(t, err)
	r.NotNil(t, span)

	r.Equal(t, time.Unix(1, 0).UTC(), span.StartTime)
	r.Equal(t, config.MaxSpanTimestamp.UTC(), span.EndTime)

}

func TestTracer_BuildBrokenPreSpan_2(t *testing.T) {
	// broken span: f1 is response
	tm := mockNewTracerManager()

	f1 := mockFlow(uuid1, time.Unix(1, 0), true, "bar", "foo")
	span, err := tm.buildPreSpanForSingleResponse(f1)
	r.NoError(t, err)
	r.NotNil(t, span)

	r.Equal(t, config.MinSpanTimestamp.UTC(), span.StartTime)
	r.Equal(t, time.Unix(1, 0).UTC(), span.EndTime)

}

//test utils

func TestTracer_convertSpanID(t *testing.T) {
	spanID, err := convertSpanID("00000000-0000-0000-0000-000000000001")
	r.NoError(t, err)
	r.Equal(t, true, spanID.IsValid())
	r.Equal(t, "0000000000000001", spanID.String())
}

func TestTracer_extractSvcName(t *testing.T) {
	type args struct {
		endpoint *flowpb.Endpoint
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"normal",
			args{endpoint: &flowpb.Endpoint{
				Labels: []string{"k8s:app=foo-svc", "k8s:io.kubernetes.pod.namespace=demo"}}},
			"foo-svc",
		},
		{
			"miss",
			args{endpoint: &flowpb.Endpoint{Labels: []string{""}}},
			config.NameUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractSvcName(tt.args.endpoint); got != tt.want {
				t.Errorf("extractSvcName() = %v, want %v", got, tt.want)
			}
		})
	}
}

//mockers

func mockNewTracer() *Tracer {
	tm := NewTracerManager(nil)
	tm.InitDummyExporter()
	return tm.addTracer(uuid1)
}

func mockNewTracerManager() *TracerManager {
	tm := NewTracerManager(nil)
	tm.InitDummyExporter()
	tm.addTracer(uuid1)
	return tm
}

// 创建测试用的 Flow 对象的辅助函数，暂时考虑参数中的几个要素。
// 因为是在测试 span 相关，所以 TraceID 固定，并且仅在请求中携带。
func mockFlow(flowXreqValue string, flowTime time.Time, flowIsReply bool, flowSrc string, flowDest string) *observerpb.Flow {
	const (
		podNameSuffix  = "-0000000000-00000"
		svcLabelPrefix = "k8s:app="
	)

	argHeaders := make([]*observerpb.HTTPHeader, 0)
	argHeaders = append(argHeaders, &observerpb.HTTPHeader{
		Key:   "X-Request-Id",
		Value: flowXreqValue,
	})
	argHeaders = append(argHeaders, &observerpb.HTTPHeader{
		Key:   "X-B3-Traceid",
		Value: uuid1,
	})

	argTime := &timestamppb.Timestamp{
		Seconds: flowTime.Unix(),
		Nanos:   0,
	}

	argL7Type := observerpb.L7FlowType_REQUEST
	if flowIsReply {
		argL7Type = observerpb.L7FlowType_RESPONSE
	}

	argSrc := &observerpb.Endpoint{
		Labels:  []string{svcLabelPrefix + flowSrc},
		PodName: flowSrc + podNameSuffix,
	}
	argDest := &observerpb.Endpoint{
		Labels:  []string{svcLabelPrefix + flowDest},
		PodName: flowDest + podNameSuffix,
	}

	return &observerpb.Flow{
		Time:        argTime,
		Source:      argSrc,
		Destination: argDest,
		L7: &observerpb.Layer7{
			Type:      argL7Type,
			LatencyNs: 0,
			Record: &observerpb.Layer7_Http{
				Http: &observerpb.HTTP{
					Headers: argHeaders,
				},
			},
		},
		IsReply: &wrappers.BoolValue{Value: flowIsReply},
	}
}

var mockPodName2Identity = make(map[string]uint32, 0)
var mockId = uint32(0)

func queryPodName2Identity(pod string) uint32 {
	if id, hit := mockPodName2Identity[pod]; hit {
		return id
	} else {
		id := mockId
		mockId++
		mockPodName2Identity[pod] = id
		return id
	}
}

func mockPreSpan(id string, src string, dest string, start time.Time, end time.Time) *PreSpan {
	const (
		podNameSuffix = "-0000000000-00000"
	)

	return &PreSpan{
		ID:           id,
		TraceID:      uuid1,
		SrcIdentity:  queryPodName2Identity(src),
		SrcPod:       src + podNameSuffix,
		DestIdentity: queryPodName2Identity(dest),
		DestPod:      dest + podNameSuffix,
		StartTime:    start,
		EndTime:      end,
	}
}

const (
	uuid1 = "00000000-0000-0000-0000-000000000001"
	uuid2 = "00000000-0000-0000-0000-000000000002"
	uuid3 = "00000000-0000-0000-0000-000000000003"
	uuid4 = "00000000-0000-0000-0000-000000000004"
)
