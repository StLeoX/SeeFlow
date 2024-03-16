package observe

import (
	"context"
	"fmt"
	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	monitorAPI "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/hubble/cmd/common/conn"
	hubdefaults "github.com/cilium/hubble/pkg/defaults"
	hubtime "github.com/cilium/hubble/pkg/time"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	pkgtracer "github.com/stleox/seeflow/pkg/tracer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
)

var tracerManager = pkgtracer.NewTracerManager()

func getHubbleClient(ctx context.Context, vp *viper.Viper) (observerpb.ObserverClient, func() error, error) {
	// conn to a hubble server
	hubbleEp := vp.GetString("HUBBLE_ENDPOINT")
	if hubbleEp == "" {
		hubbleEp = hubdefaults.ServerAddress
	}
	hubbleConn, err := conn.New(ctx, hubbleEp, hubdefaults.DialTimeout)
	if err != nil {
		return nil, nil, err
	}
	logrus.WithField("endpoint", hubbleEp).Info("connected to Hubble")
	client := observerpb.NewObserverClient(hubbleConn)
	cleanup := hubbleConn.Close
	return client, cleanup, nil
}

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

	// allow L7 flow
	allowList := make([]*flowpb.FlowFilter, 0)
	allowL7 := &flowpb.FlowFilter{
		EventType: []*flowpb.EventTypeFilter{
			{Type: monitorAPI.MessageTypeAccessLog},
		},
	}
	allowList = append(allowList, allowL7)

	// feat: allowList, blockList
	req := &observerpb.GetFlowsRequest{
		Number:    number,
		Follow:    selectorOpts.follow,
		Whitelist: allowList,
		Blacklist: nil,
		Since:     since,
		Until:     until,
		First:     first,
	}

	return req, nil
}

func handleFlows(ctx context.Context, client observerpb.ObserverClient, req *observerpb.GetFlowsRequest) error {
	c, err := client.GetFlows(ctx, req)
	if err != nil {
		return err
	}

	defer func() {
		tracerManager.Assemble()
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
			tracerManager.ConsumeFlow(resp.GetFlow())
		case *observerpb.GetFlowsResponse_NodeStatus:
			logrus.Infof("node status: %s", resp.GetNodeStatus().Message)
		default:
			return nil
		}
	}
}
