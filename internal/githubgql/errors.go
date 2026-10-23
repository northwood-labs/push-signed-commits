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

package githubgql

import "errors"

var (
	// ErrGithubAPI indicates that GitHub's GraphQL API returned an unexpected
	// or error response while creating a commit. The cmd package compares
	// against its own boundary sentinel via [errors.Is].
	ErrGithubAPI = errors.New("GitHub API returned an unexpected response")

	// ErrPartialPushFailure indicates that the expected head of the remote
	// branch is stale, so the push was aborted and may be only partially
	// applied. The cmd package compares against its own boundary sentinel via
	// [errors.Is].
	ErrPartialPushFailure = errors.New("push completed only partially")
)
