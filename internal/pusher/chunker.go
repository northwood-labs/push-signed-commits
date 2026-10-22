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
	"encoding/json"
	"log/slog"

	"go.nwlabs.dev/push-signed-commits/internal/githubgql"
)

const (
	// bytesPerKilobyte is the number of bytes in one kilobyte, used to derive
	// the payload threshold and the approximate-megabytes warning value.
	bytesPerKilobyte = 1024

	// kilobytesPerMegabyte is the number of kilobytes in one megabyte, used to
	// derive the payload threshold and the approximate-megabytes warning value.
	kilobytesPerMegabyte = 1024

	// payloadThresholdMegabytes is the per-request payload ceiling in
	// megabytes. It is set to 40 MB to stay under GitHub's 45 MB GraphQL
	// payload limit, leaving room for the query string, variables wrapper, and
	// commit message.
	payloadThresholdMegabytes = 40

	// MaxPayloadBytes is the maximum estimated JSON payload size per GraphQL
	// request. A FileChanges object whose serialized size exceeds this value is
	// split into multiple chunks so each request stays under GitHub's limit.
	MaxPayloadBytes = payloadThresholdMegabytes * bytesPerKilobyte * kilobytesPerMegabyte

	// bytesPerMegabyte is the number of bytes in one megabyte, used to report
	// the approximate payload size in the oversize warning.
	bytesPerMegabyte = bytesPerKilobyte * kilobytesPerMegabyte

	// logKeyPayloadMegabytes is the approximate payload size in megabytes when
	// a commit must be split. It mirrors the cmd package's log key but is
	// defined locally because the cmd package is not importable here.
	logKeyPayloadMegabytes = "payload_megabytes"

	// logKeyChunkCount is the number of chunks produced for a split commit. It
	// mirrors the cmd package's log key but is defined locally because the cmd
	// package is not importable here.
	logKeyChunkCount = "chunk_count"
)

// ChunkFileChanges splits a FileChanges object into chunks that each stay under
// MaxPayloadBytes. A payload already at or below the threshold is returned
// unchanged as a single chunk. When splitting is required, every deletion is
// placed in the first chunk and additions are distributed across chunks: a new
// chunk is started only when adding the next addition would exceed the
// threshold and the current chunk already holds at least one addition, so a
// single oversized addition is never dropped and never forces an empty chunk.
// The final chunk is appended only when it holds at least one addition or
// deletion.
func ChunkFileChanges(ctx context.Context, fc githubgql.FileChanges) []githubgql.FileChanges {
	totalSize := estimateSize(ctx, fc)
	if totalSize <= MaxPayloadBytes {
		return []githubgql.FileChanges{fc}
	}

	slog.WarnContext(
		ctx,
		"Commit file changes payload exceeds the chunking threshold; splitting into multiple commits",
		logKeyPayloadMegabytes, totalSize/bytesPerMegabyte,
	)

	var chunks []githubgql.FileChanges

	current := githubgql.FileChanges{
		Additions: nil,
		Deletions: fc.Deletions,
	}
	currentSize := estimateSize(ctx, current)

	for i := range fc.Additions {
		addition := fc.Additions[i]
		additionSize := estimateAdditionSize(ctx, addition)

		if currentSize+additionSize > MaxPayloadBytes && len(current.Additions) > 0 {
			chunks = append(chunks, current)
			current = githubgql.FileChanges{
				Additions: nil,
				Deletions: nil,
			}
			currentSize = estimateSize(ctx, current)
		}

		current.Additions = append(current.Additions, addition)

		currentSize += additionSize
	}

	if len(current.Additions) > 0 || len(current.Deletions) > 0 {
		chunks = append(chunks, current)
	}

	slog.InfoContext(
		ctx,
		"Split file changes into chunks",
		logKeyChunkCount, len(chunks),
	)

	return chunks
}

// estimateSize returns the byte length of the JSON serialization of a
// FileChanges object, matching the real request body because the JSON field
// names are identical. A marshal failure is not realistic for these simple
// value types; if one occurs it is logged and treated as a zero-byte estimate
// so the error is never silently discarded.
func estimateSize(ctx context.Context, fc githubgql.FileChanges) int {
	encoded, err := json.Marshal(fc)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to estimate file changes payload size")

		return 0
	}

	return len(encoded)
}

// estimateAdditionSize returns the byte length of the JSON serialization of a
// single FileAddition, used to decide when the current chunk must be finalized.
// A marshal failure is logged and treated as a zero-byte estimate so the error
// is never silently discarded.
func estimateAdditionSize(ctx context.Context, addition githubgql.FileAddition) int {
	encoded, err := json.Marshal(addition)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to estimate file addition payload size")

		return 0
	}

	return len(encoded)
}
