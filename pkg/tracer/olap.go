package tracer

import (
	observerpb "github.com/cilium/cilium/api/v1/observer"
	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stleox/seeflow/pkg/config"
	"github.com/stleox/seeflow/pkg/crd"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"sync"
	"time"
)

type Olap struct {
	conn        sqlx.SqlConn
	l34Inserter *sqlx.BulkInserter
	l7Inserter  *sqlx.BulkInserter

	// 异常 flow 列表，包含情况：broken flow、non-insert flow、
	// 目前认为异常概率小
	listEL34 []*observerpb.Flow
	muEL34   sync.Mutex
	listEL7  []*observerpb.Flow
	muEL7    sync.Mutex
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
		logrus.WithError(err).Error("SeeFlow couldn't create table t_L34")
		return nil
	} else {
		logrus.Info("SeeFlow created table t_L34")
	}

	l34Inserter, err := NewL34Inserter(db)
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't open table t_L34")
		return nil
	}

	// 新建 t_L7
	err = CreateL7Table(db)
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't create table t_L7")
		return nil
	} else {
		logrus.Info("SeeFlow created table t_L7")
	}

	l7Inserter, err := NewL7Inserter(db)
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't open table t_L7")
		return nil
	}

	// 新建 t_Ep
	err = crd.CreateEndpointTable(db)
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't create table t_Ep")
		return nil
	} else {
		logrus.Info("SeeFlow created table t_Ep")
	}

	return &Olap{
		conn:        db,
		l34Inserter: l34Inserter,
		l7Inserter:  l7Inserter,
	}
}

func (o *Olap) AddEL34(flow *observerpb.Flow) {
	o.muEL34.Lock()
	defer o.muEL34.Unlock()
	o.listEL34 = append(o.listEL34, flow)
}

func (o *Olap) AddEL7(flow *observerpb.Flow) {
	o.muEL7.Lock()
	defer o.muEL7.Unlock()
	o.listEL7 = append(o.listEL7, flow)
}

func (o *Olap) SummaryELs() {
	o.muEL34.Lock()
	defer o.muEL34.Unlock()

	o.muEL7.Lock()
	defer o.muEL7.Unlock()

	if len(o.listEL34) == 0 && len(o.listEL7) == 0 {
		logrus.Info("Seeflow not found exceptional flows")
	} else if len(o.listEL34) != 0 {
		logrus.Info("Seeflow not found exceptional l34 flows: ")
		// todo dump elog to file
		for el := range o.listEL34 {
			logrus.Infof("%v", el)
		}
	} else if len(o.listEL7) != 0 {
		logrus.Info("Seeflow not found exceptional l7 flows: ")
		for el := range o.listEL7 {
			logrus.Infof("%v", el)
		}
	}
}
