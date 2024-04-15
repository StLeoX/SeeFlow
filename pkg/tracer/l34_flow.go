package tracer

import (
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/sirupsen/logrus"
	"github.com/stleox/seeflow/pkg/config"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"sync/atomic"
	"time"
)

type L34Flow struct {
	Time      time.Time `db:"time"`      // 捕获时间。
	Namespace string    `db:"namespace"` // 流量相关名字空间，存在“或”逻辑。

	SrcIdentity  uint32 `db:"src_identity"`  // NumericIdentity 的数值类型为 uint32，且用 0 作为空值。
	DestIdentity uint32 `db:"dest_identity"` //

	IsReply            bool   `db:"is_reply"`            // 区分流量方向1。
	TrafficDirection   string `db:"traffic_direction"`   // 区分流量方向2，一般是这两种 "INGRESS" 或 "EGRESS"。
	TrafficObservation string `db:"traffic_observation"` // 捕获位置，@pkg/monitor/api/types.go:150
	Verdict            string `db:"verdict"`             // 一般是这两种 "FORWARDED" 或 "DROPPED"。

	EventType int8 `db:"event_type"` // 事件类型，@pkg/monitor/api/types.go:18
	SubType   int8 `db:"sub_type"`   // 事件子类型，@pkg/monitor/api/types.go:217

	// 其他属性
	// Protocol // L4 协议，包含 TCP、UDP、ICMP。
}

func (tm *TracerManager) BuildL34Flow(flow *observerpb.Flow) (*L34Flow, error) {
	// first check
	if err := checkSrcDest(flow); err != nil {
		return nil, err
	}

	// then build
	l34 := L34Flow{
		Time:               flow.Time.AsTime(),
		Namespace:          extractNamespace(flow),
		SrcIdentity:        flow.Source.Identity,
		DestIdentity:       flow.Destination.Identity,
		IsReply:            flow.IsReply.Value,
		TrafficDirection:   flow.TrafficDirection.String(),
		TrafficObservation: flow.TraceObservationPoint.String(),
		Verdict:            flow.Verdict.String(),
	}

	tm.olap.InsertL34Flow(&l34)

	return &l34, nil
}

// DB

func CreateL34Table(db sqlx.SqlConn) error {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS `t_L34` " +
		"(time DATETIME(6), " +
		"namespace VARCHAR(127), " +
		"src_identity BIGINT, " +
		"dest_identity BIGINT, " +
		"is_reply BOOLEAN, " +
		"traffic_direction VARCHAR(15), " +
		"traffic_observation VARCHAR(15), " +
		"verdict VARCHAR(15)) " +
		"DISTRIBUTED BY HASH(src_identity, dest_identity) BUCKETS 32 " +
		"PROPERTIES (\"replication_num\" = \"1\");")
	return err
}

func NewL34Inserter(db sqlx.SqlConn) (*sqlx.BulkInserter, error) {
	return sqlx.NewBulkInserter(db, "INSERT INTO `t_L34` "+
		"(time, "+
		"namespace, "+
		"src_identity, "+
		"dest_identity, "+
		"is_reply, "+
		"traffic_direction, "+
		"traffic_observation, "+
		"verdict) "+
		"VALUES (?,?,?,?,?,?,?,?)")
}

var numInsertedL34 atomic.Int32

func (o *Olap) InsertL34Flow(l34 *L34Flow) {
	if o == nil {
		return
	}
	err := o.l34Inserter.Insert(
		l34.Time.String()[:config.L_DATE6],
		l34.Namespace,
		l34.SrcIdentity,
		l34.DestIdentity,
		l34.IsReply,
		l34.TrafficDirection,
		l34.TrafficObservation,
		l34.Verdict)
	if err != nil {
		logrus.WithError(err).Warn("SeeFlow couldn't insert into t_L34")
	}

	numInsertedL34.Add(1)
}

func (o *Olap) SelectL34Flow() {
	// todo
}
