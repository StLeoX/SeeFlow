package cmd

import (
	"github.com/cilium/hubble/cmd/common/conn"
	"github.com/cilium/hubble/cmd/common/validate"
	"github.com/cilium/hubble/pkg/defaults"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stleox/seeflow/pkg/cmd/observe"
	"github.com/stleox/seeflow/pkg/config"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	// debug flag
	pflag.BoolVar(&config.Debug, "debug", false, "Enable debug mode")
}

// NewViper creates a new viper instance configured.
func NewViper() *viper.Viper {
	vp := viper.New()

	// read config from a file
	vp.SetConfigName("config") // name of config file (without extension)
	vp.SetConfigType("yaml")   // useful if the given config file does not have the extension in the name
	vp.AddConfigPath(".")      // look for a config in the working directory first
	if defaults.ConfigDir != "" {
		vp.AddConfigPath(defaults.ConfigDir)
	}
	if defaults.ConfigDirFallback != "" {
		vp.AddConfigPath(defaults.ConfigDirFallback)
	}

	// read config from environment variables
	vp.SetEnvPrefix("seeflow") // env var must start with SEEFLOW_
	// replace - by _ for environment variable names
	// (eg: the env var for tls-server-name is TLS_SERVER_NAME)
	vp.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	vp.AutomaticEnv() // read in environment variables that match
	return vp
}

func New(vp *viper.Viper) *cobra.Command {
	root := &cobra.Command{
		Use:   "seeflow",
		Short: "seeflow",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := validate.Flags(cmd, vp); err != nil {
				return err
			}

			if config.Debug {
				logrus.Info("enabled debug mode")
			} else {
				logrus.Info("disabled debug mode")
			}

			if err := conn.Init(vp); err != nil {
				return err
			}

			return nil
		},
	}
	return root
}

func Execute() {
	// 全局初始化 VP 配置
	vp := NewViper()

	root := New(vp)
	root.AddCommand(observe.New(vp))

	err := root.Execute()
	if err != nil {
		os.Exit(1)
	}
}
