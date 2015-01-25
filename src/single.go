package src

import (
	"bytes"
	"github.com/codegangsta/cli"
	"time"

	log "github.com/Sirupsen/logrus"
)

func failBuild(config RunnerConfig, build Build, err error) {
	log.Println(config.ShortDescription(), build.Id, "Build failed", err)
	for {
		error_buffer := bytes.NewBufferString(err.Error())
		result := UpdateBuild(config, build.Id, Failed, error_buffer)
		switch result {
		case UpdateSucceeded:
			return
		case UpdateAbort:
			return
		case UpdateFailed:
			time.Sleep(UPDATE_RETRY_INTERVAL * time.Second)
			continue
		}
	}
}

func runSingle(c *cli.Context) {
	runner_config := RunnerConfig{
		URL:   c.String("URL"),
		Token: c.String("token"),
	}

	log.Println("Starting runner for", runner_config.URL, "with token", runner_config.ShortDescription(), "...")

	for {
		new_build := GetBuild(runner_config)
		if new_build == nil {
			time.Sleep(CHECK_INTERVAL * time.Second)
			continue
		}

		new_job := Job{
			Build:  &Build{*new_build},
			Runner: &runner_config,
		}

		go new_job.Run()
		<-new_job.Finish
	}
}
