package main

import (
	"context"
	"testing"
	"time"

	"github.com/sacloud/iaas-api-go"
	"github.com/sacloud/iaas-api-go/types"
	usage "github.com/sacloud/sacloud-usage-lib"
	"github.com/stretchr/testify/require"
)

type stubIaasServerAPI struct {
	servers  map[string][]*iaas.Server
	activity map[string]map[string]*iaas.CPUTimeActivity // map[zone][resource-id]
}

var _ iaasServerAPI = (*stubIaasServerAPI)(nil)

func (s *stubIaasServerAPI) Find(_ context.Context, zone string, _ *iaas.FindCondition) (*iaas.ServerFindResult, error) {
	if _, ok := s.servers[zone]; !ok {
		return &iaas.ServerFindResult{}, nil
	}
	return &iaas.ServerFindResult{
		Total:   len(s.servers[zone]),
		From:    0,
		Count:   0,
		Servers: s.servers[zone],
	}, nil
}

func (s *stubIaasServerAPI) MonitorCPU(_ context.Context, zone string, id types.ID, _ *iaas.MonitorCondition) (*iaas.CPUTimeActivity, error) {
	if _, ok := s.activity[zone]; !ok {
		return &iaas.CPUTimeActivity{}, nil
	}
	if _, ok := s.activity[zone][id.String()]; !ok {
		return &iaas.CPUTimeActivity{}, nil
	}

	return s.activity[zone][id.String()], nil
}

