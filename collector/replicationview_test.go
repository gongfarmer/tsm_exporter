// Copyright 2020 Trey Dockendorf
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/treydock/tsm_exporter/config"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	mockReplicationViewStdout = `
COMPLETE,2020-03-23 06:06:45.000000,/TEST2CONF,TEST2DB2,2020-03-23 00:45:29.000000,167543418,2
COMPLETE,2020-03-23 06:06:45.000000,/TEST4,TEST2DB2,2020-03-23 00:45:29.000000,1052637876956,2
COMPLETE,2020-03-23 00:06:07.000000,/srv,TEST2DB.DOMAIN,2020-03-23 00:05:24.000000,245650752,10
COMPLETE,2020-03-22 06:02:38.000000,/TEST2CONF,TEST2DB2,2020-03-22 00:45:29.000000,167543418,2
COMPLETE,2020-03-22 06:02:38.000000,/TEST4,TEST2DB2,2020-03-22 00:45:29.000000,1052637876316,2
COMPLETE,2020-03-22 00:05:57.000000,/srv,TEST2DB.DOMAIN,2020-03-22 00:05:23.000000,234680204,12
`
)

func TestReplicationViewsParse(t *testing.T) {
	metrics := replicationviewParse(mockReplicationViewStdout, &config.Target{Name: "test"}, false, log.NewNopLogger())
	if len(metrics) != 3 {
		t.Errorf("Expected 3 metrics, got %d", len(metrics))
		return
	}
	if val := metrics["TEST2DB2-/TEST2CONF"].NotCompleted; val != 0 {
		t.Errorf("Expected 0 notCompleted, got %v", val)
	}
	if val := metrics["TEST2DB2-/TEST2CONF"].Duration; val != 19276 {
		t.Errorf("Expected 19276 duration, got %v", val)
	}
	if val := metrics["TEST2DB.DOMAIN-/srv"].Duration; val != 43 {
		t.Errorf("Expected 43 duration, got %v", val)
	}
}

func TestReplicationViewsParseMultipleNotCompleted(t *testing.T) {
	stdout := `
NOT COMPLETED,2020-03-23 06:06:45.000000,/TEST2CONF,TEST2DB2,2020-03-23 00:45:29.000000,167543418,2
COMPLETE,2020-03-23 06:06:45.000000,/TEST4,TEST2DB2,2020-03-23 00:45:29.000000,1052637876956,2
COMPLETE,2020-03-23 00:06:07.000000,/srv,TEST2DB.DOMAIN,2020-03-23 00:05:24.000000,245650752,10
NOT COMPLETED,2020-03-22 06:02:38.000000,/TEST2CONF,TEST2DB2,2020-03-22 00:45:29.000000,167543418,2
COMPLETE,2020-03-22 06:02:38.000000,/TEST4,TEST2DB2,2020-03-22 00:45:29.000000,1052637876316,2
COMPLETE,2020-03-22 00:05:57.000000,/srv,TEST2DB.DOMAIN,2020-03-22 00:05:23.000000,234680204,12
`
	metrics := replicationviewParse(stdout, &config.Target{Name: "test"}, false, log.NewNopLogger())
	if len(metrics) != 3 {
		t.Errorf("Expected 3 metrics, got %d", len(metrics))
		return
	}
	if val := metrics["TEST2DB2-/TEST2CONF"].NotCompleted; val != 2 {
		t.Errorf("Expected 2 notCompleted, got %v", val)
	}
}

func TestReplicationViewsParseWithNodeNames(t *testing.T) {
	metrics := replicationviewParse(mockReplicationViewStdout, &config.Target{Name: "test", ReplicationNodeNames: []string{"TEST2DB2"}}, false, log.NewNopLogger())
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(metrics))
	}
	if val := metrics["TEST2DB2-/TEST2CONF"].NotCompleted; val != 0 {
		t.Errorf("Expected 0 notCompleted, got %v", val)
	}
}

func TestReplicationViewsParseNegativeEndtime(t *testing.T) {
	stdout := `
