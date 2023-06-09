//go:build integration

package shells

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func TestPowershellUTF8EncodingStdin(t *testing.T) {
	for _, shell := range []string{SNPowershell, SNPwsh} {
		t.Run(shell, func(t *testing.T) {
			helpers.SkipIntegrationTests(t, shell)

			cmd := exec.Command(shell, stdinCmdArgs(shell)...)

			buf := new(bytes.Buffer)
			// script to detect regression based on https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29160
			cmd.Stdin = strings.NewReader(`& { $Q_Test_ = '∅'; Write-Host "Actual: $($Q_Test_) $(($Q_Test_ | Format-Hex).Bytes -join ', ')" }`)
			cmd.Stdout = buf
			cmd.Stderr = buf

			require.NoError(t, cmd.Run())

			switch shell {
			case SNPwsh:
				require.Contains(t, buf.String(), "Actual: ∅ 226, 136, 133")
			case SNPowershell:
				// Ideally, we'd have the same output as Powershell Core, however
				// there doesn't appear to be a way to set the input encoding for
				// Windows Powershell that would correct this.
				require.Contains(t, buf.String(), "Actual: ∅ 63")
			}
		})
	}
}
