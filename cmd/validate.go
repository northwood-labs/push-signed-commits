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
	"strings"
)

const (
	// prefixOrigin is the prohibited "origin/" prefix on a branch name.
	prefixOrigin = "origin/"

	// prefixRefsHeads is the prohibited "refs/heads/" prefix on a branch name.
	prefixRefsHeads = "refs/heads/"
)

type (
	// BranchNameInput carries the values ValidateBranchName needs. BranchType
	// is "local" or "remote" and appears only in the error message; RemoteName
	// is the configured remote whose "<RemoteName>/" prefix is also prohibited.
	BranchNameInput struct {
		BranchName string
		BranchType string
		RemoteName string
	}
)

// ValidateBranchName rejects a branch name that includes a ref prefix or the
// remote name. It returns an error wrapping [ErrInvalidBranchName] when the
// name begins with "origin/", "refs/heads/", or "<RemoteName>/", naming the
// offending prefix and whether the local or remote branch name was at fault. A
// name without any prohibited prefix returns nil.
func ValidateBranchName(opts *BranchNameInput) error {
	if strings.HasPrefix(opts.BranchName, prefixOrigin) {
		return fmt.Errorf(
			"%w: do not include %q in the %s branch name",
			ErrInvalidBranchName,
			prefixOrigin,
			opts.BranchType,
		)
	}

	if strings.HasPrefix(opts.BranchName, prefixRefsHeads) {
		return fmt.Errorf(
			"%w: do not include %q in the %s branch name",
			ErrInvalidBranchName,
			prefixRefsHeads,
			opts.BranchType,
		)
	}

	if strings.HasPrefix(opts.BranchName, opts.RemoteName+"/") {
		return fmt.Errorf(
			"%w: do not include the remote name in the %s branch name",
			ErrInvalidBranchName,
			opts.BranchType,
		)
	}

	return nil
}