NOT COMPLETED,1970-01-01 00:00:00.000000,/TEST4,TEST2DB2,2020-03-23 00:45:29.000000,1052637876956,2
`
	metrics := replicationviewParse(stdout, &config.Target{Name: "test"}, false, log.NewNopLogger())
	if len(metrics) != 1 {
		t.Errorf("Expected 1 metrics, got %d", len(metrics))
		return
	}
	if val := metrics["TEST2DB2-/TEST4"].EndTimestamp; val != 0 {
		t.Errorf("Expected 0 EndTimestamp, got %v", val)
	}
	if val := metrics["TEST2DB2-/TEST4"].Duration; val != 0 {
		t.Errorf("Expected 0 Duration, got %v", val)
	}
}

func TestReplicationViewsParseDurationCache(t *testing.T) {
	metrics := replicationviewParse(mockReplicationViewStdout, &config.Target{Name: "test"}, true, log.NewNopLogger())
	if len(metrics) != 3 {
		t.Errorf("Expected 3 metrics, got %d", len(metrics))
		return
	}
	if val := metrics["TEST2DB2-/TEST2CONF"].NotCompleted; val != 0 {
		t.Errorf("Expected 0 notCompleted, got %v", val)
	}
	if val := metrics["TEST2DB2-/TEST2CONF"].Duration; val != 19276 {
		t.Errorf("Expected 19276 duration, got %v", val)
	}
	stdout := `
NOTCOMPLETE,2020-03-23 06:06:45.000000,/TEST2CONF,TEST2DB2,2020-03-23 00:45:29.000000,167543418,2
COMPLETE,2020-03-23 06:06:45.000000,/TEST4,TEST2DB2,2020-03-23 00:45:29.000000,1052637876956,2
COMPLETE,2020-03-23 00:06:08.000000,/srv,TEST2DB.DOMAIN,2020-03-23 00:05:24.000000,245650752,10
`
	metrics = replicationviewParse(stdout, &config.Target{Name: "test"}, true, log.NewNopLogger())
	if len(metrics) != 3 {
		t.Errorf("Expected 3 metrics, got %d", len(metrics))
		return
	}
	if val := metrics["TEST2DB2-/TEST2CONF"].NotCompleted; val != 1 {
		t.Errorf("Expected 1 notCompleted, got %v", val)
	}
	if val := metrics["TEST2DB2-/TEST2CONF"].Duration; val != 19276 {
		t.Errorf("Expected 19276 duration, got %v", val)
	}
	if val := metrics["TEST2DB.DOMAIN-/srv"].Duration; val != 44 {
		t.Errorf("Expected 43 duration, got %v", val)
	}
}

func TestReplicationHandleBadValues(t *testing.T) {
	stdout := `
