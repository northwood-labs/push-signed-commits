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
	"strings"
	"testing"

	"github.com/go-openapi/testify/assert"
	"pgregory.net/rapid"
)

// Feature: github-token-git-auth, Property 1: Constructed URL is always the well-known HTTPS form
//
// TestBuildHTTPSRemoteURLProperty asserts that for any owner/name pair drawn
// from the allowed GitHub slug alphabet (letters, digits, hyphens, dots, and
// mixed case), buildHTTPSRemoteURL always produces the fixed github.com HTTPS
// clone URL of the form https://github.com/<owner>/<name>.git. It further
// confirms the URL begins with the https:// scheme and never leaks an SSH
// transport (no ssh:// scheme and no git@ user prefix). rapid.Check runs at
// least 100 iterations by default, generating random owner and name segments
// for each.
//
// Validates: Requirements 3.1, 5.1, 5.2.
func TestBuildHTTPSRemoteURLProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Draw non-empty owner and name segments from the allowed GitHub slug
		// alphabet: the first character is a letter or digit and the rest may
		// also include dots, hyphens, and underscores. Mixed case is covered
		// because the alphabet includes both upper- and lower-case letters.
		segment := `[A-Za-z0-9][A-Za-z0-9._-]*`

		owner := rapid.StringMatching(segment).Draw(t, "owner")
		name := rapid.StringMatching(segment).Draw(t, "name")

		url := buildHTTPSRemoteURL(owner + "/" + name)

		assert.Equal(t, "https://github.com/"+owner+"/"+name+".git", url)
		assert.True(t, strings.HasPrefix(url, "https://"))
		assert.NotContains(t, url, "ssh://")
		assert.NotContains(t, url, "git@")
	})
}
