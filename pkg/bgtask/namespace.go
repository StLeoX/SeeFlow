package tracer

import (
	"context"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	"sync"
)

type NamespaceTask struct {
	m        *BgTaskManager
	muUpdate sync.Mutex
	kRequest *observerpb.GetNamespacesRequest
}

func (m *BgTaskManager) addNamespaceTask() {
	m.bgTasks = append(m.bgTasks, &NamespaceTask{
		m:        m,
		kRequest: &observerpb.GetNamespacesRequest{},
	})

}

// 全量更新
func (t *NamespaceTask) Run() {
	resp, err := t.m.hubble.GetNamespaces(context.Background(), t.kRequest)
	if err != nil {
		logrus.WithError(err).Errorf("SeeFlow couldn't fetch Hubble's namespaces")
		return
	}

	t.muUpdate.Lock()
	t.m.olap.ActiveNamespaces = make([]string, 0) // 重置
	for _, namespace := range resp.Namespaces {
		t.m.olap.ActiveNamespaces = append(t.m.olap.ActiveNamespaces, namespace.String())
	}
	t.muUpdate.Unlock()

}

func (t *NamespaceTask) Start() {
	c := cron.New()
	_, err := c.AddJob("@every 1s", t)
	if err != nil {
		logrus.Warn("SeeFlow couldn't add namespace task")
		return
	}
	c.Start()
}

// 级联删除
// - 删除 olap 中某一 namespace 相关的 span（不活跃）
// - 删除 o.mapSpanCount 中的记录
func (t *NamespaceTask) DeleteNamespaceRelatedSpans(namespace string) {
	// todo
}
