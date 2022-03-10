package ec2

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/common/parallel"
	"github.com/influxdata/telegraf/plugins/processors"
)

type GceProcessor struct {
	AllowTags        []string        `toml:"allow_tags"`
	Timeout          config.Duration `toml:"timeout"`
	Ordered          bool            `toml:"ordered"`
	MaxParallelCalls int             `toml:"max_parallel_calls"`
	Log              telegraf.Logger `toml:"-"`

	gceClient    *metadata.Client
	allowTagsMap map[string]struct{}
	parallel     parallel.Parallel
	instanceID   string
}

const sampleConfig = `
  ## GCP Instance and project metadata to attach to metrics as tags.
  ## For more information see:
  ## https://cloud.google.com/compute/docs/metadata/default-metadata-values
  ##
  ## Available tags:
  ## * zone
  ## * tags
  ## * name
  ## * hostname
  allow_tags = []

  ## Timeout for http requests made by against gce metadata endpoint.
  timeout = "10s"

  ## ordered controls whether or not the metrics need to stay in the same order
  ## this plugin received them in. If false, this plugin will change the order
  ## with requests hitting cached results moving through immediately and not
  ## waiting on slower lookups. This may cause issues for you if you are
  ## depending on the order of metrics staying the same. If so, set this to true.
  ## Keeping the metrics ordered may be slightly slower.
  ordered = false

  ## max_parallel_calls is the maximum number of AWS API calls to be in flight
  ## at the same time.
  ## It's probably best to keep this number fairly low.
  max_parallel_calls = 10
`

const (
	DefaultMaxOrderedQueueSize = 10_000
	DefaultMaxParallelCalls    = 10
	DefaultTimeout             = 10 * time.Second
)

var permittedTags = map[string]struct{}{
	"zone":     {},
	"tags":     {},
	"name":     {},
	"hostname": {},
}

func (r *GceProcessor) SampleConfig() string {
	return sampleConfig
}

func (r *GceProcessor) Description() string {
	return "Attach GCE metadata to metrics"
}

func (r *GceProcessor) Add(metric telegraf.Metric, _ telegraf.Accumulator) error {
	r.parallel.Enqueue(metric)
	return nil
}

func (r *GceProcessor) Init() error {
	r.Log.Debug("Initializing GCE Processor")
	for _, tag := range r.AllowTags {
		if len(tag) == 0 || !isTagPermitted(tag) {
			return fmt.Errorf("un-permitted metadata tag specified in configuration: %s", tag)
		}
		r.allowTagsMap[tag] = struct{}{}
	}

	return nil
}

func (r *GceProcessor) Start(acc telegraf.Accumulator) error {
	r.gceClient = metadata.NewClient(nil)

	if r.Ordered {
		r.parallel = parallel.NewOrdered(acc, r.asyncAdd, DefaultMaxOrderedQueueSize, r.MaxParallelCalls)
	} else {
		r.parallel = parallel.NewUnordered(acc, r.asyncAdd, r.MaxParallelCalls)
	}

	return nil
}

func (r *GceProcessor) Stop() error {
	if r.parallel == nil {
		return errors.New("trying to stop unstarted GCE Processor")
	}
	r.parallel.Stop()
	return nil
}

func (r *GceProcessor) asyncAdd(metric telegraf.Metric) []telegraf.Metric {
	_, cancel := context.WithTimeout(context.Background(), time.Duration(r.Timeout))
	defer cancel()

	if len(r.allowTagsMap) > 0 {
		for tag := range r.allowTagsMap {
			fmt.Println(tag)
			val, err := r.getTagFromGCE(tag)
			if err != nil {
				panic(err)
			}
			metric.AddTag(tag, val)
		}
	}

	return []telegraf.Metric{metric}
}

func init() {
	processors.AddStreaming("aws_ec2", func() telegraf.StreamingProcessor {
		return newGceProcessor()
	})
}

func newGceProcessor() *GceProcessor {
	return &GceProcessor{
		MaxParallelCalls: DefaultMaxParallelCalls,
		Timeout:          config.Duration(DefaultTimeout),
		allowTagsMap:     make(map[string]struct{}),
	}
}

func (r *GceProcessor) getTagFromGCE(tag string) (string, error) {
	switch tag {
	case "zone":
		zone, err := r.gceClient.Zone()
		if err != nil {
			return "", err
		}
		return strings.Split(zone, "/")[3], nil
	case "tags":
		instance_tags, err := r.gceClient.InstanceTags()
		if err != nil {
			return "", err
		}
		return strings.Join(instance_tags, ","), nil
	case "name":
		name, err := r.gceClient.InstanceName()
		if err != nil {
			return "", err
		}
		return name, nil
	case "hostname":
		hostname, err := r.gceClient.Hostname()
		if err != nil {
			return "", err
		}
		return hostname, nil
	default:
		return "", nil
	}
}

func isTagPermitted(tag string) bool {
	_, ok := permittedTags[tag]
	return ok
}
