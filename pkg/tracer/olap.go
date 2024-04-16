package tracer

import (
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

	// 异常流量列表，目前认为异常概率小
	// todo dump ExFlows to file
	arrExL34  []ExFlow
	muExL34   sync.Mutex
	arrExL7   []ExFlow
	muExL7    sync.Mutex
	arrExSock []ExFlow
	muExSock  sync.Mutex
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
	}
	logrus.Info("SeeFlow created table t_L34")

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
	}
	logrus.Info("SeeFlow created table t_L7")

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
	}
	logrus.Info("SeeFlow created table t_Sock")

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
	}
	logrus.Info("SeeFlow created table t_Ep")

	return &Olap{
		conn:         db,
		l34Inserter:  l34Inserter,
		l7Inserter:   l7Inserter,
		sockInserter: sockInserter,
		mapSpanCount: make(map[string]int, 0),
		arrExL34:     make([]ExFlow, 0),
		arrExL7:      make([]ExFlow, 0),
		arrExSock:    make([]ExFlow, 0),
	}
}

func (o *Olap) SummaryExFlows() {
	o.muExL34.Lock()
	o.muExL7.Lock()
	o.muExSock.Lock()

	if len(o.arrExL34) == 0 && len(o.arrExL7) == 0 && len(o.arrExSock) == 0 {
		logrus.Info("Seeflow not found exceptional flows")
		o.muExL34.Unlock()
		o.muExL7.Unlock()
		o.muExSock.Unlock()
		return
	}

	if len(o.arrExL34) != 0 {
		logrus.Info("Seeflow found exceptional l34 flows: ")
		for ef := range o.arrExL34 {
			logrus.Infof("%v", ef)
		}
	}
	o.muExL34.Unlock()

	if len(o.arrExL7) != 0 {
		logrus.Info("Seeflow found exceptional l7 flows: ")
		for ef := range o.arrExL7 {
			logrus.Infof("%v", ef)
		}
	}
	o.muExL7.Unlock()

	if len(o.arrExSock) != 0 {
		logrus.Info("Seeflow found exceptional sock flows: ")
		for ef := range o.arrExSock {
			logrus.Infof("%v", ef)
		}
	}
	o.muExSock.Unlock()

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
