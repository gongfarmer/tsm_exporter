# TSM Prometheus exporter

[![Build Status](https://circleci.com/gh/treydock/tsm_exporter/tree/main.svg?style=shield)](https://circleci.com/gh/treydock/tsm_exporter)
[![GitHub release](https://img.shields.io/github/v/release/treydock/tsm_exporter?include_prereleases&sort=semver)](https://github.com/treydock/tsm_exporter/releases/latest)
![GitHub All Releases](https://img.shields.io/github/downloads/treydock/tsm_exporter/total)
[![Go Report Card](https://goreportcard.com/badge/github.com/treydock/tsm_exporter)](https://goreportcard.com/report/github.com/treydock/tsm_exporter)
[![codecov](https://codecov.io/gh/treydock/tsm_exporter/branch/main/graph/badge.svg)](https://codecov.io/gh/treydock/tsm_exporter)

# TSM Prometheus exporter

The TSM exporter collects metrics from the Tivoli Storage Manager (IBM Spectrum Protect).

This expecter is intended to query multiple TSM servers from an external host.

The `/tsm` metrics endpoint exposes TSM metrics and requires the `target` parameter.

The `/metrics` endpoint exposes Go and process metrics for this exporter.

This exporter has been tested with TSM 8.1.2.

## Collectors

Collectors are enabled or disabled via a config file.

Name | Description | Default
-----|-------------|--------
status | Collect status information about TSM | Enabled
volumes | Collect count of unavailable or readonly volumes | Enabled
log | Collect active log space metrics | Enabled
db | Collect DB space information | Enabled
occupancy | Collect occupancy metrics | Enabled
libvolumes | Collect count of scratch tapes | Enabled
drives | Collect count of offline drives | Enabled
events | Collect event duration and number of not completed events | Enabled
replicationview | Collect metrics about replication | Enabled
stgpools | Collect storage pool metrics | Enabled
volumeusage | Collect aggregates of volume counts by node name | Enabled
summary | Collect backup summary information | Enabled

## Configuration

The configuration defines targets that are to be queried. Example:

```yaml
targets:
  tsm1.example.com:
    id: somwell
    password: secret
    library_name: TAPE
    schedules:
    - MYSQL
    replication_node_names:
    - TESTDB
  tsm2.example.com:
    id: somwell
    password: secret
    timezone: America/New_York
    collectors:
    - status
    - volumes
    - log
    - db
    - volumeusage
    - summary
    volumeusage_map:
      LTO6: '^E.*'
      LT07: '^F.*'
    summary_activities:
    - BACKUP
    - REPLICATION
```

**WARNING**: Due to limitations with Go expect libraries and limitations with how passwords as passed to dsmadmc, 
this code must pass the configured password via CLI arguments. In testing it appears like dsmadmc strips the password
after execution but this does not guarantee the password cannot be exposed.
Take proper precautions in protecting the host running this exporter.

This exporter could then be queried via one of these two commands below.  The `tsm2.example.com` target will only run the `status`, `volumes`, `log` and `db` collectors.

```
curl http://localhost:9310/tsm?target=tsm1.example.com
curl http://localhost:9310/tsm?target=tsm2.example.com
```

The key for each target should match the `servername` value for the entry in `dsm.sys`.  You may optionally add the `servername` key to override the servername used when executing `dsmadmc`.

The `libvolumes` and `drives` collectors can be limited to a specific library name via `library_name` config value, eg: `library_name: TAPE`.

The `events` collector can be limited to specific schedules via the `schedules` config value.

The `replicationview` collector can be limited to specific node names via the `replication_node_names` config value.

The `volumeusage` collector can map specific volume names to metric labels via `volumeusage_map` config value.
The example above will map volumes starting with `E` to be counted as `LTO6` and volumes starting with `F` counted as `LT07`. If no mapping is defined the metrics will just set `volumename="all"` and the metrics will count volumes per node name.

The `summary` collector can have specific activies queried via the `summary_activities` config value. By default
all activities are queried except `'TAPE MOUNT','EXPIRATION','PROCESS_START','PROCESS_END'` and anything beginning with `SUR_`.

Times are parsed using the timezone of the host running this exporter. If that timezone differs for a TSM host you can use `--config.timezone` flag or set `timezone` configuration for a target, such as `America/New_York`.  The target `timezone` config option takes precedence.

## Dependencies

This exporter relies on the `dsmadmc` command. The host running the exporter is expected to have both the `dsmadmc` executable and files `/opt/tivoli/tsm/client/ba/bin/dsm.sys` and `/opt/tivoli/tsm/client/ba/bin/dsm.opt`.

The hosts being queried by this exporter must exist in `/opt/tivoli/tsm/client/ba/bin/dsm.sys`.

To validate your system is able to properly query TSM servers (substitute environment variables for real values):

```
/opt/tivoli/tsm/client/ba/bin/dsmadmc -servername=$SERVERNAME -id=$USERNAME -password=$PASSWORD \
  -DATAONLY=YES -COMMAdelimited "QUERY STATUS"
```

This has been validated on a host with `TIVsm-BA` and `TIVsm-API64` RPMs installed.

## Docker

Example of running the Docker container. This relies on the [Dependencies](#dependencies) being installed on the host running Docker.

```
docker run -d -p 9310:9310 --name tsm_exporter \
-v "tsm_exporter.yaml:/tsm_exporter.yaml:ro" \
-v "/opt/tivoli:/opt/tivoli:ro" \
-v "/usr/local/ibm:/usr/local/ibm:ro" \
treydock/tsm_exporter
```

## Install

Download the [latest release](https://github.com/treydock/tsm_exporter/releases)

Add the user that will run `tsm_exporter`

```
groupadd -r tsm_exporter
useradd -r -d /var/lib/tsm_exporter -s /sbin/nologin -M -g tsm_exporter -M tsm_exporter
```

Install compiled binaries after extracting tar.gz from release page.

```
cp /tmp/tsm_exporter /usr/local/bin/tsm_exporter
```

Install the necessary dependencies, see [dependencies section](#dependencies)

Add the necessary config, see [configuration section](#configuration)

Add systemd unit file and start service. Modify the `ExecStart` with desired flags.

```
cp systemd/tsm_exporter.service /etc/systemd/system/tsm_exporter.service
systemctl daemon-reload
systemctl start tsm_exporter
```

## Build from source

To produce the `tsm_exporter` binary:

```
make build
```

Or

```
go get github.com/treydock/tsm_exporter
```

## Prometheus configs

The following example assumes this exporter is running on the Prometheus server and communicating to the remote TSM hosts.

```yaml
- job_name: tsm
  metrics_path: /tsm
  static_configs:
  - targets:
    - tsm1.example.com
    - tsm2.example.com
  relabel_configs:
  - source_labels: [__address__]
    target_label: __param_target
  - source_labels: [__param_target]
    target_label: instance
  - target_label: __address__
    replacement: 127.0.0.1:9310
- job_name: tsm-metrics
  metrics_path: /metrics
  static_configs:
  - targets:
    - localhost:9310
```

## Grafana Dashboard

An example Grafana dashboard can be found here: https://grafana.com/grafana/dashboards/14054

The dashboard can also be found at [grafana/tsm.json](grafana/tsm.json)
