---
# @config-manager:start cli-patterns-frontmatter
inclusion: auto
name: go-cli-application
description: Patterns to use when creating or updating a Go-based CLI or TUI application.
# @config-manager:end cli-patterns-frontmatter
---

<!-- @config-manager:start cli-patterns -->
# Go CLI Tool Patterns

This repository is a Go CLI tool. Follow these conventions when writing or modifying code.

For general-purpose Go coding conventions (linting, error handling, style rules, suppression comments, etc.), refer to `.kiro/steering/go-code-conventions.md`. This document covers only the CLI-specific project structure and patterns.

## Project structure

* `main.go` — minimal entrypoint; calls `cmd.Execute()` and nothing else.
* `cmd/` — Cobra command definitions. One file per command (e.g., `root.go`, `serve.go`).
* Module path uses a vanity import domain (e.g., `go.nwlabs.dev/<project>`).

## License header

Every `.go` file must begin with the Apache 2.0 copyright header:

```go
// Copyright <year>, Northwood Labs, LLC <license@northwood-labs.com>
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
```

Use the current year for new files.

## CLI framework

* Use `github.com/spf13/cobra` for command structure.
* Use `charm.land/fang/v2` to wrap Cobra execution for terminal color support.
* Use `github.com/knadh/koanf` instead of `github.com/spf13/viper` for CLI configuration.
* Use `go.nwlabs.dev/cli-helpers/v2` for shared CLI helpers (e.g., `clihelpers.LongHelpText()`) and standardized colors.
  * Import the cli-helpers package with the alias `clihelpers`.
* Use `go.nwlabs.dev/x/logutils` for logging, and pass the "verbose" flag (e.g., `fVerbosity`) as the parameter.
* Use `charm.land/lipgloss/v2` for general purpose terminal coloring.
* Use the `charm.land/<library>/v2` import variants instead of the older `github.com/charmbracelet/` imports.

## Configuration management

* Use `github.com/knadh/koanf` for configuration management. Do not use `github.com/spf13/viper`.
* Store user configuration files in `$XDG_CONFIG_HOME`. When that environment variable is not set, fall back to the default defined by the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/) (`$HOME/.config`).
* Store global configuration files in `/etc/<app>/`.

When a CLI tool accepts configuration from multiple sources, resolve values in this order (highest priority first):

1. CLI flag
2. Environment variable
3. Local config file
4. User config file (in `$XDG_CONFIG_HOME/<app>/`)
5. Global config file (in `/etc/<app>/`)

A higher-priority source always overrides a lower-priority one.

## Command conventions

* Define each command as a `var` block with a `*cobra.Command` struct.
* CLI flag variables are prefixed with lowercase `f` (e.g., `fToggle`, `fVerbose`).
* Use `init()` to register flags and subcommands. Mark every `init()` function with a `// lint:allow_init` comment to suppress `gochecknoinits`.
* Long help text uses `clihelpers.LongHelpText()` with a dedented raw string.
* `Execute()` takes a `context.Background()` and passes it through `fang.Execute()`.
* Exit with `os.Exit(1)` on error from `Execute()`.

## Import organization

Group imports in this order, separated by blank lines:

1. Standard library packages
2. Third-party packages
3. Internal/project packages

Use named imports only when needed to avoid collisions (e.g., `clihelpers "go.nwlabs.dev/cli-helpers/v2"`).

## Version command

Every CLI tool must include a `cmd/version.go` file that provides a `version` subcommand. It always follows this exact pattern:

```go
package cmd

import clihelpers "go.nwlabs.dev/cli-helpers/v2"

var versionCmd = clihelpers.VersionScreen()

func init() { // lint:allow_init
	rootCmd.AddCommand(versionCmd)
}
```

Key points:

* The version screen is provided by `clihelpers.VersionScreen()` — do not implement custom version logic.
* The command variable is named `versionCmd`.
* It is registered on `rootCmd` in `init()`.
* No other imports are needed beyond the `clihelpers` alias.

## Package documentation

Every package must have a `doc.go` file containing the package-level documentation comment. The file follows this structure:

```go
// Copyright <year>, Northwood Labs, LLC <license@northwood-labs.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// ...full license header...

// Package <name> implements ...
package <name>
```

* The `// Package <name>` comment is the only code in `doc.go` — no imports, no declarations.
* The comment must start with `Package <name>` followed by a brief description of the package's purpose.
* No other file in the package should contain a `package`-level doc comment.
<!-- @config-manager:end cli-patterns -->
