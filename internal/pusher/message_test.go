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

package pusher

import (
	"strings"
	"testing"

	"github.com/go-openapi/testify/assert"
	"github.com/go-openapi/testify/require"
)

func TestBuildCommitMessage(t *testing.T) {
	testCases := []struct {
		name             string
		messageBody      string
		commitHash       string
		expectedHeadline string
		expectedBody     string
	}{
		{
			name:             "single-line message",
			messageBody:      "Fix bug",
			commitHash:       "abc123",
			expectedHeadline: "Fix bug",
			expectedBody:     "\n\nThis commit was created from the local commit with hash abc123.",
		},
		{
			name:             "multi-line message with body",
			messageBody:      "Add feature\n\nThis explains the feature.\nSecond body line.",
			commitHash:       "def456",
			expectedHeadline: "Add feature",
			expectedBody: "\nThis explains the feature.\nSecond body line." +
				"\n\nThis commit was created from the local commit with hash def456.",
		},
		{
			name:             "leading and trailing whitespace is trimmed before splitting",
			messageBody:      "   \n\n  Trimmed headline\n\nTrimmed body line.  \n\n   ",
			commitHash:       "fedcba987654",
			expectedHeadline: "Trimmed headline",
			expectedBody: "\nTrimmed body line." +
				"\n\nThis commit was created from the local commit with hash fedcba987654.",
		},
		{
			name:             "provenance line names the exact commit hash",
			messageBody:      "Short",
			commitHash:       "0123456789abcdef0123456789abcdef01234567",
			expectedHeadline: "Short",
			expectedBody: "\n\nThis commit was created from the local commit with hash " +
				"0123456789abcdef0123456789abcdef01234567.",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			message := BuildCommitMessage(testCase.messageBody, testCase.commitHash)

			assert.Equal(t, testCase.expectedHeadline, message.Headline)
			assert.Equal(t, testCase.expectedBody, message.Body)

			// The provenance line must name the exact commit hash passed in
			// and end with a period so the remote history stays traceable
			// back to the local commit.
			provenance := "This commit was created from the local commit with hash " + testCase.commitHash + "."
			require.True(t, strings.HasSuffix(message.Body, provenance))
			assert.True(t, strings.HasSuffix(message.Body, "."))
			assert.Contains(t, message.Body, testCase.commitHash)
		})
	}
}
