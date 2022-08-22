package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/itchyny/gojq"
	"github.com/jessevdk/go-flags"
	"github.com/joho/godotenv"
	"github.com/sacloud/libsacloud/v2/helper/api"
	"github.com/sacloud/libsacloud/v2/sacloud"
	"github.com/sacloud/libsacloud/v2/sacloud/search"
)

// version by Makefile
var version string

const UNKNOWN = 3
const CRITICAL = 2
const WARNING = 1
const OK = 0

type commandOpts struct {
	Time          uint     `long:"time" description:"Get average CPU usage for a specified amount of time" default:"3"`
	Prefix        []string `long:"prefix" description:"prefix for server names. prefix accepts more than one." required:"true"`
	Zones         []string `long:"zone" description:"zone name" required:"true"`
	PercentileSet string   `long:"percentile-set" default:"99,95,90,75" description:"percentiles to dispaly"`
	Version       bool     `short:"v" long:"version" description:"Show version"`
	Query         string   `long:"query" description:"jq style query to result and display"`
	EnvFrom       string   `long:"env-from" description:"load envrionment values from this file"`
	client        sacloud.ServerAPI
	percentiles   []percentile
}

type percentile struct {
	str   string
	float float64
}

func round(f float64) int64 {
	return int64(math.Round(f)) - 1
}

func serverClient() (sacloud.ServerAPI, error) {
	options := api.OptionsFromEnv()
	if options.AccessToken == "" {
		return nil, fmt.Errorf("environment variable %q is required", "SAKURACLOUD_ACCESS_TOKEN")
	}
	if options.AccessTokenSecret == "" {
		return nil, fmt.Errorf("environment variable %q is required", "SAKURACLOUD_ACCESS_TOKEN_SECRET")
	}

	if options.UserAgent == "" {
		options.UserAgent = fmt.Sprintf(
			"sacloud/sacloud-cpu-uage/v%s (%s/%s; +https://github.com/sacloud/sacloud-cpu-uage) %s",
			version,
			runtime.GOOS,
			runtime.GOARCH,
			sacloud.DefaultUserAgent,
		)
	}

	caller := api.NewCaller(options)
	return sacloud.NewServerOp(caller), nil
}

func findServers(opts commandOpts) ([]*sacloud.Server, error) {
	var servers []*sacloud.Server
	for _, prefix := range opts.Prefix {
		for _, zone := range opts.Zones {
			condition := &sacloud.FindCondition{
				Filter: map[search.FilterKey]interface{}{},
			}
			condition.Filter[search.Key("Name")] = search.PartialMatch(prefix)
			result, err := opts.client.Find(
				context.Background(),
				zone,
				condition,
			)
			if err != nil {
				return nil, err
			}
			for _, s := range result.Servers {
				if strings.Index(s.Name, prefix) == 0 {
					servers = append(servers, s)
				}
			}
		}
	}
	return servers, nil
}

func fetchMetrics(opts commandOpts, ss []*sacloud.Server) (map[string]interface{}, error) {

	b, _ := time.ParseDuration(fmt.Sprintf("-%dm", (opts.Time+3)*5))
	condition := &sacloud.MonitorCondition{
		Start: time.Now().Add(b),
		End:   time.Now(),
	}
	var fs sort.Float64Slice
	servers := make([]interface{}, 0)
	total := float64(0)
	for _, t := range ss {
		activity, err := opts.client.MonitorCPU(
			context.Background(),
			t.Zone.Name,
			t.ID,
			condition,
		)
		if err != nil {
			return nil, err
		}
		usages := activity.GetValues()
		if len(usages) == 0 {
			continue
		}
		if len(usages) > int(opts.Time) {
			usages = usages[len(usages)-int(opts.Time):]
		}
		sum := float64(0)
		monitors := make([]interface{}, 0)
		for _, p := range usages {
			m := map[string]interface{}{
				"cpu_time": p.GetCPUTime(),
				"time":     p.GetTime().String(),
			}
			monitors = append(monitors, m)
			log.Printf("%s zone:%s cpu_cores:%d cpu_time:%f time:%s", t.Name, t.Zone.Name, t.GetCPU(), p.GetCPUTime(), p.GetTime().String())
			u := p.GetCPUTime() / float64(t.GetCPU())
			sum += u
		}
		avg := sum * 100 / float64(len(usages))
		log.Printf("%s average_cpu_usage:%f", t.Name, avg)
		fs = append(fs, avg)
		total += avg

		servers = append(servers, map[string]interface{}{
			"name":     t.Name,
			"zone":     t.Zone.Name,
			"avg":      avg,
			"cores":    t.GetCPU(),
			"monitors": monitors,
		})
	}

	if len(fs) == 0 {
		result := map[string]interface{}{}
		result["max"] = float64(0)
		result["avg"] = float64(0)
		result["min"] = float64(0)
		for _, p := range opts.percentiles {
			result[fmt.Sprintf("%spt", p.str)] = float64(0)
		}
		result["servers"] = servers
		return result, nil
	}

	sort.Sort(fs)
	fl := float64(len(fs))
	result := map[string]interface{}{}
	result["max"] = fs[len(fs)-1]
	result["avg"] = total / fl
	result["min"] = fs[0]
	for _, p := range opts.percentiles {
		result[fmt.Sprintf("%spt", p.str)] = fs[round(fl*(p.float))]
	}
	result["servers"] = servers
	return result, nil
}

func printVersion() {
	fmt.Printf(`%s %s
Compiler: %s %s
`,
		os.Args[0],
		version,
		runtime.Compiler,
		runtime.Version())
}

func main() {
	os.Exit(_main())
}

func _main() int {
	opts := commandOpts{}
	psr := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)
	_, err := psr.Parse()
	if opts.Version {
		printVersion()
		return OK
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return UNKNOWN
	}

	if opts.Time < 1 {
		opts.Time = 1
	}

	if opts.EnvFrom != "" {
		godotenv.Load(opts.EnvFrom)
	}

	m := make(map[string]struct{})
	for _, z := range opts.Zones {
		if _, ok := m[z]; ok {
			log.Printf("zone %q is duplicated", z)
			return UNKNOWN
		}
		m[z] = struct{}{}
	}

	client, err := serverClient()
	if err != nil {
		log.Printf("%v", err)
		return UNKNOWN
	}
	opts.client = client

	percentiles := []percentile{}
	percentileStrings := strings.Split(opts.PercentileSet, ",")
	for _, s := range percentileStrings {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			log.Printf("Could not parse --percentile-set: %v", err)
			return UNKNOWN
		}
		f = f / 100
		percentiles = append(percentiles, percentile{s, f})
	}
	opts.percentiles = percentiles

	servers, err := findServers(opts)
	if err != nil {
		log.Printf("%v", err)
		return UNKNOWN
	}

	result, err := fetchMetrics(opts, servers)
	if err != nil {
		log.Printf("%v", err)
		return UNKNOWN
	}

	j, _ := json.Marshal(result)

	if opts.Query != "" {
		query, err := gojq.Parse(opts.Query)
		if err != nil {
			log.Printf("%v", err)
			return UNKNOWN
		}
		iter := query.Run(result)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := v.(error); ok {
				log.Printf("%v", err)
				return UNKNOWN
			}
			if v == nil {
				log.Printf("%s not found in result", opts.Query)
				return UNKNOWN
			}
			j2, _ := json.Marshal(v)
			fmt.Println(string(j2))
		}
	} else {
		fmt.Println(string(j))
	}

	return OK
}
