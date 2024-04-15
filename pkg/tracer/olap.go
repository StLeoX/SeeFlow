package tracer

import (
	observerpb "github.com/cilium/cilium/api/v1/observer"
	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stleox/seeflow/pkg/bgtask"
	"github.com/stleox/seeflow/pkg/config"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"sync"
	"time"
)

type Olap struct {
	conn         sqlx.SqlConn
	l34Inserter  *sqlx.BulkInserter
	l7Inserter   *sqlx.BulkInserter
	sockInserter *sqlx.BulkInserter

	// 维持各个 namespace 下的 span 数量
	mapSpanCount map[string]int
	muSpanCount  sync.Mutex

	// 异常 flow 列表，包含情况：broken flow、non-insert flow、
	// 目前认为异常概率小
	// todo: 改成一个 map: const_string -> array
	arrEL34   []*observerpb.Flow
	muEL34    sync.Mutex
	arrEL7    []*observerpb.Flow
	muEL7     sync.Mutex
	arrELSock []*observerpb.Flow
	muELSock  sync.Mutex
}

func NewOlap(vp *viper.Viper) *Olap {
	// conn to the OLAP server
	olapDSN := vp.GetString("SEEFLOW_OLAP_DSN")
	if olapDSN == "" {
		olapDSN = config.SEEFLOW_DEFAULT_DSN
	}

	// 新建 OLAP 实例
	db := sqlx.NewMysql(olapDSN)
	// 关闭 SQL 普通日志
	sqlx.DisableStmtLog()
	// 开启 SQL 慢查询日志，插入时延控制在 500ms 以内。
	sqlx.SetSlowThreshold(500 * time.Millisecond)

	// 新建 t_L34
	err := CreateL34Table(db)
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't create t_L34")
		return nil
	} else {
		logrus.Info("SeeFlow created table t_L34")
	}

	l34Inserter, err := NewL34Inserter(db)
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't open t_L34")
		return nil
	}

	// 新建 t_L7
	err = CreateL7Table(db)
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't create t_L7")
		return nil
	} else {
		logrus.Info("SeeFlow created table t_L7")
	}

	l7Inserter, err := NewL7Inserter(db)
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't open t_L7")
		return nil
	}

	// 新建 t_Sock
	err = CreateSockTable(db)
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't create t_Sock")
		return nil
	} else {
		logrus.Info("SeeFlow created table t_Sock")
	}

	sockInserter, err := NewSockInserter(db)
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't open t_Sock")
		return nil
	}

	// 新建 t_Ep
	err = bgtask.CreateEndpointTable(db)
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't create t_Ep")
		return nil
	} else {
		logrus.Info("SeeFlow created table t_Ep")
	}

	return &Olap{
		conn:         db,
		l34Inserter:  l34Inserter,
		l7Inserter:   l7Inserter,
		sockInserter: sockInserter,
		mapSpanCount: make(map[string]int, 0),
		arrEL34:      make([]*observerpb.Flow, 0),
		arrEL7:       make([]*observerpb.Flow, 0),
		arrELSock:    make([]*observerpb.Flow, 0),
	}
}

func (o *Olap) AddEL34(flow *observerpb.Flow) {
	o.muEL34.Lock()
	defer o.muEL34.Unlock()
	o.arrEL34 = append(o.arrEL34, flow)
}

func (o *Olap) AddEL7(flow *observerpb.Flow) {
	o.muEL7.Lock()
	defer o.muEL7.Unlock()
	o.arrEL7 = append(o.arrEL7, flow)
}

func (o *Olap) AddELSock(flow *observerpb.Flow) {
	// dummy
}

func (o *Olap) SummaryELs() {
	o.muEL34.Lock()
	defer o.muEL34.Unlock()

	o.muEL7.Lock()
	defer o.muEL7.Unlock()

	if len(o.arrEL34) == 0 && len(o.arrEL7) == 0 {
		logrus.Info("Seeflow not found exceptional flows")
	} else if len(o.arrEL34) != 0 {
		logrus.Info("Seeflow found exceptional l34 flows: ")
		// todo dump elog to file
		for el := range o.arrEL34 {
			logrus.Infof("%v", el)
		}
	} else if len(o.arrEL7) != 0 {
		logrus.Info("Seeflow found exceptional l7 flows: ")
		for el := range o.arrEL7 {
			logrus.Infof("%v", el)
		}
	}
}

func (o *Olap) GetSpanCount(namespace string) int {
	o.muSpanCount.Lock()
	defer o.muSpanCount.Unlock()
	return o.mapSpanCount[namespace]
}

func (o *Olap) RemoveSpanCount(namespace string) {
	o.muSpanCount.Lock()
	defer o.muSpanCount.Unlock()
	delete(o.mapSpanCount, namespace)
}
