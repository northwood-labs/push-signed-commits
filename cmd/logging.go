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
	"log/slog"
	"strings"

	"charm.land/log/v2"

	"go.nwlabs.dev/x/logutils"
)

const (
	// levelError names the most restrictive log level, emitting only error
	// events and suppressing warning, informational, and debug output.
	levelError = "ERROR"

	// levelInfo names the informational log level, emitting informational,
	// warning, and error events while suppressing debug output.
	levelInfo = "INFO"

	// levelDebug names the most verbose log level, emitting debug,
	// informational, warning, and error events.
	levelDebug = "DEBUG"

	// verboseWarn is the logutils verbosity that maps to a warning-level logger
	// (informational and debug output suppressed). It is the default for an
	// unrecognized or empty level and for an explicit WARN level.
	verboseWarn = 0

	// verboseInfo is the logutils verbosity that maps to an informational-level
	// logger (debug output suppressed).
	verboseInfo = 1

	// verboseDebug is the logutils verbosity that maps to a debug-level logger
	// (all output emitted).
	verboseDebug = 2
)

// ConfigureLogging installs the process-wide structured logger at the level
// named by logLevel, which is one of DEBUG, INFO, WARN, or ERROR. An
// unrecognized or empty value falls back to WARN so that informational and
// debug output stays suppressed by default. The configured logger is set as
// the slog default, so every slog *Context call in the tool honors the chosen
// level.
func ConfigureLogging(logLevel string) {
	normalized := strings.ToUpper(strings.TrimSpace(logLevel))

	// logutils maps a verbosity integer to a level: 0 = WARN, 1 = INFO, and
	// 2 = DEBUG. ERROR is more restrictive than WARN, so it starts from the
	// warning-level logger and tightens the level directly below. Any
	// unrecognized or empty value falls back to the warning-level default so
	// that informational and debug output stays suppressed.
	verbose := verboseWarn
	tightenToError := false

	switch normalized {
	case levelDebug:
		verbose = verboseDebug
	case levelInfo:
		verbose = verboseInfo
	case levelError:
		tightenToError = true
	default:
		verbose = verboseWarn
	}

	charmLogger := logutils.GetCharmLogger(verbose)

	if tightenToError {
		charmLogger.SetLevel(log.ErrorLevel)
	}

	slog.SetDefault(slog.New(charmLogger))
}
