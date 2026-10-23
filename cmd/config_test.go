// Copyright 2026, Northwood Labs, LLC <license@northwood-labs.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/testify/assert"
	"github.com/go-openapi/testify/require"
	"github.com/spf13/cobra"
)

// newTestCommand builds a command whose flags mirror the real root command so
// that each test case resolves configuration against isolated flag state.
func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: appName}
	flags := cmd.Flags()

	flags.String(FlagOwnerRepo, "", "")
	flags.String(FlagLocalBranch, "", "")
	flags.String(FlagRemoteName, "", "")
	flags.String(FlagRemoteBranch, "", "")
	flags.CountP(FlagVerbose, "v", "")

	return cmd
}

func TestResolveConfigPriority(t *testing.T) {
	testCases := []struct {
		env      map[string]string
		expected *Config
		name     string
		fileBody string
		args     []string
	}{
		{
			name: "flags only with default log level",
			args: []string{
				"--owner-repo", "owner/repo",
				"--local-branch", "feature",
				"--remote-name", "origin",
				"--remote-branch", "main",
			},
			expected: &Config{
				OwnerRepo:    "owner/repo",
				LocalBranch:  "feature",
				RemoteName:   "origin",
				RemoteBranch: "main",
			},
		},
		{
			name: "explicit verbose flag overrides default",
			args: []string{
				"--owner-repo", "owner/repo",
				"--local-branch", "feature",
				"--remote-name", "origin",
				"--remote-branch", "main",
				"-vv",
			},
			expected: &Config{
				OwnerRepo:    "owner/repo",
				LocalBranch:  "feature",
				RemoteName:   "origin",
				RemoteBranch: "main",
				Verbose:      2,
			},
		},
		{
			name: "environment variable fills unset flag",
			args: []string{
				"--owner-repo", "owner/repo",
				"--local-branch", "feature",
				"--remote-name", "origin",
				"--remote-branch", "main",
			},
			env: map[string]string{
				envPrefix + "VERBOSE": "1",
			},
			expected: &Config{
				OwnerRepo:    "owner/repo",
				LocalBranch:  "feature",
				RemoteName:   "origin",
				RemoteBranch: "main",
				Verbose:      1,
			},
		},
		{
			name: "flag overrides environment variable",
			args: []string{
				"--owner-repo", "owner/repo",
				"--local-branch", "feature",
				"--remote-name", "origin",
				"--remote-branch", "main",
				"-vv",
			},
			env: map[string]string{
				envPrefix + "VERBOSE": "1",
			},
			expected: &Config{
				OwnerRepo:    "owner/repo",
				LocalBranch:  "feature",
				RemoteName:   "origin",
				RemoteBranch: "main",
				Verbose:      2,
			},
		},
		{
			name: "local config file fills unset values",
			args: []string{
				"--owner-repo", "owner/repo",
				"--remote-name", "origin",
			},
			fileBody: "local-branch = \"from-file\"\n" +
				"remote-branch = \"release\"\n" +
				"verbose = 2\n",
			expected: &Config{
				OwnerRepo:    "owner/repo",
				LocalBranch:  "from-file",
				RemoteName:   "origin",
				RemoteBranch: "release",
				Verbose:      2,
			},
		},
		{
			name: "env overrides local config file",
			args: []string{
				"--owner-repo", "owner/repo",
				"--local-branch", "feature",
				"--remote-name", "origin",
				"--remote-branch", "main",
			},
			env: map[string]string{
				envPrefix + "VERBOSE": "2",
			},
			fileBody: "verbose = 1\n",
			expected: &Config{
				OwnerRepo:    "owner/repo",
				LocalBranch:  "feature",
				RemoteName:   "origin",
				RemoteBranch: "main",
				Verbose:      2,
			},
		},
		{
			name: "flag values are passed through unaltered",
			args: []string{
				"--owner-repo", "Owner/Repo.Name",
				"--local-branch", "feature/ABC-123",
				"--remote-name", "upstream",
				"--remote-branch", "main",
			},
			expected: &Config{
				OwnerRepo:    "Owner/Repo.Name",
				LocalBranch:  "feature/ABC-123",
				RemoteName:   "upstream",
				RemoteBranch: "main",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			isolateConfigEnvironment(t, testCase.fileBody)

			for key, value := range testCase.env {
				t.Setenv(key, value)
			}

			cmd := newTestCommand()

			parseErr := cmd.ParseFlags(testCase.args)
			require.NoError(t, parseErr)

			cfg, err := ResolveConfig(cmd)
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, cfg)
		})
	}
}

// isolateConfigEnvironment points the user and local config lookups at an empty
// temporary directory and, when fileBody is non-empty, writes it to the local
// config file so a test case can exercise the file-backed resolution path.
func isolateConfigEnvironment(t *testing.T, fileBody string) {
	t.Helper()

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "xdg"))

	// t.Chdir restores the previous working directory automatically when the
	// test finishes, so the local config.toml lookup resolves inside tempDir.
	t.Chdir(tempDir)

	if fileBody == "" {
		return
	}

	writeErr := os.WriteFile(filepath.Join(tempDir, configFileName), []byte(fileBody), 0o0666) // lint:allow_666
	require.NoError(t, writeErr)
}
