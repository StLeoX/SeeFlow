package crd

import (
	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// concept: CiliumEndpoint vs CiliumIdentity

type Endpoint struct {
	Namespace string `db:"namespace"`
	PodName   string `db:"pod_name"`
	SvcName   string `db:"svc_name"`
	Endpoint  uint32 `db:"endpoint"`
	Identity  string `db:"identity"`
	State     string `db:"state"`
	IP        string `db:"ip"` // now supported IPv4
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

func UpsertEndpoints(db sqlx.SqlConn, endpoints []*Endpoint) {
	inserter, err := sqlx.NewBulkInserter(db, "")
	if err != nil {
		logrus.Error("SeeFlow couldn't open table t_Ep")
	}
	defer inserter.Flush()

	for _, ep := range endpoints {
		err := inserter.Insert(ep.Namespace, ep.PodName, ep.SvcName, ep.Endpoint, ep.Identity, ep.State, ep.IP)
		if err != nil {
			logrus.Error("SeeFlow couldn't insert into t_Ep")
		}
	}
}
