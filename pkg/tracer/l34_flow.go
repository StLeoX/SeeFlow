package tracer

import (
	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"time"
)

type L34Flow struct {
	Time               time.Time `db:"time"`                // 捕获时间。
	Namespace          string    `db:"namespace"`           // 流量相关名字空间，存在“或”逻辑。
	SrcIdentity        uint32    `db:"src_identity"`        // NumericIdentity 的数值类型为 uint32，且用 0 作为空值。
	DestIdentity       uint32    `db:"dest_identity"`       //
	IsReply            bool      `db:"is_reply"`            // 区分流量方向1。
	TrafficDirection   string    `db:"traffic_direction"`   // 区分流量方向2，一般是这两种 "INGRESS" 或 "EGRESS"。
	TrafficObservation string    `db:"traffic_observation"` // 捕获位置。
	Verdict            string    `db:"verdict"`             // 一般是这两种 "FORWARDED" 或 "DROPPED"。
	// 其他属性
	// EventType // 需要研究枚举
	// Protocol // L4 协议，包含 TCP、UDP、ICMP。
}

func (tm *TracerManager) BuildL34Flow(flow *observerpb.Flow) (*L34Flow, error) {
	// deb 是 check 还是直接用，这是一个测试的问题。
	//// first check
	//if flow.Time == nil ||
	//	flow.Source == nil ||
	//	flow.Destination == nil ||
	//	flow.IsReply == nil {
	//	return nil, fmt.Errorf("broken l34_flow at %s", flow.Uuid)
	//}

	// build
	l34 := L34Flow{
		Time:               flow.Time.AsTime(),
		Namespace:          flow.Source.Namespace,
		SrcIdentity:        extractIdentity(flow.Source),
		DestIdentity:       extractIdentity(flow.Destination),
		IsReply:            flow.IsReply.Value,
		TrafficDirection:   flow.TrafficDirection.String(),
		TrafficObservation: flow.TraceObservationPoint.String(),
		Verdict:            flow.Verdict.String(),
	}

	tm.olap.InsertL34Flow(&l34)

	return &l34, nil
}

// Util
func extractIdentity(endpoint *flowpb.Endpoint) uint32 {
	if endpoint == nil {
		return 0
	}
	return 0
}

// DB

func CreateL34Table(db sqlx.SqlConn) {

}

func (o *Olap) InsertL34Flow(l34 *L34Flow) {
	// todo 日志表示 20 条全是 l34
	logrus.Info("insert l34 flow")
}

func (o *Olap) SelectL34Flow() {

}
