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
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/treydock/tsm_exporter/config"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	mockEventCompletedStdout = `
FOO,2020-03-22 05:09:43.000000,2020-03-22 05:41:14.000000
FOO,2020-03-21 05:09:43.000000,2020-03-21 05:39:14.000000
FOO,2020-03-20 05:09:43.000000,2020-03-20 05:40:14.000000
`
	mockEventNotCompletedStdout = `
FOO,Future
BAR,Not Started
BAR,Not Started
`
)

func TestBuildEventsCompletedQuery(t *testing.T) {
	expectedQuery := "SELECT schedule_name, actual_start, completed FROM events WHERE status = 'Completed' ORDER BY completed DESC"
	query := buildEventsCompletedQuery(&config.Target{Name: "test"})
	if query != expectedQuery {
		t.Errorf("\nExpected: %s\nGot: %s", expectedQuery, query)
	}
	expectedQuery = "SELECT schedule_name, actual_start, completed FROM events WHERE schedule_name IN ('FOO','BAR') AND status = 'Completed' ORDER BY completed DESC"
	query = buildEventsCompletedQuery(&config.Target{Name: "test", Schedules: []string{"FOO", "BAR"}})
	if query != expectedQuery {
		t.Errorf("\nExpected: %s\nGot: %s", expectedQuery, query)
	}
}

func TestBuildEventsNotCompletedQuery(t *testing.T) {
	mockNow, _ := time.Parse("01/02/2006 15:04:05", "07/02/2020 13:00:00")
	timeNow = func() time.Time {
		return mockNow
	}
	expectedQuery := "SELECT schedule_name,status FROM events WHERE DATE(scheduled_start) BETWEEN '2020-07-01' AND '2020-07-02'"
	query := buildEventsNotCompletedQuery(&config.Target{Name: "test"})
	if query != expectedQuery {
		t.Errorf("Expected: %s\nGot: %s", expectedQuery, query)
	}
	expectedQuery = "SELECT schedule_name,status FROM events WHERE schedule_name IN ('FOO','BAR') AND DATE(scheduled_start) BETWEEN '2020-07-01' AND '2020-07-02'"
	query = buildEventsNotCompletedQuery(&config.Target{Name: "test", Schedules: []string{"FOO", "BAR"}})
	if query != expectedQuery {
		t.Errorf("Expected: %s\nGot: %s", expectedQuery, query)
	}
}

func TestEventsParse(t *testing.T) {
	metrics := eventsParse(mockEventCompletedStdout, mockEventNotCompletedStdout, log.NewNopLogger())
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(metrics))
		return
	}
	if val := metrics["FOO"].notCompleted; val != 0 {
		t.Errorf("Expected 0 notCompleted, got %v", val)
	}
	if val := metrics["FOO"].duration; val != 1891 {
		t.Errorf("Expected 1891 duration, got %v", val)
	}
	if val := metrics["BAR"].notCompleted; val != 2 {
		t.Errorf("Expected 1 notCompleted, got %v", val)
	}
}

func TestEventsCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	DsmadmcEventsCompletedExec = func(target *config.Target, logger log.Logger) (string, error) {
		return mockEventCompletedStdout, nil
	}
	DsmadmcEventsNotCompletedExec = func(target *config.Target, logger log.Logger) (string, error) {
		return mockEventNotCompletedStdout, nil
	}
	expected := `
    # HELP tsm_exporter_collect_error Indicates if error has occurred during collection
    # TYPE tsm_exporter_collect_error gauge
    tsm_exporter_collect_error{collector="events"} 0
	# HELP tsm_schedule_duration_seconds Amount of time taken to complete the most recent completed scheduled event
	# TYPE tsm_schedule_duration_seconds gauge
	tsm_schedule_duration_seconds{schedule="BAR"} 0
	tsm_schedule_duration_seconds{schedule="FOO"} 1891
    # HELP tsm_schedule_not_completed Number of scheduled events not completed for today
    # TYPE tsm_schedule_not_completed gauge
    tsm_schedule_not_completed{schedule="BAR"} 2
    tsm_schedule_not_completed{schedule="FOO"} 0
	`
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	collector := NewEventsExporter(&config.Target{}, logger)
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 6 {
		t.Errorf("Unexpected collection count %d, expected 6", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"tsm_schedule_not_completed", "tsm_schedule_duration_seconds",
		"tsm_exporter_collect_error"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestEventsCollectorError(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	DsmadmcEventsCompletedExec = func(target *config.Target, logger log.Logger) (string, error) {
		return "", fmt.Errorf("Error")
	}
	DsmadmcEventsNotCompletedExec = func(target *config.Target, logger log.Logger) (string, error) {
		return "", fmt.Errorf("Error")
	}
	expected := `
    # HELP tsm_exporter_collect_error Indicates if error has occurred during collection
    # TYPE tsm_exporter_collect_error gauge
    tsm_exporter_collect_error{collector="events"} 1
	`
	collector := NewEventsExporter(&config.Target{}, log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 2 {
		t.Errorf("Unexpected collection count %d, expected 2", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected),
		"tsm_schedule_not_completed", "tsm_schedule_duration_seconds",
		"tsm_exporter_collect_error"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

/*func TestDsmadmcEventsCompleted(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := dsmadmcEventsCompleted(&config.Target{}, ctx, log.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}

func TestDsmadmcEventsNotCompleted(t *testing.T) {
	execCommand = fakeExecCommand
	mockedExitStatus = 0
	mockedStdout = "foo"
	defer func() { execCommand = exec.CommandContext }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := dsmadmcEventsNotCompleted(&config.Target{}, ctx, log.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if out != mockedStdout {
		t.Errorf("Unexpected out: %s", out)
	}
}*/
