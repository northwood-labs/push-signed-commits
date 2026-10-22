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
	"context"
	"strings"
	"testing"

	"github.com/go-openapi/testify/assert"
	"github.com/go-openapi/testify/require"

	"go.nwlabs.dev/push-signed-commits/internal/githubgql"
)

const (
	// halfPayloadContentsBytes is the size, in bytes, of each oversized
	// addition's Contents used in the splitting tests. It is set just over
	// half of MaxPayloadBytes so that any two such additions together exceed
	// the threshold and are forced into separate chunks, while a single one
	// still fits on its own.
	halfPayloadContentsBytes = MaxPayloadBytes/2 + bytesPerMegabyte

	// oversizedContentsBytes is the size, in bytes, of a single addition whose
	// Contents alone exceed MaxPayloadBytes. It confirms an oversized addition
	// still produces its own chunk rather than being dropped.
	oversizedContentsBytes = MaxPayloadBytes + bytesPerMegabyte
)

// collectAdditions flattens the additions across every chunk in order so a
// test can confirm ordering and completeness are preserved.
func collectAdditions(chunks []githubgql.FileChanges) []githubgql.FileAddition {
	var additions []githubgql.FileAddition
	for i := range chunks {
		additions = append(additions, chunks[i].Additions...)
	}

	return additions
}

// deletionsAfterFirstChunk counts every deletion that appears in any chunk
// other than the first, so a test can confirm deletions live only in the first
// chunk.
func deletionsAfterFirstChunk(chunks []githubgql.FileChanges) int {
	count := 0
	for i := 1; i < len(chunks); i++ {
		count += len(chunks[i].Deletions)
	}

	return count
}

func TestChunkFileChangesSingleChunk(t *testing.T) {
	testCases := []struct {
		name string
		fc   githubgql.FileChanges
	}{
		{
			name: "empty file changes returns one chunk equal to the original",
			fc: githubgql.FileChanges{
				Deletions: nil,
				Additions: nil,
			},
		},
		{
			name: "small file changes with additions and deletions returns one chunk",
			fc: githubgql.FileChanges{
				Deletions: []githubgql.FileDeletion{
					{Path: "removed-one.txt"},
					{Path: "removed-two.txt"},
				},
				Additions: []githubgql.FileAddition{
					{Path: "added-one.txt", Contents: "aGVsbG8="},
					{Path: "added-two.txt", Contents: "d29ybGQ="},
					{Path: "added-three.txt", Contents: "IQ=="},
				},
			},
		},
		{
			name: "deletions-only file changes returns one chunk",
			fc: githubgql.FileChanges{
				Deletions: []githubgql.FileDeletion{
					{Path: "gone.txt"},
				},
				Additions: nil,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			chunks := ChunkFileChanges(context.Background(), testCase.fc)

			require.Len(t, chunks, 1)
			assert.Equal(t, testCase.fc, chunks[0])
		})
	}
}

func TestChunkFileChangesAboveThreshold(t *testing.T) {
	deletions := []githubgql.FileDeletion{
		{Path: "delete-one.txt"},
		{Path: "delete-two.txt"},
	}

	// Two additions, each just over half the threshold, so together they
	// exceed MaxPayloadBytes and must land in separate chunks.
	twoLargeAdditions := []githubgql.FileAddition{
		{Path: "large-one.txt", Contents: strings.Repeat("a", halfPayloadContentsBytes)},
		{Path: "large-two.txt", Contents: strings.Repeat("b", halfPayloadContentsBytes)},
	}

	// Three additions, each just over half the threshold, so no two fit in one
	// chunk and each is forced into its own chunk.
	threeLargeAdditions := []githubgql.FileAddition{
		{Path: "large-one.txt", Contents: strings.Repeat("a", halfPayloadContentsBytes)},
		{Path: "large-two.txt", Contents: strings.Repeat("b", halfPayloadContentsBytes)},
		{Path: "large-three.txt", Contents: strings.Repeat("c", halfPayloadContentsBytes)},
	}

	// A single addition whose Contents alone exceed the threshold. It cannot be
	// split further, so it still produces exactly one chunk.
	singleOversizedAddition := []githubgql.FileAddition{
		{Path: "oversized.txt", Contents: strings.Repeat("z", oversizedContentsBytes)},
	}

	testCases := []struct {
		name              string
		fc                githubgql.FileChanges
		expectedDeletions []githubgql.FileDeletion
		expectedChunks    int
	}{
		{
			name: "two oversized additions split into two chunks with deletions in the first",
			fc: githubgql.FileChanges{
				Deletions: deletions,
				Additions: twoLargeAdditions,
			},
			expectedChunks:    2,
			expectedDeletions: deletions,
		},
		{
			name: "three oversized additions split into three chunks",
			fc: githubgql.FileChanges{
				Deletions: deletions,
				Additions: threeLargeAdditions,
			},
			expectedChunks:    3,
			expectedDeletions: deletions,
		},
		{
			name: "single oversized addition is never dropped",
			fc: githubgql.FileChanges{
				Deletions: nil,
				Additions: singleOversizedAddition,
			},
			expectedChunks:    1,
			expectedDeletions: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			chunks := ChunkFileChanges(context.Background(), testCase.fc)

			// The expected number of chunks is produced.
			require.Len(t, chunks, testCase.expectedChunks)

			// Every deletion lands in the first chunk and nowhere else: the
			// first chunk holds the original deletions and no later chunk
			// carries any deletion.
			assert.Equal(t, testCase.expectedDeletions, chunks[0].Deletions)
			assert.Equal(t, 0, deletionsAfterFirstChunk(chunks))

			// No chunk is empty: every returned chunk holds at least one
			// addition or deletion, so the final chunk is never an empty
			// trailing chunk.
			for i := range chunks {
				hasContent := len(chunks[i].Additions) > 0 || len(chunks[i].Deletions) > 0
				assert.True(t, hasContent, "chunk %d must hold at least one addition or deletion", i)
			}

			// The last chunk specifically must carry content, confirming an
			// empty final chunk is never appended.
			last := chunks[len(chunks)-1]
			assert.True(t, len(last.Additions) > 0 || len(last.Deletions) > 0)

			// Concatenating additions across chunks preserves order and equals
			// the original additions, so nothing is dropped or reordered.
			assert.Equal(t, testCase.fc.Additions, collectAdditions(chunks))
		})
	}
}
