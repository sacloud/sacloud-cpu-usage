package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/sacloud/iaas-api-go"
	"github.com/sacloud/iaas-api-go/helper/api"
	"github.com/sacloud/iaas-api-go/search"
	"github.com/sacloud/iaas-api-go/types"
	usage "github.com/sacloud/sacloud-usage-lib"
)

// version by Makefile
var version string

func main() {
	os.Exit(_main())
}

func _main() int {
	opts := &usage.Option{}
	if err := usage.ParseOption(opts); err != nil {
		log.Println(err)
		return usage.ExitUnknown
	}
	if opts.Version {
		usage.PrintVersion(version)
		return usage.ExitOk
	}

	client, err := serverClient()
	if err != nil {
		log.Println(err)
		return usage.ExitUnknown
	}

	resources, err := fetchResources(client, opts)
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

func serverClient() (iaasServerAPI, error) {
	options := api.OptionsFromEnv()
	if options.AccessToken == "" {
		return nil, fmt.Errorf("environment variable %q is required", "SAKURACLOUD_ACCESS_TOKEN")
	}
	if options.AccessTokenSecret == "" {
		return nil, fmt.Errorf("environment variable %q is required", "SAKURACLOUD_ACCESS_TOKEN_SECRET")
	}

	if options.UserAgent == "" {
		options.UserAgent = fmt.Sprintf(
			"sacloud/sacloud-cpu-usage/v%s (%s/%s; +https://github.com/sacloud/sacloud-cpu-usage) %s",
			version,
			runtime.GOOS,
			runtime.GOARCH,
			iaas.DefaultUserAgent,
		)
	}

	caller := api.NewCallerWithOptions(options)
	return iaas.NewServerOp(caller), nil
}

func fetchResources(client iaasServerAPI, opts *usage.Option) (*usage.Resources, error) {
	rs := &usage.Resources{Label: "servers", Option: opts}
	for _, prefix := range opts.Prefix {
		for _, zone := range opts.Zones {
			condition := &iaas.FindCondition{
				Filter: map[search.FilterKey]interface{}{},
			}
			condition.Filter[search.Key("Name")] = search.PartialMatch(prefix)
			result, err := client.Find(
				context.Background(),
				zone,
				condition,
			)
			if err != nil {
				return nil, err
			}
			for _, r := range result.Servers {
				if !strings.HasPrefix(r.Name, prefix) {
					continue
				}
				monitors, err := fetchServerActivities(client, zone, r.ID, opts)
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

func fetchServerActivities(client iaasServerAPI, zone string, id types.ID, opts *usage.Option) ([]usage.MonitorValue, error) {
	b, _ := time.ParseDuration(fmt.Sprintf("-%dm", (opts.Time+3)*5))
	condition := &iaas.MonitorCondition{
		Start: time.Now().Add(b),
		End:   time.Now(),
	}
	activity, err := client.MonitorCPU(context.Background(), zone, id, condition)
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
