package observe

import (
	"context"
	"fmt"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	hubdefaults "github.com/cilium/hubble/pkg/defaults"
	hubtime "github.com/cilium/hubble/pkg/time"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stleox/seeflow/pkg/cmd/common"
	"github.com/stleox/seeflow/pkg/config"
	pkgtracer "github.com/stleox/seeflow/pkg/tracer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
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

// 在 observe 下，保留 selectorFlags 便于单次调试
func getFlowsRequest() (*observerpb.GetFlowsRequest, error) {
	first := selectorOpts.first > 0
	last := selectorOpts.last > 0
	if first && last {
		return nil, fmt.Errorf("cannot set both --first and --last")
	}
	if first && selectorOpts.all {
		return nil, fmt.Errorf("cannot set both --first and --all")
	}
	if first && selectorOpts.follow {
		return nil, fmt.Errorf("cannot set both --first and --follow")
	}
	if last && selectorOpts.all {
		return nil, fmt.Errorf("cannot set both --last and --all")
	}

	// convert selectorOpts.since into a param for GetFlows
	var since, until *timestamppb.Timestamp
	if selectorOpts.since != "" {
		st, err := hubtime.FromString(selectorOpts.since)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the since time: %v", err)
		}

		since = timestamppb.New(st)
		if err := since.CheckValid(); err != nil {
			return nil, fmt.Errorf("failed to convert `since` timestamp to proto: %v", err)
		}
	}
	// Set the until field if --until option is specified and --follow
	// is not specified. If --since is specified but --until is not, the server sets the
	// --until option to the current timestamp.
	if selectorOpts.until != "" && !selectorOpts.follow {
		ut, err := hubtime.FromString(selectorOpts.until)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the until time: %v", err)
		}
		until = timestamppb.New(ut)
		if err := until.CheckValid(); err != nil {
			return nil, fmt.Errorf("failed to convert `until` timestamp to proto: %v", err)
		}
	}

	if since == nil && until == nil && !first {
		switch {
		case selectorOpts.all:
			// all is an alias for last=uint64_max
			selectorOpts.last = ^uint64(0)
		case selectorOpts.last == 0 && !selectorOpts.follow:
			// no specific parameters were provided, just a vanilla
			// `hubble observe` in non-follow mode
			selectorOpts.last = hubdefaults.FlowPrintCount
		}
	}

	number := selectorOpts.last
	if first {
		number = selectorOpts.first
	}

	req := &observerpb.GetFlowsRequest{
		Number:    number,
		Follow:    selectorOpts.follow,
		Whitelist: common.ConstructAllowList(),
		Blacklist: common.ConstructBlockList(),
		Since:     since,
		Until:     until,
		First:     first,
	}
	return req, nil
}

// 在 observe 下，设置有回调点
func handleFlows(ctx context.Context, hubble observerpb.ObserverClient, req *observerpb.GetFlowsRequest, tm *pkgtracer.TracerManager) error {
	c, err := hubble.GetFlows(ctx, req)
	if err != nil {
		return err
	}

	// mark: defer-point of observe cmd
	defer func() {
		tm.Assemble()
		tm.Flush()
		tm.SummaryELs()
	}()

	for {
		resp, err := c.Recv()

		switch err {
		case io.EOF, context.Canceled:
			return nil
		case nil:
		default:
			if status.Code(err) == codes.Canceled {
				return nil
			}
			return err
		}

		switch resp.GetResponseTypes().(type) {
		case *observerpb.GetFlowsResponse_Flow:
			tm.ConsumeFlow(resp.GetFlow())
		case *observerpb.GetFlowsResponse_NodeStatus:
			logrus.Infof("SeeFlow got Hubble status: %s", resp.GetNodeStatus().Message)
		default:
			return nil
		}
	}
}

func New(vp *viper.Viper) *cobra.Command {
	observe := &cobra.Command{
		Use:   "observe",
		Short: "Observe flows since last 10 seconds, then assemble traces",
		RunE: func(cmd *cobra.Command, args []string) error {
			// init main context
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
			defer cancel()

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

			// init Hubble's gRPC
			hubble, cleanup, err := common.GetHubbleClient(ctx, vp)
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
			if err := handleFlows(ctx, hubble, req, tracerManager); err != nil {
				msg := err.Error()
				// extract custom error message from failed grpc call
				if s, ok := status.FromError(err); ok && s.Code() == codes.Unknown {
					msg = s.Message()
				}
				return fmt.Errorf(msg)
			}
			return nil

		},
	}
	observe.Flags().AddFlagSet(selectorFlags)
	return observe
}
