package command

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

func newCommand(ctx context.Context, t *testing.T, executable string, options CreateOptions) (*mockCommander, *process.MockKillWaiter, Command, func()) {
	commanderMock := new(mockCommander)
	processKillWaiterMock := new(process.MockKillWaiter)

	oldNewCmd := newCmd
	oldNewProcessKillWaiter := newProcessKillWaiter

	cleanup := func() {
		newCmd = oldNewCmd
		newProcessKillWaiter = oldNewProcessKillWaiter

		commanderMock.AssertExpectations(t)
		processKillWaiterMock.AssertExpectations(t)
	}

	newCmd = func(executable string, args []string, options CreateOptions) commander {
		return commanderMock
	}

	newProcessKillWaiter = func(process.Logger, time.Duration, time.Duration) process.KillWaiter {
		return processKillWaiterMock
	}

	c := New(ctx, executable, []string{}, options)

	return commanderMock, processKillWaiterMock, c, cleanup
}

func TestCommand_Run(t *testing.T) {
	testErr := errors.New("test error")

	tests := map[string]struct {
		cmdStartErr       error
		cmdWaitErr        error
		getExitCode       func(err *exec.ExitError) int
		contextClosed     bool
		process           *os.Process
		expectedError     string
		expectedErrorType interface{}
	}{
		"error on cmd start()": {
			cmdStartErr:   errors.New("test-error"),
			expectedError: "failed to start command: test-error",
		},
		"command ends with a build failure": {
			cmdWaitErr:        &exec.ExitError{ProcessState: &os.ProcessState{}},
			getExitCode:       func(err *exec.ExitError) int { return BuildFailureExitCode },
			expectedError:     "exit status 0",
			expectedErrorType: &common.BuildError{},
		},
		"command ends with a system failure": {
			cmdWaitErr:        &exec.ExitError{ProcessState: &os.ProcessState{}},
			getExitCode:       func(err *exec.ExitError) int { return SystemFailureExitCode },
			expectedError:     "exit status 0",
			expectedErrorType: &exec.ExitError{},
		},
		"command ends with a unknown failure": {
			cmdWaitErr:        &exec.ExitError{ProcessState: &os.ProcessState{}},
			getExitCode:       func(err *exec.ExitError) int { return 255 },
			expectedError:     "unknown Custom executor executable exit code 255; executable execution terminated with: exit status 0",
			expectedErrorType: &ErrUnknownFailure{},
		},
		"command times out": {
			contextClosed: true,
			process:       &os.Process{Pid: 1234},
			expectedError: testErr.Error(),
		},
	}

	for testName, tt := range tests {
		tt := tt
		t.Run(testName, func(t *testing.T) {
			ctx, ctxCancel := context.WithCancel(context.Background())
			defer ctxCancel()

			options := CreateOptions{
				Logger:              common.NewBuildLogger(nil, logrus.NewEntry(logrus.New())),
				GracefulKillTimeout: 100 * time.Millisecond,
				ForceKillTimeout:    100 * time.Millisecond,
			}

			commanderMock, processKillWaiterMock, c, cleanup := newCommand(ctx, t, "exec", options)
			defer cleanup()

			commanderMock.On("Start").
				Return(tt.cmdStartErr)
			commanderMock.On("Wait").
				Return(func() error {
					<-time.After(500 * time.Millisecond)
					return tt.cmdWaitErr
				}).
				Maybe()

			if tt.getExitCode != nil {
				oldGetExitCode := getExitCode
				defer func() {
					getExitCode = oldGetExitCode
				}()
				getExitCode = tt.getExitCode
			}

			if tt.contextClosed {
				ctxCancel()
				commanderMock.On("Process").
					Return(tt.process)

				processKillWaiterMock.
					On("KillAndWait", tt.process, mock.Anything).
					Return(testErr).
					Once()
			}

			err := c.Run()

			if tt.expectedError == "" {
				assert.NoError(t, err)

				return
			}

			assert.EqualError(t, err, tt.expectedError)
			if tt.expectedErrorType != nil {
				assert.IsType(t, tt.expectedErrorType, err)
			}
		})
	}
}
