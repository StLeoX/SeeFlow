package tracer

import (
	"context"
	"github.com/stleox/seeflow/pkg/config"
	r "github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestTracer_BasicAssemble_2(t *testing.T) {
	// 测试 BasicAssemble，首先要确定 Debug 开启
	r.Equal(t, true, config.Debug)

	// 服务拓补图：foo <-> bar <-> loo
	// 输入：依次进入 <bar, loo>, <foo, bar>

	a := mockNewTracer()
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid2, "bar", "loo", time.Unix(3, 0), time.Unix(7, 0)))
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid1, "foo", "bar", time.Unix(1, 0), time.Unix(10, 0)))

	err := a.BasicAssemble(context.Background())
	r.NoError(t, err)

	r.Equal(t, 2, len(a.debMapTraceID))
	r.NotEqual(t, a.debMapTraceID["foo-bar"], a.debMapTraceID["bar-loo"])

	r.Equal(t, 2, len(a.debMapSpanID))
	r.NotEqual(t, a.debMapSpanID["foo-bar"], a.debMapSpanID["bar-loo"])

	r.Equal(t, a.debMapSpanID["foo-bar"], a.debMapParent["bar-loo"])

}

func TestTracer_BasicAssemble_3_1(t *testing.T) {
	r.Equal(t, true, config.Debug)

	// 服务拓补图：foo <-> bar <-> loo
	//                       <-> baz
	// 输入：依次进入 <bar, baz>, <bar, loo>, <foo, bar>

	a := mockNewTracer()
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid3, "bar", "baz", time.Unix(4, 0), time.Unix(5, 0)))
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid2, "bar", "loo", time.Unix(3, 0), time.Unix(7, 0)))
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid1, "foo", "bar", time.Unix(1, 0), time.Unix(10, 0)))

	err := a.BasicAssemble(context.Background())
	r.NoError(t, err)
	r.Equal(t, 3, len(a.debMapSpanID))
	r.Equal(t, a.debMapSpanID["foo-bar"], a.debMapParent["bar-loo"])
	r.Equal(t, a.debMapSpanID["foo-bar"], a.debMapParent["bar-baz"])

}

func TestTracer_BasicAssemble_3_2(t *testing.T) {
	r.Equal(t, true, config.Debug)

	// 服务拓补图：foo <-> bar <-> loo <-> baz
	// 输入：依次进入 <bar, baz>, <bar, loo>, <foo, bar>

	a := mockNewTracer()
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid3, "loo", "baz", time.Unix(8, 0), time.Unix(9, 0)))
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid2, "bar", "loo", time.Unix(3, 0), time.Unix(7, 0)))
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid1, "foo", "bar", time.Unix(1, 0), time.Unix(10, 0)))

	err := a.BasicAssemble(context.Background())
	r.NoError(t, err)
	r.Equal(t, 3, len(a.debMapSpanID))
	r.Equal(t, a.debMapSpanID["foo-bar"], a.debMapParent["bar-loo"])
	r.Equal(t, a.debMapSpanID["bar-loo"], a.debMapParent["loo-baz"])

}

func TestTracer_BasicAssemble_4_1(t *testing.T) {
	r.Equal(t, true, config.Debug)

	// 验证 parentCtx 确实起作用了，类似于 3_1 但是之上再有一个 span

	a := mockNewTracer()
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid3, "C", "E", time.Unix(8, 0), time.Unix(9, 0)))
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid2, "C", "D", time.Unix(3, 0), time.Unix(7, 0)))
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid1, "B", "C", time.Unix(1, 0), time.Unix(10, 0)))
	a.bufPreSpan = append(a.bufPreSpan, mockPreSpan(uuid4, "A", "B", time.Unix(0, 0), time.Unix(11, 0)))

	err := a.BasicAssemble(context.Background())
	r.NoError(t, err)
	r.Equal(t, 4, len(a.debMapSpanID))
	r.Equal(t, a.debMapSpanID["A-B"], a.debMapParent["B-C"])
	r.Equal(t, a.debMapSpanID["B-C"], a.debMapParent["C-D"])
	r.Equal(t, a.debMapSpanID["B-C"], a.debMapParent["C-E"])

}
