package config

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"time"
)

// for Log

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

func initLog4(path string) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	tmpLog, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	// defer tmpLog.Close()
	logger.SetOutput(tmpLog)
	return logger
}

const (
	PathRawL7  = "/tmp/seeflow_raw_l7.log.json"
	PathExL34  = "/tmp/seeflow_ex_l34.log.json"
	PathExL7   = "/tmp/seeflow_ex_l7.log.json"
	PathExSock = "/tmp/seeflow_ex_sock.log.json"
)

var (
	Log4RawL7  = initLog4(PathRawL7)
	Log4ExL34  = initLog4(PathExL34)
	Log4ExL7   = initLog4(PathExL7)
	Log4ExSock = initLog4(PathExSock)
)

func init() {
	initLogrus(nil)

	Log4RawL7.SetLevel(logrus.DebugLevel)

}
