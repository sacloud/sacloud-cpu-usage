package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sacloud/go-otelsetup"
	"github.com/sacloud/iaas-api-go"
	"github.com/sacloud/iaas-api-go/search"
	"github.com/sacloud/iaas-api-go/types"
	usage "github.com/sacloud/sacloud-usage-lib"
	"go.opentelemetry.io/otel"
)

// version by Makefile
var version string

const appName = "github.com/sacloud/sacloud-cpu-usage"

func main() {
	os.Exit(_main())
}

func _main() int {
	// initialize OTel SDK
	otelShutdown, err := otelsetup.Init(context.Background(), appName, version)
	if err != nil {
		log.Println("Error in initializing OTel SDK: " + err.Error())
		return usage.ExitUnknown
	}
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
		if err != nil {
			log.Println("Error in initializing OTel SDK: " + err.Error())
		}
	}()

	// init root span
	ctx, span := otel.Tracer(appName).Start(otelsetup.ContextForTrace(context.Background()), "usage.main")
	defer span.End()

	opts := &usage.Option{}
	if err := usage.ParseOption(opts); err != nil {
		log.Println(err)
		return usage.ExitUnknown
	}
	if opts.Version {
		usage.PrintVersion(version)
		return usage.ExitOk
	}

	caller, err := usage.SacloudAPICaller("sacloud-cpu-usage", version)
	if err != nil {
		log.Println(err)
		return usage.ExitUnknown
	}

	resources, err := fetchResources(ctx, iaas.NewServerOp(caller), opts)
	if err != nil {
		log.Println(err)
		return usage.ExitUnknown
	}

	if err := usage.OutputMetrics(os.Stdout, resources.Metrics(), opts.Query); err != nil {
		log.Println(err)
		return usage.ExitUnknown
	}
	return usage.ExitOk
}

type iaasServerAPI interface {
	Find(ctx context.Context, zone string, conditions *iaas.FindCondition) (*iaas.ServerFindResult, error)
	MonitorCPU(ctx context.Context, zone string, id types.ID, condition *iaas.MonitorCondition) (*iaas.CPUTimeActivity, error)
}

func fetchResources(ctx context.Context, client iaasServerAPI, opts *usage.Option) (*usage.Resources, error) {
	ctx, span := otel.Tracer(appName).Start(ctx, "usage.fetchResources")
	defer span.End()

	rs := &usage.Resources{Label: "servers", Option: opts}
	for _, prefix := range opts.Prefix {
		for _, zone := range opts.Zones {
			condition := &iaas.FindCondition{
				Filter: map[search.FilterKey]interface{}{},
			}
			condition.Filter[search.Key("Name")] = search.PartialMatch(prefix)
			result, err := client.Find(ctx, zone, condition)
			if err != nil {
				return nil, err
			}
			for _, r := range result.Servers {
				if !strings.HasPrefix(r.Name, prefix) {
					continue
				}
				monitors, err := fetchServerActivities(ctx, client, zone, r.ID, opts)
				if err != nil {
					return nil, err
				}
				rs.Resources = append(rs.Resources, &usage.Resource{
					ID:       r.ID,
					Name:     r.Name,
					Zone:     zone,
					Monitors: monitors,
					Label:    "cpu_time",
					AdditionalInfo: map[string]interface{}{
						"cores": r.GetCPU(),
					},
				})
			}
		}
	}
	return rs, nil
}

func fetchServerActivities(ctx context.Context, client iaasServerAPI, zone string, id types.ID, opts *usage.Option) ([]usage.MonitorValue, error) {
	ctx, span := otel.Tracer(appName).Start(ctx, "usage.fetchServerActivities")
	defer span.End()

	b, _ := time.ParseDuration(fmt.Sprintf("-%dm", (opts.Time+3)*5))
	condition := &iaas.MonitorCondition{
		Start: time.Now().Add(b),
		End:   time.Now(),
	}
	activity, err := client.MonitorCPU(ctx, zone, id, condition)
	if err != nil {
		return nil, err
	}
	usages := activity.GetValues()
	if len(usages) > int(opts.Time) {
		usages = usages[len(usages)-int(opts.Time):]
	}

	var results []usage.MonitorValue
	for _, u := range usages {
		results = append(results, usage.MonitorValue{Time: u.Time, Value: u.CPUTime})
	}
	return results, nil
}
