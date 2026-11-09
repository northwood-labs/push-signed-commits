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

package gitclient

import (
	"testing"

	"github.com/go-openapi/testify/assert"
	"pgregory.net/rapid"
)

// Feature: github-token-git-auth, Property 2: Transport auth is always token basic auth
//
// TestNewBasicAuthProperty asserts that for any non-empty token value, the
// credential newBasicAuth builds (the value supplied as the Auth field on the
// go-git ListOptions and FetchOptions) is HTTP basic auth whose username is
// exactly the fixed x-access-token value and whose password is exactly the
// token. rapid.Check runs at least 100 iterations by default, generating
// random non-empty token strings for each.
//
// Validates: Requirements 4.1, 4.2.
func TestNewBasicAuthProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random non-empty token: any string of length >= 1 keeps
		// the generator within the authenticated input space (an empty token
		// is rejected earlier by the missing-token check in the cmd layer).
		token := rapid.StringN(1, -1, -1).Draw(t, "token")

		auth := newBasicAuth(token)

		assert.Equal(t, authUsername, auth.Username)
		assert.Equal(t, "x-access-token", auth.Username)
		assert.Equal(t, token, auth.Password)
	})
}
