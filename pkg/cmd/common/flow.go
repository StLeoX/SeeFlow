package common

import (
	"context"
	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	monitorAPI "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/hubble/cmd/common/conn"
	hubdefaults "github.com/cilium/hubble/pkg/defaults"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func GetHubbleClient(ctx context.Context, vp *viper.Viper) (observerpb.ObserverClient, func() error, error) {
	// conn to a hubble server
	hubbleEp := vp.GetString("SEEFLOW_HUBBLE_ENDPOINT")
	if hubbleEp == "" {
		hubbleEp = hubdefaults.ServerAddress
	}
	hubbleConn, err := conn.New(ctx, hubbleEp, hubdefaults.DialTimeout)
	if err != nil {
		return nil, nil, err
	}
	logrus.WithField("endpoint", hubbleEp).Info("SeeFlow connected to Hubble")
	client := observerpb.NewObserverClient(hubbleConn)
	cleanup := hubbleConn.Close
	return client, cleanup, nil
}

func ConstructAllowList() []*flowpb.FlowFilter {
	// allow all L7 flows, 参考 @github.com/cilium/hubble@v0.12.3/cmd/observe/flows_filter_test.go:73
	allowList := make([]*flowpb.FlowFilter, 0)
	allowL7 := &flowpb.FlowFilter{
		EventType: []*flowpb.EventTypeFilter{
			{Type: monitorAPI.MessageTypeAccessLog},
		},
	}
	allowList = append(allowList, allowL7)

	// allow partial L34 flows
	allowL34 := &flowpb.FlowFilter{
		EventType: []*flowpb.EventTypeFilter{
			{Type: monitorAPI.MessageTypeTrace},
		},
	}
	allowList = append(allowList, allowL34)

	// allow all Sock flows
	allowSock := &flowpb.FlowFilter{
		EventType: []*flowpb.EventTypeFilter{
			{Type: monitorAPI.MessageTypeTraceSock},
		},
	}
	allowList = append(allowList, allowSock)

	return allowList
}

func ConstructBlockList() []*flowpb.FlowFilter {
	// block DNS flows, because DNS is not related with payload
	blockList := make([]*flowpb.FlowFilter, 0)
	// 过滤器是基于 "k8s:k8s-app=kube-dns" label，参考 @github.com/cilium/hubble@v0.12.3/cmd/observe/flows_filter_test.go:260
	blockDNS := &flowpb.FlowFilter{
		SourceLabel:      []string{"k8s:k8s-app=kube-dns"},
		DestinationLabel: []string{"k8s:k8s-app=kube-dns"},
	}
	blockList = append(blockList, blockDNS)
	return blockList
}