COMPLETE,bad end date,/TEST4,TEST2DB2,2020-03-23 00:45:29.000000,1052637876956,bad number
COMPLETE,2020-03-23 00:06:08.000000,/srv,TEST2DB.DOMAIN,bad start date,bad number,10
`
	metrics := replicationviewParse(stdout, &config.Target{Name: "test"}, false, log.NewNopLogger())
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(metrics))
		return
	}
	if val := metrics["TEST2DB2-/TEST4"].NotCompleted; val != 0 {
		t.Errorf("Expected 0 notCompleted, got %v", val)
	}
	if val := metrics["TEST2DB2-/TEST4"].Duration; val != 0 {
		t.Errorf("Expected no duration, got %v", val)
	}
	if val := metrics["TEST2DB.DOMAIN-/srv"].Duration; val != 0 {
		t.Errorf("Expected no duration, got %v", val)
	}
	if val := metrics["TEST2DB.DOMAIN-/srv"].ReplicatedBytes; val != 0 {
		t.Errorf("Expected no ReplicatedBytes, got %v", val)
	}
	if val := metrics["TEST2DB2-/TEST4"].ReplicatedFiles; val != 0 {
		t.Errorf("Expected no ReplicatedFiles, got %v", val)
	}
}

func TestReplicationViewsCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	cache := false
	useReplicationViewMetricCache = &cache
	DsmadmcReplicationViewsExec = func(target *config.Target, ctx context.Context, logger log.Logger) (string, error) {
		return mockReplicationViewStdout, nil
	}
	expected := `
    # HELP tsm_exporter_collect_error Indicates if error has occurred during collection
    # TYPE tsm_exporter_collect_error gauge
    tsm_exporter_collect_error{collector="replicationview"} 0
    # HELP tsm_exporter_collect_timeout Indicates the collector timed out
    # TYPE tsm_exporter_collect_timeout gauge
    tsm_exporter_collect_timeout{collector="replicationview"} 0
	# HELP tsm_replication_duration_seconds Amount of time taken to complete the most recent replication
	# TYPE tsm_replication_duration_seconds gauge
	tsm_replication_duration_seconds{fsname="/TEST2CONF",nodename="TEST2DB2"} 19276
	tsm_replication_duration_seconds{fsname="/TEST4",nodename="TEST2DB2"} 19276
	tsm_replication_duration_seconds{fsname="/srv",nodename="TEST2DB.DOMAIN"} 43
	# HELP tsm_replication_not_completed Number of replications not completed for today
	# TYPE tsm_replication_not_completed gauge
	tsm_replication_not_completed{fsname="/TEST2CONF",nodename="TEST2DB2"} 0
	tsm_replication_not_completed{fsname="/TEST4",nodename="TEST2DB2"} 0
	tsm_replication_not_completed{fsname="/srv",nodename="TEST2DB.DOMAIN"} 0
	# HELP tsm_replication_replicated_bytes Amount of data replicated in bytes
	# TYPE tsm_replication_replicated_bytes gauge
	tsm_replication_replicated_bytes{fsname="/TEST2CONF",nodename="TEST2DB2"} 167543418
	tsm_replication_replicated_bytes{fsname="/TEST4",nodename="TEST2DB2"} 1052637876956
	tsm_replication_replicated_bytes{fsname="/srv",nodename="TEST2DB.DOMAIN"} 245650752
	# HELP tsm_replication_replicated_files Number of files replicated
	# TYPE tsm_replication_replicated_files gauge
	tsm_replication_replicated_files{fsname="/TEST2CONF",nodename="TEST2DB2"} 2
	tsm_replication_replicated_files{fsname="/TEST4",nodename="TEST2DB2"} 2
	tsm_replication_replicated_files{fsname="/srv",nodename="TEST2DB.DOMAIN"} 10
	`
	collector := NewReplicationViewsExporter(&config.Target{}, log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 21 {
		t.Errorf("Unexpected collection count %d, expected 21", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"tsm_replication_duration_seconds", "tsm_replication_not_completed",
		"tsm_replication_replicated_bytes", "tsm_replication_replicated_files",
		"tsm_exporter_collect_error", "tsm_exporter_collect_timeout"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestReplicationViewsCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	DsmadmcReplicationViewsExec = func(target *config.Target, ctx context.Context, logger log.Logger) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
    # HELP tsm_exporter_collect_error Indicates if error has occurred during collection
    # TYPE tsm_exporter_collect_error gauge
    tsm_exporter_collect_error{collector="replicationview"} 1
    # HELP tsm_exporter_collect_timeout Indicates the collector timed out
    # TYPE tsm_exporter_collect_timeout gauge
    tsm_exporter_collect_timeout{collector="replicationview"} 0
	`
	collector := NewReplicationViewsExporter(&config.Target{}, log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 3 {
		t.Errorf("Unexpected collection count %d, expected 3", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"tsm_replication_duration_seconds", "tsm_replication_not_completed",
		"tsm_replication_replicated_bytes", "tsm_replication_replicated_files",
		"tsm_exporter_collect_error", "tsm_exporter_collect_timeout"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestReplicationViewsCollectorTimeout(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	DsmadmcReplicationViewsExec = func(target *config.Target, ctx context.Context, logger log.Logger) (string, error) {
		return "", context.DeadlineExceeded
	}
	expected := `
    # HELP tsm_exporter_collect_error Indicates if error has occurred during collection
    # TYPE tsm_exporter_collect_error gauge
    tsm_exporter_collect_error{collector="replicationview"} 0
    # HELP tsm_exporter_collect_timeout Indicates the collector timed out
    # TYPE tsm_exporter_collect_timeout gauge
    tsm_exporter_collect_timeout{collector="replicationview"} 1
	`
	collector := NewReplicationViewsExporter(&config.Target{}, log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 3 {
		t.Errorf("Unexpected collection count %d, expected 3", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"tsm_replication_duration_seconds", "tsm_replication_not_completed",
		"tsm_replication_replicated_bytes", "tsm_replication_replicated_files",
		"tsm_exporter_collect_error", "tsm_exporter_collect_timeout"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestDsmadmcReplicationViews(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := dsmadmcReplicationViews(&config.Target{}, ctx, log.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}
