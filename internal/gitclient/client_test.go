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
	"github.com/go-openapi/testify/require"
)

// TestSplitNameStatusLines verifies the line-splitting transformation that
// NameStatus applies to raw `git diff --name-status` output. The raw lines must
// be returned unaltered (tabs and rename scores preserved) so downstream
// parsing can split each line on tab characters, and the trailing empty element
// produced by the final newline must be dropped to match Python's splitlines().
func TestSplitNameStatusLines(t *testing.T) {
	testCases := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name:     "single addition with trailing newline",
			output:   "A\tnewfile.txt\n",
			expected: []string{"A\tnewfile.txt"},
		},
		{
			name:     "single modification with trailing newline",
			output:   "M\tmod.txt\n",
			expected: []string{"M\tmod.txt"},
		},
		{
			name:     "rename with similarity score preserves both filenames",
			output:   "R100\told.txt\tnew.txt\n",
			expected: []string{"R100\told.txt\tnew.txt"},
		},
		{
			name:     "deletion with trailing newline",
			output:   "D\tgone.txt\n",
			expected: []string{"D\tgone.txt"},
		},
		{
			name:   "multiple mixed lines with trailing newline drops no content",
			output: "A\tnewfile.txt\nM\tmod.txt\nR100\told.txt\tnew.txt\nD\tgone.txt\n",
			expected: []string{
				"A\tnewfile.txt",
				"M\tmod.txt",
				"R100\told.txt\tnew.txt",
				"D\tgone.txt",
			},
		},
		{
			name:   "no trailing newline keeps every line",
			output: "A\tfoo\nM\tbar",
			expected: []string{
				"A\tfoo",
				"M\tbar",
			},
		},
		{
			name:     "empty output yields an empty slice",
			output:   "",
			expected: []string(nil),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lines := splitNameStatusLines(testCase.output)

			// The empty-output case must drop the trailing empty element down
			// to a zero-length slice so a repo with no changes yields no
			// spurious entries for downstream parsing.
			require.Len(t, lines, len(testCase.expected))

			// The lines must be returned exactly as produced by git, in order,
			// with tabs and rename scores intact so task 7's parser can split
			// on tab. Compare element by element so a zero-length result
			// matches whether the slice is nil or empty.
			for i, want := range testCase.expected {
				assert.Equal(t, want, lines[i])
			}
		})
	}
}
