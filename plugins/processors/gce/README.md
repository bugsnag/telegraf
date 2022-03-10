# GCE Metadata Processor Plugin

GCE Metadata processor plugin appends metadata gathered from Google Cloud
to metrics associated with GCE instances.

## Configuration

```toml
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
```
