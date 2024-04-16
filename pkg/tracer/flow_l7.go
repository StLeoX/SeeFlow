package tracer

import (
	"fmt"
	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/sirupsen/logrus"
	"github.com/stleox/seeflow/pkg/config"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"time"
)

type L7FlowEntity struct {
	ID      string `db:"id"`       // UUID32 格式的 SpanID
	TraceID string `db:"trace_id"` // UUID16、X-B3-Traceid 格式的 TraceID

	SrcIdentity uint32 `db:"src_identity"` // identity 是对服务组的编址
	SrcPod      string `db:"src_pod"`      // pod_name 是对 pod 的编址，类似的有 endpoint

	DestIdentity uint32 `db:"dest_identity"`
	DestPod      string `db:"dest_pod"`

	StartTime time.Time `db:"start_time"` // 请求发出时间
	EndTime   time.Time `db:"end_time"`   // 响应发出时间（不是响应被接收的时间）

	// 其他字段
	// http.method
	// http.url
	// http.status_code

}

// L7Flow 别名 Span、PreSpan。
type L7Flow struct {
	L7FlowEntity
	tm *TracerManager
}

func (l *L7Flow) Check(flow *flowpb.Flow) error {
	err := checkSrcDest(flow)
	if err != nil {
		return err
	}
	if flow.L7.GetHttp() == nil {
		return fmt.Errorf("flow#%s missed HTTP field for L7 flow", flow.Uuid)
	}
	return nil
}

func (l *L7Flow) Build(flow *observerpb.Flow) error {
	xreqID, err := extractXreqID(flow)
	if err != nil {
		return err
	}

	// 检查缓存
	l7FlowLRU := l.tm.bufFlow
	hitFlow, hit := l7FlowLRU.Get(xreqID)
	if hit {
		// 命中，构造 span
		spanReq, spanResp := flow, hitFlow
		if flow.IsReply.Value {
			spanReq, spanResp = hitFlow, flow
		}

		traceID, err := extractTraceID(spanReq)
		if err != nil {
			return err
		}

		l.L7FlowEntity = L7FlowEntity{
			ID:           xreqID,
			TraceID:      traceID,
			SrcIdentity:  spanReq.Source.Identity,
			SrcPod:       extractPodName(spanReq.Source),
			DestIdentity: spanReq.Destination.Identity,
			DestPod:      extractPodName(spanReq.Destination),
			StartTime:    spanReq.Time.AsTime(),
			// fixme: EndTime 是否涉及到 latencyNs 字段
			EndTime: spanResp.Time.AsTime(),
		}

		// 命中后清除
		l7FlowLRU.Remove(xreqID)

		return nil
	}

	// 缺失匹配项，放入缓存
	l7FlowLRU.Add(xreqID, flow)

	// 缓存未满，不淘汰，返回 L7 是空的
	if l7FlowLRU.Len() < config.MaxNumFlow {
		return nil
	}

	// 缓存满，淘汰最旧项，并为其构造 BrokenSpan
	_, evict, _ := l7FlowLRU.RemoveOldest()
	if evict.IsReply.Value {
		// 从单条响应构建
		l.L7FlowEntity = L7FlowEntity{
			ID:           flow.Uuid,
			TraceID:      "", // 注意：空 TraceID 的记录不该写入数据库
			SrcIdentity:  config.IdentityWorld,
			SrcPod:       config.NameWorld,
			DestIdentity: flow.Destination.Identity,
			DestPod:      extractPodName(flow.Destination),
			StartTime:    config.MinSpanTimestamp, // 如果是响应，其请求时间是 MinSpanTimestamp
			EndTime:      flow.Time.AsTime(),
		}
	} else {
		// 从单条请求构建
		traceID, err := extractTraceID(flow)
		if err != nil {
			return err
		}

		l.L7FlowEntity = L7FlowEntity{
			ID:           flow.Uuid,
			TraceID:      traceID,
			SrcIdentity:  flow.Source.Identity,
			SrcPod:       extractPodName(flow.Source),
			DestIdentity: config.IdentityWorld,
			DestPod:      config.NameWorld,
			StartTime:    flow.Time.AsTime(),
			EndTime:      config.MaxSpanTimestamp, // 如果是请求，其响应时间是 MaxSpanTimestamp
		}
	}
	return nil
}

