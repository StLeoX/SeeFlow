package serve

import (
	"context"
	"fmt"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
	"time"
)

// 在 observe 下，直接构造请求
func getFlowsRequest() *observerpb.GetFlowsRequest {
	now := time.Now()
	since := timestamppb.New(now.Add(-config.GetFlowsInterval))
	until := timestamppb.New(now)
	req := &observerpb.GetFlowsRequest{
		Blacklist: common.ConstructBlockList(),
		Whitelist: common.ConstructAllowList(),
		Since:     since,
		Until:     until,
	}
	return req
}

// 在 observe 下，分发处理
func handleFlows(ctx context.Context, hubble observerpb.ObserverClient, req *observerpb.GetFlowsRequest, tm *pkgtracer.TracerManager) error {
	c, err := hubble.GetFlows(ctx, req)
	if err != nil {
		return err
	}

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
	serve := &cobra.Command{
		Use:   "serve",
		Short: "Observe flows and assemble traces periodically",
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
			req := getFlowsRequest()
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
	return serve
}
