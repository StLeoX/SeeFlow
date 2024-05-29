package tracer

import (
	"context"
	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/cilium/pkg/hive/cell"
	"github.com/cilium/cilium/pkg/k8s/client"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// 通过数据库 t_Ep 小表，维持 Cilium 中的 endpoint 列表
type Endpoint struct {
	Namespace string `db:"namespace"`
	PodName   string `db:"pod_name"`
	SvcName   string `db:"svc_name"`
	Endpoint  uint32 `db:"endpoint"`
	Identity  string `db:"identity"`
	State     string `db:"state"`
	IP        string `db:"ip"` // now supported IPv4
}

// 全量更新，先清表再插入
func insertEndpoints(db sqlx.SqlConn, endpoints []*Endpoint) {
	inserter, err := sqlx.NewBulkInserter(db, "")
	if err != nil {
		logrus.Error("SeeFlow couldn't open t_Ep")
	}
	defer inserter.Flush()

	_, err = db.Exec("TRUNCATE TABLE `t_Ep`")
	if err != nil {
		logrus.Error("SeeFlow couldn't truncate t_Ep")
	}

	for _, ep := range endpoints {
		err = inserter.Insert(ep.Namespace, ep.PodName, ep.SvcName, ep.Endpoint, ep.Identity, ep.State, ep.IP)
		if err != nil {
			logrus.Error("SeeFlow couldn't insert into t_Ep")
		}
	}
}

type EndpointTask struct {
	m *BgTaskManager
}

func (m *BgTaskManager) addEndpointTask() {
	m.bgTasks = append(m.bgTasks, &EndpointTask{
		m: m,
	})
}

func (t *EndpointTask) Run() {
	endpoints := make([]*Endpoint, 0)
	fetchEndpoints(endpoints)
	insertEndpoints(t.m.olap.Conn(), nil)
}

func fetchEndpoints(endpoints []*Endpoint) {
	// 获取 endpoint CRD 参考 github.com/cilium/cilium@v1.15.2/pkg/k8s/client/client_test.go:268
	var myClientset client.Clientset
	myHive := hive.New(
		client.Cell,
		cell.Invoke(func(c client.Clientset) { myClientset = c }),
	)
	ctx := context.Background()
	err := myHive.Start(ctx)
	if err != nil {
		logrus.Error("SeeFlow couldn't start myHive")
		return
	}
	// fixme: CiliumV2 is nil
	_, err = myClientset.CiliumV2().CiliumEndpoints("kube-system").Get(ctx, "cep", metav1.GetOptions{})
	if err != nil {
		logrus.Error("SeeFlow couldn't get endpoints")
		return
	}
}

func (t *EndpointTask) Start() {
	// olap 初始化时已建表

	c := cron.New()
	_, err := c.AddJob("@every 1s", t)
	if err != nil {
		logrus.Warn("SeeFlow couldn't add endpoint task")
		return
	}
	c.Start()
}
