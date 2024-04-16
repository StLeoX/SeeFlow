package tracer

import (
	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
)

// Flow operate
// 为三种 flow 抽象出父类：l34_flow、sock_flow、l7_flow(PreSpan)。
type Flow interface {
	// Check 预检查流量
	Check(flow *flowpb.Flow) error

	// Build 基于 manager 上下文构建字段
	Build(flow *observerpb.Flow) error

	// Insert 插入数据库
	Insert() error

	// MarkExFlow 标记异常流量
	MarkExFlow(exFlow ExFlow)

	// Consume 消费流量，not error but log
	Consume(flow *flowpb.Flow)
}

const (
	kExFlowUnknownProblem = iota
	kExL34Broken
	kExL34NotInserted
	kExL7Broken
	kExL7NotInserted
	kExSockBroken
	kExSockNotInserted
)

// ExFlow stands for Exceptional Flow
type ExFlow struct {
	reason int
	errMsg string
	flow   *observerpb.Flow
}
