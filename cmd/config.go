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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/cobra"
)

const (
	// FlagGithubToken is the name of the --github-token flag.
	FlagGithubToken = "github-token"

	// FlagRepoWithOwner is the name of the --repository-name-with-owner flag.
	FlagRepoWithOwner = "repository-name-with-owner"

	// FlagLocalBranch is the name of the --local-branch-name flag.
	FlagLocalBranch = "local-branch-name"

	// FlagRemoteName is the name of the --remote-name flag.
	FlagRemoteName = "remote-name"

	// FlagRemoteBranch is the name of the --remote-branch-name flag.
	FlagRemoteBranch = "remote-branch-name"

	// FlagLogLevel is the name of the --log-level flag.
	FlagLogLevel = "log-level"

	// DefaultLogLevel is the log level used when --log-level is omitted.
	DefaultLogLevel = "WARN"

	// appName is the application name used to locate configuration files and to
	// derive the environment-variable prefix.
	appName = "push-signed-commits"

	// configFileName is the base name (without extension) of the TOML config
	// file read from the local, user, and global locations.
	configFileName = "config.toml"

	// envPrefix is the prefix for environment variables that override config,
	// for example PUSH_SIGNED_COMMITS_GITHUB_TOKEN maps to github-token.
	envPrefix = "PUSH_SIGNED_COMMITS_"

	// keyDelim is the koanf key-path delimiter. Flag names use dashes rather
	// than dots, so no nesting is introduced by this delimiter.
	keyDelim = "."

	// globalConfigDir is the directory that holds the global configuration file.
	globalConfigDir = "/etc"
)

type (
	// Config holds the resolved input values for one run. Values are populated
	// by koanf in the standard priority order and passed to consumers unaltered.
	Config struct {
		GithubToken             string `koanf:"github-token"`
		RepositoryNameWithOwner string `koanf:"repository-name-with-owner"`
		LocalBranchName         string `koanf:"local-branch-name"`
		RemoteName              string `koanf:"remote-name"`
		RemoteBranchName        string `koanf:"remote-branch-name"`
		LogLevel                string `koanf:"log-level"`
	}
)

// ResolveConfig resolves the six configuration values through koanf in the
// repository's standard priority order (highest priority first): CLI flag,
// environment variable, local config file, user config file, global config
// file. A higher-priority source always overrides a lower-priority one. Flag
// string values are carried into the returned Config without transformation.
func ResolveConfig(cmd *cobra.Command) (*Config, error) {
	k := koanf.New(keyDelim)

	// Load lowest priority first so that higher-priority sources, loaded later,
	// override earlier values. Missing config files are not an error.
	for _, path := range configFilePaths() {
		err := loadConfigFile(k, path)
		if err != nil {
			return nil, fmt.Errorf("failed to load configuration: %w", err)
		}
	}

	err := k.Load(env.Provider(keyDelim, env.Opt{
		Prefix:        envPrefix,
		TransformFunc: transformEnvKey,
	}), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment variables: %w", err)
	}

	// CLI flags have the highest priority. Passing the koanf instance lets
	// posflag ignore unset flags whose defaults would otherwise clobber values
	// already loaded from config files or the environment.
	err = k.Load(posflag.Provider(cmd.Flags(), keyDelim, k), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read command-line flags: %w", err)
	}

	cfg := &Config{}

	err = k.Unmarshal("", cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve configuration: %w", err)
	}

	return cfg, nil
}

// configFilePaths returns the config file locations in ascending priority
// order: global config, then user config, then local config. A later entry
// overrides an earlier one when both exist.
func configFilePaths() []string {
	paths := []string{
		filepath.Join(globalConfigDir, appName, configFileName),
	}

	userConfigDir := userConfigBase()
	if userConfigDir != "" {
		paths = append(paths, filepath.Join(userConfigDir, appName, configFileName))
	}

	paths = append(paths, configFileName)

	return paths
}

// userConfigBase returns the base directory for user configuration, honoring
// $XDG_CONFIG_HOME and falling back to $HOME/.config per the XDG Base
// Directory Specification. It returns an empty string when neither is set.
func userConfigBase() string {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg != "" {
		return xdg
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config")
}

// loadConfigFile loads a single TOML config file into k. A file that does not
// exist is skipped without error; any other read or parse failure is returned.
func loadConfigFile(k *koanf.Koanf, path string) error {
	_, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return nil
		}

		return fmt.Errorf("failed to read configuration file %q: %w", path, statErr)
	}

	err := k.Load(file.Provider(path), toml.Parser())
	if err != nil {
		return fmt.Errorf("failed to read configuration file %q: %w", path, err)
	}

	return nil
}

// transformEnvKey maps a prefixed environment variable name to its config key.
// It strips the envPrefix, lowercases the remainder, and converts underscores
// to dashes so that PUSH_SIGNED_COMMITS_GITHUB_TOKEN becomes github-token.
func transformEnvKey(key, value string) (string, any) {
	trimmed := strings.TrimPrefix(key, envPrefix)
	normalized := strings.ReplaceAll(strings.ToLower(trimmed), "_", "-")

	return normalized, value
}
