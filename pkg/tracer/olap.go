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

func CreateL7Table(db sqlx.SqlConn) {
	// fixme 针对 Doris 的建表语句要修改。
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS `t_L7` " +
		"(id VARCHAR(36), " + // UUID32
		"trace_id VARCHAR(16), " + // UUID16
		"src_pod STRING, " +
		"dest_pod STRING, " +
		"start_time DATETIME(6), " +
		"end_time DATETIME(6)) " +
		"DISTRIBUTED BY HASH(id) BUCKETS 32 " +
		"PROPERTIES (\"replication_num\" = \"1\");")
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't create Table t_L7")
	}
}

func (o *Olap) InsertL7Span(span *PreSpan) {
	// fixme flush 的用法是否正确？
	defer o.l7Inserter.Flush()
	err := o.l7Inserter.Insert(span.ID,
		span.TraceID,
		span.SrcPod,
		span.DestPod,
		span.StartTime.String()[:config.L_DATE6],
		span.EndTime.String()[:config.L_DATE6])
	if err != nil {
		logrus.WithError(err).WithField("span", *span).Warn("SeeFlow couldn't insert L7 span")
	}
}

func (o *Olap) SelectL7Spans(buf *[]*PreSpan) {
	err := o.conn.QueryRows(buf, "SELECT id, trace_id, src_pod, '', dest_pod, '', start_time, end_time FROM `t_L7` ORDER BY start_time")
	if err != nil {
		logrus.WithError(err).Error("SeeFlow couldn't select L7 spans")
	}
}
