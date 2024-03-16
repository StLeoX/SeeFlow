package config

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"time"
)

const (
	NameUnknown = "unknown"
	NameWorld   = "world"
)

// for root
var (
	Debug = false
)

// for pkg tracer
var (
	MaxNumFlow       = 1024
	MaxNumTracer     = 16
	MaxSpanTimestamp = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	MinSpanTimestamp = time.Unix(0, 0).UTC()
)

var LoggerRawL7Flow *logrus.Logger

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
