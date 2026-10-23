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

// Package githubgqlfake provides a hand-written fake implementation of the
// [githubgql.GraphQLClient] interface for table-driven tests. The fake returns
// per-call OIDs or errors defined by the test and records every invocation so
// assertions can verify the expected-head-OID chaining and payload contents
// without a live GraphQL endpoint.
package githubgqlfake

import (
	"context"
	"errors"

	"go.nwlabs.dev/push-signed-commits/internal/githubgql"
)

var (
	// Compile-time assertion that *Fake satisfies the GraphQLClient interface.
	_ githubgql.GraphQLClient = (*Fake)(nil)

	// ErrFakeExhausted indicates the fake was called more times than it had
	// scripted results for.
	ErrFakeExhausted = errors.New("github graphql fake: no scripted result for call")
)

type (
	// Fake is a scripted, hand-written implementation of
	// [githubgql.GraphQLClient]. Each call to CreateCommitOnBranch consumes the
	// next entry from Results and records the input it was called with. When
	// Results is exhausted, it returns ErrFakeExhausted.
	Fake struct {
		// Results holds the scripted outcomes consumed in order, one per call.
		Results []Result

		// Calls records every input the fake was called with, in order.
		Calls []githubgql.CreateCommitInput

		// next is the index of the Results entry to return on the next call.
		next int
	}

	// Result is a single scripted outcome for one CreateCommitOnBranch call:
	// exactly one of OID or Err is meaningful, mirroring the real client's
	// (string, error) return.
	Result struct {
		Err error
		OID string
	}
)

// CreateCommitOnBranch records the input and returns the next scripted result.
// When the scripted results are exhausted, it returns ErrFakeExhausted.
func (f *Fake) CreateCommitOnBranch(_ context.Context, input *githubgql.CreateCommitInput) (string, error) {
	if input != nil {
		f.Calls = append(f.Calls, *input)
	}

	if f.next >= len(f.Results) {
		return "", ErrFakeExhausted
	}

	result := f.Results[f.next]
	f.next++

	return result.OID, result.Err
}
