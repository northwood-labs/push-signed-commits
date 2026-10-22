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

import "context"

type (
	// GraphQLClient creates commits through GitHub's GraphQL API. The
	// production implementation wraps the genqlient-generated
	// createCommitOnBranch function and maps its errors onto the package
	// sentinels; tests substitute a hand-written fake that returns
	// table-defined OIDs or sentinel errors.
	GraphQLClient interface {
		// CreateCommitOnBranch posts the createCommitOnBranch mutation and
		// returns the created commit OID, or a wrapped sentinel error
		// describing the failure.
		CreateCommitOnBranch(ctx context.Context, input *CreateCommitInput) (string, error)
	}

	// CreateCommitInput carries the per-call values into the GraphQLClient
	// wrapper, which maps them onto the genqlient-generated input types. The
	// token is applied by the HTTP transport and is never serialized into the
	// mutation variables.
	CreateCommitInput struct {
		Message                 CommitMessage
		RepositoryNameWithOwner string
		RemoteBranchName        string
		ExpectedHeadOid         string
		FileChanges             FileChanges
	}
)