func (l *L7Flow) Insert() error {
	o := l.tm.olap
	if o == nil {
		return nil
	}

	err := o.l7Inserter.Insert(
		l.ID,
		l.TraceID,
		l.SrcIdentity,
		l.SrcPod,
		l.DestIdentity,
		l.DestPod,
		l.StartTime,
		l.EndTime)
	if err != nil {
		logrus.WithError(err).Warn("SeeFlow couldn't insert into t_L7")
		return err
	}
	return nil
}

func (l *L7Flow) MarkExFlow(exFlow ExFlow) {
	o := l.tm.olap
	if o == nil {
		return
	}

	o.muExL7.Lock()
	o.arrExL7 = append(o.arrExL7, exFlow)
	o.muExL7.Unlock()

}

func (l *L7Flow) Consume(flow *flowpb.Flow) {
	// 异步处理，且需要 WG 同步
	l.tm.wgL7Consume.Add(1)
	go func() {
		defer l.tm.wgL7Consume.Done()

		var reason int
		var err error
		// 首先检查
		err = l.Check(flow)
		if err != nil {
			reason = kExL7Broken
			goto ret
		}

		// 然后构建
		err = l.Build(flow)
		if err != nil {
			reason = kExL7Broken
			goto ret
		}

		// 最后插入
		err = l.Insert()
		if err != nil {
			reason = kExL7NotInserted
			goto ret
		}

	ret:
		l.MarkExFlow(ExFlow{
			reason: reason,
			errMsg: err.Error(),
			flow:   flow,
		})

	}()

}

// DB

func CreateL7Table(db sqlx.SqlConn) error {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS `t_L7` " +
		"(id CHAR(36), " + // len(UUID32)
		"trace_id CHAR(16), " + // len(UUID16)
		"src_identity BIGINT, " +
		"dest_identity BIGINT, " +
		"start_time DATETIME(6), " +
		"end_time DATETIME(6)) " +
		"DISTRIBUTED BY HASH(src_identity, dest_identity) BUCKETS 32 " +
		"PROPERTIES (\"replication_num\" = \"1\");")
	return err
}

func NewL7Inserter(db sqlx.SqlConn) (*sqlx.BulkInserter, error) {
	return sqlx.NewBulkInserter(db, "INSERT INTO `t_L7` "+
		"(id, "+
		"trace_id, "+
		"src_identity, "+
		"dest_identity, "+
		"start_time, "+
		"end_time) "+
		"VALUES (?,?,?,?,?,?)")
}

// SelectL7Spans 选择某一 trace_id 下的全体 span
func (o *Olap) SelectL7Spans(spans *[]*L7FlowEntity, trace_id string) {
	err := o.conn.QueryRows(*spans, "SELECT "+
		"id, "+
		"trace_id, "+
		"src_identity, "+
		"'', "+
		"dest_identity, "+
		"'', "+
		"start_time, "+
		"end_time "+
		"FROM `t_L7` WHERE trace_id = ? "+
		"ORDER BY start_time", trace_id)
	if err != nil {
		logrus.WithError(err).Warn("SeeFlow couldn't select t_L7")
	}
}

// 目前允许的 filterKey: "namespace"
func (o *Olap) countL7Spans(filterKey string, filterValue string) int {
	if filterKey != "namespace" &&
		filterKey != "trace_id" {
		return -1
	}
	var count int
	err := o.conn.QueryRow(&count, fmt.Sprintf("SELECT COUNT(*) FROM `t_L7` WHERE "+
		"%s = ?", filterKey), filterValue)
	if err != nil {
		logrus.WithError(err).Warn("SeeFlow couldn't select t_L7")
	}
	return count
}

// CheckSpansCount 检查某一 namespace 下的 span 是否增加了
// true 代表增加了
func (o *Olap) CheckSpansCount(namespace string) bool {
	return o.countL7Spans("namespace", namespace) != o.mapSpanCount[namespace]
}
