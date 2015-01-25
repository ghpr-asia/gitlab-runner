package src

import (
	"bytes"
	"errors"
	"time"

	log "github.com/Sirupsen/logrus"
)

type Job struct {
	Build  *Build
	Runner *RunnerConfig
	Finish chan *Job
}

func (j *Job) fail(config RunnerConfig, build Build, err error) {
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

func (j *Job) Run() error {
	executor := GetExecutor(*j.Runner)
	if executor == nil {
		err := errors.New("couldn't get executor")
		j.Finish <- j
		failBuild(*j.Runner, *j.Build, err)
		return err
	}

	err := executor.Run(*j.Runner, *j.Build)
	if err != nil {
		j.Finish <- j
		failBuild(*j.Runner, *j.Build, err)
		return err
	}

	// notify about job finish
	j.Finish <- j
	return nil
}
