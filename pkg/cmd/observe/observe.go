package observe

import (
	"context"
	"errors"
	"fmt"
	hubtime "github.com/cilium/hubble/pkg/time"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stleox/seeflow/pkg/config"
	pkgtracer "github.com/stleox/seeflow/pkg/tracer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	"os/signal"
	"strings"
	"time"
)

var (
	// selector var
	selectorOpts struct {
		all          bool
		last         uint64
		since, until string
		follow       bool
		first        uint64
	}

	// selector flags
	selectorFlags = pflag.NewFlagSet("selectors", pflag.ContinueOnError)
)

func init() {
	selectorFlags.BoolVar(&selectorOpts.all, "all", false, "Get all flows stored in Hubble's buffer. Note: this option may cause Hubble to return a lot of data. It is recommended to only use it along filters to limit the amount of data returned.")
	selectorFlags.Uint64Var(&selectorOpts.last, "last", 0, fmt.Sprintf("Get last N flows stored in Hubble's buffer (default %d). When querying against Hubble Relay, this gets N flows per instance of Hubble connected to that Relay.", config.BatchLastFlow))
	selectorFlags.Uint64Var(&selectorOpts.first, "first", 0, "Get first N flows stored in Hubble's buffer. When querying against Hubble Relay, this gets N flows per instance of Hubble connected to that Relay.")
	selectorFlags.BoolVarP(&selectorOpts.follow, "follow", "f", false, "Follow flows output")
	selectorFlags.StringVar(&selectorOpts.since,
		"since", "",
		fmt.Sprintf(`Filter flows since a specific date. The format is relative (e.g. 3s, 4m, 1h43,, ...) or one of:
  StampMilli:             %s
  YearMonthDay:           %s
  YearMonthDayHour:       %s
  YearMonthDayHourMinute: %s
  RFC3339:                %s
  RFC3339Milli:           %s
  RFC3339Micro:           %s
  RFC3339Nano:            %s
  RFC1123Z:               %s
 `,
			time.StampMilli,
			hubtime.YearMonthDay,
			strings.Replace(hubtime.YearMonthDayHour, "Z", "-", 1),
			strings.Replace(hubtime.YearMonthDayHourMinute, "Z", "-", 1),
			strings.Replace(time.RFC3339, "Z", "-", 1),
			strings.Replace(hubtime.RFC3339Milli, "Z", "-", 1),
			strings.Replace(hubtime.RFC3339Micro, "Z", "-", 1),
			strings.Replace(time.RFC3339Nano, "Z", "-", 1),
			strings.Replace(time.RFC1123Z, "Z", "-", 1),
		),
	)
	selectorFlags.StringVar(&selectorOpts.until,
		"until", "",
		fmt.Sprintf(`Filter flows until a specific date. The format is relative (e.g. 3s, 4m, 1h43,, ...) or one of:
  StampMilli:             %s
  YearMonthDay:           %s
  YearMonthDayHour:       %s
  YearMonthDayHourMinute: %s
  RFC3339:                %s
  RFC3339Milli:           %s
  RFC3339Micro:           %s
  RFC3339Nano:            %s
  RFC1123Z:               %s
 `,
			time.StampMilli,
			hubtime.YearMonthDay,
			strings.Replace(hubtime.YearMonthDayHour, "Z", "-", 1),
			strings.Replace(hubtime.YearMonthDayHourMinute, "Z", "-", 1),
			strings.Replace(time.RFC3339, "Z", "-", 1),
			strings.Replace(hubtime.RFC3339Milli, "Z", "-", 1),
			strings.Replace(hubtime.RFC3339Micro, "Z", "-", 1),
			strings.Replace(time.RFC3339Nano, "Z", "-", 1),
			strings.Replace(time.RFC1123Z, "Z", "-", 1),
		),
	)
}

func New(vp *viper.Viper) *cobra.Command {
	observe := &cobra.Command{
		Use:   "observe",
		Short: "Observe flows since last 10 seconds, then assemble traces",
		RunE: func(cmd *cobra.Command, args []string) error {
			// init main context
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
			defer cancel()

			// init bgTaskManager

			// init tracerManager
			tracerManager := pkgtracer.NewTracerManager(vp)

			// init exporter
			//shutdown, _ := tracerManager.InitGRPCExporter(tracerManager.ShutdownCtx)
			//shutdown, _ := tracerManager.InitStdoutExporter()
			shutdown, _ := tracerManager.InitDummyExporter()

			defer func() {
				if err := shutdown(tracerManager.ShutdownCtx); err != nil {
					logrus.Error(err)
				}
			}()

			// init gRPC
			client, cleanup, err := getHubbleClient(ctx, vp)
			if err != nil {
				return err
			}
			defer func() {
				if err = cleanup(); err != nil {
					panic(err)
				}
			}()

			// construct request
			req, err := getFlowsRequest()
			if err != nil {
				return err
			}
			logrus.WithField("request", req).Debug("SeeFlow sent GetFlows request")

			// handle flows
			if err := handleFlows(ctx, client, req, tracerManager); err != nil {
				msg := err.Error()
				// extract custom error message from failed grpc call
				if s, ok := status.FromError(err); ok && s.Code() == codes.Unknown {
					msg = s.Message()
				}
				return errors.New(msg)
			}
			return nil

		},
	}
	return observe
}
