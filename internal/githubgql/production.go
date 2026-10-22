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

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const (
	// graphqlEndpoint is GitHub's GraphQL API endpoint.
	graphqlEndpoint = "https://api.github.com/graphql"

	// requestTimeout bounds every GraphQL request, matching the Python tool's
	// 600-second HTTP timeout.
	requestTimeout = 600 * time.Second

	// staleDataType is the GraphQL error extension type that indicates the
	// expected head OID is stale because the remote branch moved underneath us.
	staleDataType = "STALE_DATA"
)

// Compile-time assertion that *Client satisfies the GraphQLClient interface.
var _ GraphQLClient = (*Client)(nil)

type (
	// Client is the production [GraphQLClient]. It wraps the genqlient-generated
	// createCommitOnBranch operation with an authenticating HTTP transport and
	// maps transport and GraphQL errors onto the package sentinels, preserving
	// the STALE_DATA distinction used to detect a partial push.
	Client struct {
		gql graphql.Client
	}

	// bearerTransport is an [http.RoundTripper] that attaches an
	// "Authorization: Bearer <token>" header to every outgoing request so the
	// token never appears in the serialized mutation variables.
	bearerTransport struct {
		base  http.RoundTripper
		token string
	}
)

// RoundTrip adds the bearer Authorization header to a clone of the request and
// delegates to the base transport. The request is cloned so the caller's
// original request is never mutated.
func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header.Set("Authorization", "Bearer "+t.token)

	resp, err := t.base.RoundTrip(cloned)
	if err != nil {
		return nil, fmt.Errorf("github graphql request failed: %w", err)
	}

	return resp, nil
}

// NewClient builds a production GraphQL client that authenticates with the
// provided GitHub token and applies a 600-second request timeout.
func NewClient(githubToken string) *Client {
	httpClient := &http.Client{
		Timeout: requestTimeout,
		Transport: &bearerTransport{
			base:  http.DefaultTransport,
			token: githubToken,
		},
	}

	return &Client{
		gql: graphql.NewClient(graphqlEndpoint, httpClient),
	}
}

// CreateCommitOnBranch posts the createCommitOnBranch mutation and returns the
// created commit OID. It interprets the (response, error) pair in the same
// order as the Python tool: a transport or HTTP failure that does not carry a
// GraphQL error list maps to ErrGithubAPI; a GraphQL error list containing a
// STALE_DATA entry maps to ErrPartialPushFailure; any other GraphQL error list
// maps to ErrGithubAPI; a successful response with a missing OID maps to
// ErrGithubAPI; otherwise the OID is returned.
func (c *Client) CreateCommitOnBranch(
	ctx context.Context,
	input *CreateCommitInput,
) (string, error) {
	target := fmt.Sprintf("%s/%s", input.RepositoryNameWithOwner, input.RemoteBranchName)

	resp, err := createCommitOnBranch(ctx, c.gql, CreateCommitOnBranchInput{
		Branch: CommittableBranch{
			RepositoryNameWithOwner: input.RepositoryNameWithOwner,
			BranchName:              input.RemoteBranchName,
		},
		Message:         input.Message,
		ExpectedHeadOid: input.ExpectedHeadOid,
		FileChanges:     input.FileChanges,
	})
	if err != nil {
		detail, sentinel := interpretError(ctx, err, target)

		return "", fmt.Errorf("%w: %s", sentinel, detail)
	}

	oid := resp.CreateCommitOnBranch.Commit.Oid
	if oid == "" {
		return "", fmt.Errorf(
			"%w: creating a commit on %s produced a response in an unexpected format",
			ErrGithubAPI,
			target,
		)
	}

	slog.InfoContext(ctx, "Created commit on remote branch",
		LogKeyCommitOID, oid,
		LogKeyBranchName, target,
	)

	return oid, nil
}

// interpretError maps a non-nil error from the generated operation onto a
// package sentinel and a human-readable detail string. A GraphQL error list
// carrying a STALE_DATA entry maps to ErrPartialPushFailure; any other error
// (transport, HTTP, or non-stale GraphQL list) maps to ErrGithubAPI. The caller
// wraps the sentinel with the detail so the sentinel stays matchable via
// [errors.Is].
func interpretError(ctx context.Context, err error, target string) (string, error) {
	gqlErrors := gqlerror.List{}
	if !errors.As(err, &gqlErrors) {
		detail := fmt.Sprintf(
			"creating a commit on %s produced a response in an unexpected format: %s",
			target,
			err,
		)

		return detail, ErrGithubAPI
	}

	slog.ErrorContext(ctx, "GitHub GraphQL API returned errors",
		LogKeyBranchName, target,
		LogKeyErrorContent, gqlErrors.Error(),
	)

	if hasStaleData(gqlErrors) {
		detail := fmt.Sprintf(
			"the expected head of %s is stale, likely because the remote branch moved; aborting",
			target,
		)

		return detail, ErrPartialPushFailure
	}

	detail := fmt.Sprintf("creating a commit on %s failed: %s", target, gqlErrors.Error())

	return detail, ErrGithubAPI
}

// hasStaleData reports whether any entry in the GraphQL error list indicates a
// stale expected head OID. It checks the Extensions "type" value first and
// falls back to inspecting the error message, mirroring the Python tool.
func hasStaleData(gqlErrors gqlerror.List) bool {
	for _, gqlErr := range gqlErrors {
		if gqlErr == nil {
			continue
		}

		if extType, ok := gqlErr.Extensions["type"].(string); ok && extType == staleDataType {
			return true
		}

		if strings.Contains(gqlErr.Message, staleDataType) {
			return true
		}
	}

	return false
}
