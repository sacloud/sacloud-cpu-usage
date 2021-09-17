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

	"github.com/jessevdk/go-flags"
	"github.com/joho/godotenv"
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
	Prefix        []string `long:"prefix" description:"prefix for server name" required:"true"`
	Zone          string   `long:"zone" description:"zone name" required:"true"`
	PercentileSet string   `long:"percentile-set" default:"99,95,90,75" description:"percentiles to dispaly"`
	Version       bool     `short:"v" long:"version" description:"Show version"`
}

type percentile struct {
	str   string
	float float64
}

func round(f float64) int64 {
	return int64(math.Round(f)) - 1
}

func serverClient() (sacloud.ServerAPI, error) {
	client, err := sacloud.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return sacloud.NewServerOp(client), nil
}

func findServers(opts commandOpts, client sacloud.ServerAPI) ([]*sacloud.Server, error) {
	var servers []*sacloud.Server
	for _, prefix := range opts.Prefix {
		condition := &sacloud.FindCondition{
			Filter: map[search.FilterKey]interface{}{},
		}
		condition.Filter[search.Key("Name")] = search.PartialMatch(prefix)
		result, err := client.Find(
			context.Background(),
			opts.Zone,
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
	return servers, nil
}

func printVersion() {
	fmt.Printf(`%s Compiler: %s %s`,
		version,
		runtime.Compiler,
		runtime.Version())
}

func main() {
	godotenv.Load()
	os.Exit(_main())
}

func _main() int {
	opts := commandOpts{}
	psr := flags.NewParser(&opts, flags.Default)
	_, err := psr.Parse()
	if err != nil {
		return UNKNOWN
	}

	if opts.Version {
		printVersion()
		return OK
	}

	client, err := serverClient()
	if err != nil {
		log.Printf("%v", err)
		return UNKNOWN
	}

	if opts.Time < 1 {
		opts.Time = 1
	}

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

	servers, err := findServers(opts, client)
	if err != nil {
		log.Printf("%v", err)
		return UNKNOWN
	}

	if len(servers) == 0 {
		result := map[string]interface{}{}
		result["count"] = 0
		result["max"] = float64(0)
		result["avg"] = float64(0)
		result["min"] = float64(0)
		for _, p := range percentiles {
			result[fmt.Sprintf("%spt", p.str)] = float64(0)
		}
		j, _ := json.Marshal(result)
		fmt.Println(string(j))
		return OK
	}

	b, _ := time.ParseDuration(fmt.Sprintf("-%dm", (opts.Time+3)*5))
	condition := &sacloud.MonitorCondition{
		Start: time.Now().Add(b),
		End:   time.Now(),
	}
	var fs sort.Float64Slice
	total := float64(0)
	for _, t := range servers {

		activity, err := client.MonitorCPU(
			context.Background(),
			opts.Zone,
			t.ID,
			condition,
		)
		if err != nil {
			log.Printf("%v", err)
			return UNKNOWN
		}
		usages := activity.GetValues()
		if len(usages) == 0 {
			continue
		}
		if len(usages) > int(opts.Time) {
			usages = usages[len(usages)-int(opts.Time):]
		}
		sum := float64(0)
		for _, p := range usages {
			log.Printf("%s cores:%d cpu:%f time:%s", t.Name, t.GetCPU(), p.GetCPUTime(), p.GetTime().String())
			u := p.GetCPUTime() / float64(t.GetCPU())
			sum += u
		}
		avg := sum * 100 / float64(len(usages))
		log.Printf("%s avg:%f", t.Name, avg)
		fs = append(fs, avg)
		total += avg
	}
	sort.Sort(fs)
	fl := float64(len(fs))
	result := map[string]interface{}{}
	result["count"] = len(fs)
	result["max"] = fs[len(fs)-1]
	result["avg"] = total / fl
	result["min"] = fs[0]
	for _, p := range percentiles {
		result[fmt.Sprintf("%spt", p.str)] = fs[round(fl*(p.float))]
	}
	j, _ := json.Marshal(result)
	fmt.Println(string(j))
	return OK
}