func Test_fetchResources(t *testing.T) {
	type args struct {
		client iaasServerAPI
		opts   *usage.Option
	}
	tests := []struct {
		name    string
		args    args
		want    *usage.Resources
		wantErr bool
	}{
		{
			name: "empty",
			args: args{
				client: &stubIaasServerAPI{
					servers:  map[string][]*iaas.Server{},
					activity: nil,
				},
				opts: &usage.Option{},
			},
			want: &usage.Resources{
				Resources: nil,
				Label:     "servers",
			},
			wantErr: false,
		},
		{
			name: "single resource - single value",
			args: args{
				client: &stubIaasServerAPI{
					servers: map[string][]*iaas.Server{
						"is1a": {
							{
								ID:   types.ID(1),
								Name: "test1",
								CPU:  1,
							},
						},
					},
					activity: map[string]map[string]*iaas.CPUTimeActivity{
						"is1a": {
							"1": {
								Values: []*iaas.MonitorCPUTimeValue{
									{
										Time:    time.Unix(1, 0),
										CPUTime: 1,
									},
								},
							},
						},
					},
				},
				opts: &usage.Option{
					Prefix: []string{"test"},
					Zones:  []string{"is1a"},
					Time:   1,
				},
			},
			want: &usage.Resources{
				Resources: []*usage.Resource{
					{
						ID:   1,
						Name: "test1",
						Zone: "is1a",
						Monitors: []usage.MonitorValue{
							{
								Time:  time.Unix(1, 0),
								Value: 1,
							},
						},
						Label: "cpu_time",
						AdditionalInfo: map[string]interface{}{
							"cores": 1,
						},
					},
				},
				Label: "servers",
			},
		},
		{
			name: "single resource - multi values",
			args: args{
				client: &stubIaasServerAPI{
					servers: map[string][]*iaas.Server{
						"is1a": {
							{
								ID:   types.ID(1),
								Name: "test1",
								CPU:  1,
							},
						},
					},
					activity: map[string]map[string]*iaas.CPUTimeActivity{
						"is1a": {
							"1": {
								Values: []*iaas.MonitorCPUTimeValue{
									{
										Time:    time.Unix(1, 0),
										CPUTime: 1,
									},
									{
										Time:    time.Unix(2, 0),
										CPUTime: 2,
									},
									{
										Time:    time.Unix(3, 0),
										CPUTime: 3,
									},
								},
							},
						},
					},
				},
				opts: &usage.Option{
					Prefix: []string{"test"},
					Zones:  []string{"is1a"},
					Time:   3,
				},
			},
			want: &usage.Resources{
				Resources: []*usage.Resource{
					{
						ID:   1,
						Name: "test1",
						Zone: "is1a",
						Monitors: []usage.MonitorValue{
							{
								Time:  time.Unix(1, 0),
								Value: 1,
							},
							{
								Time:  time.Unix(2, 0),
								Value: 2,
							},
							{
								Time:  time.Unix(3, 0),
								Value: 3,
							},
						},
						Label: "cpu_time",
						AdditionalInfo: map[string]interface{}{
							"cores": 1,
						},
					},
				},
				Label: "servers",
			},
		},
		{
			name: "multi resources - single value",
			args: args{
				client: &stubIaasServerAPI{
					servers: map[string][]*iaas.Server{
						"is1a": {
							{
								ID:   types.ID(1),
								Name: "test1",
								CPU:  1,
							},
						},
						"is1b": {
							{
								ID:   types.ID(2),
								Name: "test2",
								CPU:  2,
							},
						},
					},
					activity: map[string]map[string]*iaas.CPUTimeActivity{
						"is1a": {
							"1": {
								Values: []*iaas.MonitorCPUTimeValue{
									{
										Time:    time.Unix(1, 0),
										CPUTime: 1,
									},
								},
							},
						},
						"is1b": {
							"2": {
								Values: []*iaas.MonitorCPUTimeValue{
									{
										Time:    time.Unix(2, 0),
										CPUTime: 2,
									},
								},
							},
						},
					},
				},
				opts: &usage.Option{
					Prefix: []string{"test"},
					Zones:  []string{"is1a", "is1b"},
					Time:   1,
				},
			},
			want: &usage.Resources{
				Resources: []*usage.Resource{
					{
						ID:   1,
						Name: "test1",
						Zone: "is1a",
						Monitors: []usage.MonitorValue{
							{
								Time:  time.Unix(1, 0),
								Value: 1,
							},
						},
						Label: "cpu_time",
						AdditionalInfo: map[string]interface{}{
							"cores": 1,
						},
					},
					{
						ID:   2,
						Name: "test2",
						Zone: "is1b",
						Monitors: []usage.MonitorValue{
							{
								Time:  time.Unix(2, 0),
								Value: 2,
							},
						},
						Label: "cpu_time",
						AdditionalInfo: map[string]interface{}{
							"cores": 2,
						},
					},
				},
				Label: "servers",
			},
		},
		{
			name: "multi resources - multi values",
			args: args{
				client: &stubIaasServerAPI{
					servers: map[string][]*iaas.Server{
						"is1a": {
							{
								ID:   types.ID(1),
								Name: "test1",
								CPU:  1,
							},
						},
						"is1b": {
							{
								ID:   types.ID(2),
								Name: "test2",
								CPU:  2,
							},
						},
					},
					activity: map[string]map[string]*iaas.CPUTimeActivity{
						"is1a": {
							"1": {
								Values: []*iaas.MonitorCPUTimeValue{
									{
										Time:    time.Unix(1, 0),
										CPUTime: 1,
									},
									{
										Time:    time.Unix(2, 0),
										CPUTime: 2,
									},
									{
										Time:    time.Unix(3, 0),
										CPUTime: 3,
									},
								},
							},
						},
						"is1b": {
							"2": {
								Values: []*iaas.MonitorCPUTimeValue{
									{
										Time:    time.Unix(4, 0),
										CPUTime: 4,
									},
									{
										Time:    time.Unix(5, 0),
										CPUTime: 5,
									},
									{
										Time:    time.Unix(6, 0),
										CPUTime: 6,
									},
								},
							},
						},
					},
				},
				opts: &usage.Option{
					Prefix: []string{"test"},
					Zones:  []string{"is1a", "is1b"},
					Time:   3,
				},
			},
			want: &usage.Resources{
				Resources: []*usage.Resource{
					{
						ID:   1,
						Name: "test1",
						Zone: "is1a",
						Monitors: []usage.MonitorValue{
							{
								Time:  time.Unix(1, 0),
								Value: 1,
							},
							{
								Time:  time.Unix(2, 0),
								Value: 2,
							},
							{
								Time:  time.Unix(3, 0),
								Value: 3,
							},
						},
						Label: "cpu_time",
						AdditionalInfo: map[string]interface{}{
							"cores": 1,
						},
					},
					{
						ID:   2,
						Name: "test2",
						Zone: "is1b",
						Monitors: []usage.MonitorValue{
							{
								Time:  time.Unix(4, 0),
								Value: 4,
							},
							{
								Time:  time.Unix(5, 0),
								Value: 5,
							},
							{
								Time:  time.Unix(6, 0),
								Value: 6,
							},
						},
						Label: "cpu_time",
						AdditionalInfo: map[string]interface{}{
							"cores": 2,
						},
					},
				},
				Label: "servers",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetchResources(context.Background(), tt.args.client, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("fetchResources() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got.Option = nil
			tt.want.Option = nil
			require.EqualValues(t, tt.want, got)
		})
	}
}
