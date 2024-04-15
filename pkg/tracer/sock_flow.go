package tracer

import (
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/sirupsen/logrus"
	"github.com/stleox/seeflow/pkg/config"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"sync/atomic"
	"time"
)

type SockFlow struct {
	Time      time.Time `db:"time"`      // 捕获时间。
	Namespace string    `db:"namespace"` // 流量相关名字空间，存在“或”逻辑。

	SrcIdentity  uint32 `db:"src_identity"`  // NumericIdentity 的数值类型为 uint32，且用 0 作为空值。
	DestIdentity uint32 `db:"dest_identity"` //

	EventType int8 `db:"event_type"` // 事件类型，@pkg/monitor/api/types.go:18
	SubType   int8 `db:"sub_type"`   // 事件子类型，@pkg/monitor/api/types.go:217
	CgroupId  int  `db:"cgroup_id"`  // todo 可能与 identity 作用重复，后面可能删掉

	// 其他属性

}

func (tm *TracerManager) BuildSockFlow(flow *observerpb.Flow) (*SockFlow, error) {
	// first check
	if err := checkSrcDest(flow); err != nil {
		return nil, err
	}

	// then build
	sock := SockFlow{
		Time:      flow.Time.AsTime(),
		Namespace: extractNamespace(flow),

		SrcIdentity:  flow.Source.Identity,
		DestIdentity: flow.Destination.Identity,

		EventType: int8(flow.EventType.Type),
		SubType:   int8(flow.EventType.SubType),
		CgroupId:  int(flow.CgroupId),
	}

	tm.olap.InsertSockFlow(&sock)

	return &sock, nil

}

// DB

func CreateSockTable(db sqlx.SqlConn) error {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS `t_Sock` " +
		"(time DATETIME(6), " +
		"namespace VARCHAR(127), " +
		"src_identity BIGINT, " +
		"dest_identity BIGINT, " +
		"event_type TINYINT, " +
		"sub_type TINYINT, " +
		"cgroup_id INT) " +
		"DISTRIBUTED BY HASH(src_identity, dest_identity) BUCKETS 32 " +
		"PROPERTIES (\"replication_num\" = \"1\");")
	return err
}

func NewSockInserter(db sqlx.SqlConn) (*sqlx.BulkInserter, error) {
	return sqlx.NewBulkInserter(db, "INSERT INTO `t_Sock` "+
		"(time, "+
		"namespace, "+
		"src_identity, "+
		"dest_identity, "+
		"event_type, "+
		"sub_type, "+
		"cgroup_id) "+
		"VALUES (?,?,?,?,?,?,?)")
}

var numInsertedSock atomic.Int32

func (o *Olap) InsertSockFlow(sock *SockFlow) {
	if o == nil {
		return
	}
	err := o.sockInserter.Insert(
		sock.Time.String()[:config.L_DATE6],
		sock.Namespace,
		sock.SrcIdentity,
		sock.DestIdentity,
		sock.EventType,
		sock.SubType,
		sock.CgroupId)
	if err != nil {
		logrus.WithError(err).Warn("SeeFlow couldn't insert into t_Sock")
	}

	numInsertedSock.Add(1)
}
