package bgtask

import (
	"context"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/sirupsen/logrus"

	"sync"
)

// （通过内存）维持 Cilium 中的 namespace 列表
type NamespaceWatcher struct {
	arrNamespace []string
	muNamespace  sync.Mutex
	kRequest     *observerpb.GetNamespacesRequest
}

func NewNamespaceWatcher() *NamespaceWatcher {
	return &NamespaceWatcher{
		arrNamespace: make([]string, 0),
		kRequest:     &observerpb.GetNamespacesRequest{},
	}
}

// 全量更新列表
func (w *NamespaceWatcher) FetchNamespaces(hubble observerpb.ObserverClient) {
	resp, err := hubble.GetNamespaces(context.Background(), w.kRequest)
	if err != nil {
		logrus.WithError(err).Errorf("SeeFlow couldn't fetch Hubble's namespaces")
		return
	}

	w.muNamespace.Lock()
	w.arrNamespace = make([]string, 0)
	for _, namespace := range resp.Namespaces {
		w.arrNamespace = append(w.arrNamespace, namespace.String())
	}
	w.muNamespace.Unlock()

}

// 级联删除：
// - 删除 olap 中某一 namespace 相关的 span（不活跃）
// - 删除 o.mapSpanCount 中的记录
func (w *NamespaceWatcher) DeleteNamespaceRelatedSpans(namespace string) {
	// todo
}
