package tracer

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
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
	arrExL34  []ExFlow
	muExL34   sync.Mutex
	arrExL7   []ExFlow
	muExL7    sync.Mutex
	arrExSock []ExFlow
	muExSock  sync.Mutex

	// 活跃 namespace 列表，通过 Hubble 更新
	ActiveNamespaces []string
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
	err = CreateEndpointTable(db)
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

func (o *Olap) Conn() sqlx.SqlConn {
	return o.conn
}

func (o *Olap) SummaryExFlows() {
	o.muExL34.Lock()
	o.muExL7.Lock()
	o.muExSock.Lock()

	if len(o.arrExL34) == 0 && len(o.arrExL7) == 0 && len(o.arrExSock) == 0 {
		logrus.Info("Seeflow didn't find exceptional flows")
		o.muExL34.Unlock()
		o.muExL7.Unlock()
		o.muExSock.Unlock()
		return
	}

	if len(o.arrExL34) != 0 {
		logrus.Infof("Seeflow found exceptional l34 flows, goto %s", config.PathExL34)
		for _, ef := range o.arrExL34 {
			config.Log4ExL34.Info(ef)
		}
	}
	o.muExL34.Unlock()

	if len(o.arrExL7) != 0 {
		logrus.Infof("Seeflow found exceptional l7 flows, goto %s", config.PathExL7)
		for _, ef := range o.arrExL7 {
			config.Log4ExL7.Info(ef)
		}
	}
	o.muExL7.Unlock()

	if len(o.arrExSock) != 0 {
		logrus.Infof("Seeflow found exceptional sock flows, goto %s", config.PathExSock)
		for _, ef := range o.arrExSock {
			config.Log4ExSock.Info(ef)
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

func CreateEndpointTable(db sqlx.SqlConn) error {
	_, err := db.Exec(
		"CREATE TABLE IF NOT EXISTS `t_Ep` " +
			"(namespace VARCHAR(127), " +
			"pod_name VARCHAR(127), " +
			"svc_name VARCHAR(127), " +
			"endpoint BIGINT, " +
			"identity VARCHAR(127), " +
			"state VARCHAR(15), " +
			"ip VARCHAR(15)) " +
			"DISTRIBUTED BY HASH(endpoint) BUCKETS 32 " +
			"PROPERTIES (\"replication_num\" = \"1\");")
	return err
}
