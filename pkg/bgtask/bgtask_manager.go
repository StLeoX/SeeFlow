package tracer

import (
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/stleox/seeflow/pkg/tracer"
)

// BgTaskManager manages background periodical tasks.
// Includes:
// - Sync t_Ep table
// - Sync namespace list
// - Run Assemble algorithm
type BgTaskManager struct {
	bgTasks []BgTask
	hubble  observerpb.ObserverClient
	olap    *tracer.Olap
}

type BgTask interface {
	Start()
}

func NewBgTaskManager(hubble observerpb.ObserverClient, olap *tracer.Olap) *BgTaskManager {
	m := &BgTaskManager{
		bgTasks: make([]BgTask, 0),
		hubble:  hubble,
		olap:    olap,
	}
	m.addNamespaceTask()
	return m
}

func (m *BgTaskManager) StartAll() {
	for _, task := range m.bgTasks {
		task.Start()
	}
}
