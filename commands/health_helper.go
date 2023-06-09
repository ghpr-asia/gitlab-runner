package commands

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type healthData struct {
	failures  int
	lastCheck time.Time
}

type healthHelper struct {
	healthy     map[string]*healthData
	healthyLock sync.Mutex

	healthCheckFailures *prometheus.CounterVec
}

func newHealthHelper() healthHelper {
	return healthHelper{
		healthCheckFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_worker_health_check_failures_total",
				Help: "Total number of runner worker health check failures",
			},
			[]string{"runner", "runner_name", "system_id"},
		),
	}
}

func (mr *healthHelper) getHealth(id string) *healthData {
	if mr.healthy == nil {
		mr.healthy = map[string]*healthData{}
	}

	health := mr.healthy[id]
	if health == nil {
		health = &healthData{
			lastCheck: time.Now(),
		}
		mr.healthy[id] = health
	}

	return health
}

func (mr *healthHelper) isHealthy(runner *common.RunnerConfig) bool {
	mr.healthyLock.Lock()
	defer mr.healthyLock.Unlock()

	mr.runnerHealthCheckFailures(runner).Add(0)

	id := runner.UniqueID()
	health := mr.getHealth(id)
	if health.failures < runner.GetUnhealthyRequestsLimit() {
		return true
	}

	if time.Since(health.lastCheck) > runner.GetUnhealthyInterval() {
		logrus.WithFields(logrus.Fields{
			"unhealthy_requests":       health.failures,
			"unhealthy_requests_limit": runner.GetUnhealthyRequestsLimit(),
			"unhealthy_interval":       runner.GetUnhealthyInterval(),
		}).Warningf("Runner %q is not healthy, but check for a new job will be forced!", id)

		health.failures = 0
		health.lastCheck = time.Now()
		return true
	}

	return false
}

func (mr *healthHelper) runnerHealthCheckFailures(runner *common.RunnerConfig) prometheus.Counter {
	return mr.healthCheckFailures.WithLabelValues(runner.ShortDescription(), runner.Name, runner.GetSystemID())
}

func (mr *healthHelper) markHealth(runner *common.RunnerConfig, healthy bool) {
	mr.healthyLock.Lock()
	defer mr.healthyLock.Unlock()

	id := runner.UniqueID()

	health := mr.getHealth(id)
	if healthy {
		health.failures = 0
		health.lastCheck = time.Now()
		return
	}

	mr.runnerHealthCheckFailures(runner).Inc()

	health.failures++
	if health.failures >= runner.GetUnhealthyRequestsLimit() {
		logrus.WithFields(logrus.Fields{
			"unhealthy_requests":       health.failures,
			"unhealthy_requests_limit": runner.GetUnhealthyRequestsLimit(),
		}).Errorf(
			"Runner %q is unhealthy and will be disabled for %s seconds!",
			id,
			runner.GetUnhealthyInterval(),
		)
	}
}

func (mr *healthHelper) Describe(ch chan<- *prometheus.Desc) {
	mr.healthCheckFailures.Describe(ch)
}

func (mr *healthHelper) Collect(ch chan<- prometheus.Metric) {
	mr.healthCheckFailures.Collect(ch)
}
