package tracer

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stleox/seeflow/pkg/config"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Olap struct {
	conn       sqlx.SqlConn
	l7Inserter *sqlx.BulkInserter
}

func NewOlap(vp *viper.Viper) *Olap {
	// conn to the OLAP server
	olapDSN := vp.GetString("SEEFLOW_OLAP_DSN")
	if olapDSN == "" {
		olapDSN = config.SEEFLOW_DEFAULT_DSN
	}

	db := sqlx.NewMysql(olapDSN)

	CreateL7Table(db)

	l7Inserter, err := sqlx.NewBulkInserter(db, "INSERT INTO `t_L7` (id, trace_id, src_pod, dest_pod, start_time, end_time) VALUES (?,?,?,?,?,?)")
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't open Table t_L7")
		return nil
	}

	return &Olap{
		conn:       db,
		l7Inserter: l7Inserter,
	}
}
