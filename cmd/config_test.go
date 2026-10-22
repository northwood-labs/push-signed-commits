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

	flags.String(FlagGithubToken, "", "")
	flags.String(FlagRepoWithOwner, "", "")
	flags.String(FlagLocalBranch, "", "")
	flags.String(FlagRemoteName, "", "")
	flags.String(FlagRemoteBranch, "", "")
	flags.String(FlagLogLevel, DefaultLogLevel, "")

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
				"--github-token", "flag-token",
				"--repository-name-with-owner", "owner/repo",
				"--local-branch-name", "feature",
				"--remote-name", "origin",
				"--remote-branch-name", "main",
			},
			expected: &Config{
				GithubToken:             "flag-token",
				RepositoryNameWithOwner: "owner/repo",
				LocalBranchName:         "feature",
				RemoteName:              "origin",
				RemoteBranchName:        "main",
				LogLevel:                DefaultLogLevel,
			},
		},
		{
			name: "explicit log level flag overrides default",
			args: []string{
				"--github-token", "flag-token",
				"--repository-name-with-owner", "owner/repo",
				"--local-branch-name", "feature",
				"--remote-name", "origin",
				"--remote-branch-name", "main",
				"--log-level", "DEBUG",
			},
			expected: &Config{
				GithubToken:             "flag-token",
				RepositoryNameWithOwner: "owner/repo",
				LocalBranchName:         "feature",
				RemoteName:              "origin",
				RemoteBranchName:        "main",
				LogLevel:                "DEBUG",
			},
		},
		{
			name: "environment variable fills unset flag",
			args: []string{
				"--repository-name-with-owner", "owner/repo",
				"--local-branch-name", "feature",
				"--remote-name", "origin",
				"--remote-branch-name", "main",
			},
			env: map[string]string{
				envPrefix + "GITHUB_TOKEN": "env-token",
				envPrefix + "LOG_LEVEL":    "INFO",
			},
			expected: &Config{
				GithubToken:             "env-token",
				RepositoryNameWithOwner: "owner/repo",
				LocalBranchName:         "feature",
				RemoteName:              "origin",
				RemoteBranchName:        "main",
				LogLevel:                "INFO",
			},
		},
		{
			name: "flag overrides environment variable",
			args: []string{
				"--github-token", "flag-token",
				"--repository-name-with-owner", "owner/repo",
				"--local-branch-name", "feature",
				"--remote-name", "origin",
				"--remote-branch-name", "main",
			},
			env: map[string]string{
				envPrefix + "GITHUB_TOKEN": "env-token",
			},
			expected: &Config{
				GithubToken:             "flag-token",
				RepositoryNameWithOwner: "owner/repo",
				LocalBranchName:         "feature",
				RemoteName:              "origin",
				RemoteBranchName:        "main",
				LogLevel:                DefaultLogLevel,
			},
		},
		{
			name: "local config file fills unset values",
			args: []string{
				"--github-token", "flag-token",
				"--repository-name-with-owner", "owner/repo",
				"--remote-name", "origin",
			},
			fileBody: "local-branch-name = \"from-file\"\n" +
				"remote-branch-name = \"release\"\n" +
				"log-level = \"ERROR\"\n",
			expected: &Config{
				GithubToken:             "flag-token",
				RepositoryNameWithOwner: "owner/repo",
				LocalBranchName:         "from-file",
				RemoteName:              "origin",
				RemoteBranchName:        "release",
				LogLevel:                "ERROR",
			},
		},
		{
			name: "env overrides local config file",
			args: []string{
				"--github-token", "flag-token",
				"--repository-name-with-owner", "owner/repo",
				"--local-branch-name", "feature",
				"--remote-name", "origin",
				"--remote-branch-name", "main",
			},
			env: map[string]string{
				envPrefix + "LOG_LEVEL": "INFO",
			},
			fileBody: "log-level = \"ERROR\"\n",
			expected: &Config{
				GithubToken:             "flag-token",
				RepositoryNameWithOwner: "owner/repo",
				LocalBranchName:         "feature",
				RemoteName:              "origin",
				RemoteBranchName:        "main",
				LogLevel:                "INFO",
			},
		},
		{
			name: "flag values are passed through unaltered",
			args: []string{
				"--github-token", "  spaced token  ",
				"--repository-name-with-owner", "Owner/Repo.Name",
				"--local-branch-name", "feature/ABC-123",
				"--remote-name", "upstream",
				"--remote-branch-name", "main",
			},
			expected: &Config{
				GithubToken:             "  spaced token  ",
				RepositoryNameWithOwner: "Owner/Repo.Name",
				LocalBranchName:         "feature/ABC-123",
				RemoteName:              "upstream",
				RemoteBranchName:        "main",
				LogLevel:                DefaultLogLevel,
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
