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

	"go.nwlabs.dev/x/logutils"
)

const (
	// verboseWarn is the logutils verbosity that maps to a warning-level logger
	// (info and debug output suppressed). It is the default when --verbose is
	// omitted.
	verboseWarn = 0

	// verboseInfo is the logutils verbosity that maps to an info-level logger
	// (debug output suppressed).
	verboseInfo = 1

	// verboseDebug is the logutils verbosity that maps to a debug-level logger
	// (all output emitted).
	verboseDebug = 2
)

// ConfigureLogging installs the process-wide structured logger at the level
// named by the verbose count: 0 is WARN (quiet; info and debug suppressed), 1
// is INFO, and 2 or more is DEBUG (clamped). A negative count is treated as 0.
//
// The configured logger is set as the slog default, so every slog *Context call
// in the tool honors the chosen level.
func ConfigureLogging(verbose int) {
	// logutils maps a verbosity integer to a level: 0 = WARN, 1 = INFO, and 2 =
	// DEBUG. Counts of 2 or more clamp to DEBUG; a negative count falls back to
	// the warning-level default so that informational and debug output stays
	// suppressed.
	var resolved int

	switch {
	case verbose <= 0:
		resolved = verboseWarn
	case verbose == 1:
		resolved = verboseInfo
	default:
		resolved = verboseDebug
	}

	charmLogger := logutils.GetCharmLogger(resolved)

	slog.SetDefault(slog.New(charmLogger))
}
