package config

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"time"
)

const (
	NameUnknown   = "unknown"
	NameWorld     = "world"
	IdentityWorld = 2
)

// for root
var (
	Debug           = false
	LoggerRawL7Flow *logrus.Logger
)

// for cmd observe
var (
	//请求 Hubble Flow 的数量
	BatchLastFlow = 50

	//插入 olap 的 Span 的数量
	BatchSpan = 50
)

// for cmd serve
var (
	// 请求 Hubble Flow 的时间间隔
	GetFlowsInterval = time.Second
	// 触发 Assemble 算法的时间间隔。
	// 与上个时间间隔最好保持一致。
	AssembleInterval = time.Second
)

// for pkg tracer
var (
	MaxNumFlow       = 1024
	MaxNumTracer     = 16
	MaxSpanTimestamp = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	MinSpanTimestamp = time.Unix(0, 0).UTC()
)

// for DB
var (
	// 测试账号
	SEEFLOW_DEFAULT_DSN = "root:@tcp(127.0.0.1:9030)/seeflow"

	// DATE6 = "2006-01-02 15:04:05.000000" 的长度
	L_DATE6 = 26
)

// initializes logrus
func initLogrus(_ *viper.Viper) {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:   true,
		TimestampFormat: time.DateTime,
	})
	if Debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}

func initRawL7FlowLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.DateTime,
		PrettyPrint:     true,
	})
	tmpLog, err := os.OpenFile("/tmp/seeflow_raw_l7flow.log.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	//defer tmpLog.Close()
	logger.SetOutput(tmpLog)
	return logger
}

func init() {
	initLogrus(nil)
	LoggerRawL7Flow = initRawL7FlowLogger()
}
