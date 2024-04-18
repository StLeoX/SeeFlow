package tracer

import (
	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/sirupsen/logrus"
	"github.com/stleox/seeflow/pkg/config"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"time"
)

type SockFlowEntity struct {
	Time      time.Time `db:"time"`      // 捕获时间。
	Namespace string    `db:"namespace"` // 流量相关名字空间，存在“或”逻辑。

	SrcIdentity  uint32 `db:"src_identity"`  // NumericIdentity 的数值类型为 uint32，且用 0 作为空值。
	DestIdentity uint32 `db:"dest_identity"` //

	EventType int8 `db:"event_type"` // 事件类型，@pkg/monitor/api/types.go:18
	SubType   int8 `db:"sub_type"`   // 事件子类型，@pkg/monitor/api/types.go:217
	CgroupId  int  `db:"cgroup_id"`  // todo 可能与 identity 作用重复，后面可能删掉

	// 其他属性

}

type SockFlow struct {
	SockFlowEntity
	tm *TracerManager
}

func (s *SockFlow) Check(flow *flowpb.Flow) error {
	return checkSrcDest(flow)
}

func (s *SockFlow) Build(flow *observerpb.Flow) error {
	s.SockFlowEntity = SockFlowEntity{
		Time:      flow.Time.AsTime(),
		Namespace: extractNamespace(flow),

		SrcIdentity:  flow.Source.Identity,
		DestIdentity: flow.Destination.Identity,

		EventType: int8(flow.EventType.Type),
		SubType:   int8(flow.EventType.SubType),
		CgroupId:  int(flow.CgroupId),
	}
	return nil
}

func (s *SockFlow) Insert() error {
	o := s.tm.olap
	if o == nil {
		return nil
	}
	err := o.sockInserter.Insert(
		s.Time.String()[:config.L_DATE6],
		s.Namespace,
		s.SrcIdentity,
		s.DestIdentity,
		s.EventType,
		s.SubType,
		s.CgroupId)
	if err != nil {
		logrus.WithError(err).Warn("SeeFlow couldn't insert into t_Sock")
		return err
	}
	return nil
}

func (s *SockFlow) MarkExFlow(exFlow ExFlow) {
	o := s.tm.olap
	if o == nil {
		return
	}

	o.muExSock.Lock()
	o.arrExSock = append(o.arrExSock, exFlow)
	o.muExSock.Unlock()

}

func (s *SockFlow) Consume(flow *flowpb.Flow) {
	// 异步处理，不需要 WG 同步
	go func() {
		var reason int
		var err error
		// 首先检查
		err = s.Check(flow)
		if err != nil {
			reason = kExSockBroken
			goto eReturn
		}

		// 然后构建
		err = s.Build(flow)
		if err != nil {
			reason = kExSockBroken
			goto eReturn
		}

		// 最后插入
		err = s.Insert()
		if err != nil {
			reason = kExSockNotInserted
			goto eReturn
		}

		return

	eReturn:
		s.MarkExFlow(ExFlow{
			reason: reason,
			errMsg: err.Error(),
			flow:   flow,
		})

	}()
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
