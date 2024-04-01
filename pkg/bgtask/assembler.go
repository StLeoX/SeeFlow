package bgtask

import (
	"github.com/stleox/seeflow/pkg/config"
	"github.com/stleox/seeflow/pkg/tracer"
	"github.com/zeromicro/go-zero/core/executors"
)

type Assembler struct {
	executor *executors.PeriodicalExecutor
	tasker   *assemblerTasker
}

func NewAssembler(olap *tracer.Olap) *Assembler {
	tasker := &assemblerTasker{
		olap: olap,
	}
	return &Assembler{
		executor: executors.NewPeriodicalExecutor(config.AssembleInterval, tasker),
		tasker:   tasker,
	}
}

type assemblerTasker struct {
	olap *tracer.Olap
}

func (a *assemblerTasker) AddTask(task any) bool {
	//TODO implement me
	panic("implement me")
}

func (a *assemblerTasker) Execute(tasks any) {
	// check a.olap.CheckSpansCount()
}

func (a *assemblerTasker) RemoveAll() any {
	//TODO implement me
	panic("implement me")
}

//func newAssemblerTasker() *assemblerTasker {
//	return &assemblerTasker{}
//}
