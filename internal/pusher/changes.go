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
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"go.nwlabs.dev/push-signed-commits/internal/gitclient"
	"go.nwlabs.dev/push-signed-commits/internal/githubgql"
)

const (
	// statusRenameMarker is the substring that identifies a rename status
	// field. Git reports renames as scored codes such as "R100", so the marker
	// is matched as a substring rather than compared for equality.
	statusRenameMarker = "R"

	// statusAdded is the git name-status code for an added file.
	statusAdded = "A"

	// statusModified is the git name-status code for a modified file.
	statusModified = "M"

	// statusDeleted is the git name-status code for a deleted file.
	statusDeleted = "D"

	// nameStatusSeparator is the tab character that separates the status field
	// from the filename fields in `git diff --name-status` output.
	nameStatusSeparator = "\t"

	// renameNewNameIndex is the index of the new filename field in a rename
	// change line, where the old name is at index 0 and the new name follows.
	renameNewNameIndex = 1

	// logKeyNameStatusOutput is the raw joined `git diff --name-status` output
	// for a commit. It is defined locally because the cmd package's log keys
	// are not importable here.
	logKeyNameStatusOutput = "name_status_output"

	// logKeyChangeStatus is the detected status field for a single change line.
	// It is defined locally because the cmd package's log keys are not
	// importable here.
	logKeyChangeStatus = "change_status"

	// logKeyFileNames is the detected filename fields for a single change line.
	// It is defined locally because the cmd package's log keys are not
	// importable here.
	logKeyFileNames = "file_names"
)

type (
	// nameStatusChange carries the inputs needed to apply a single parsed
	// name-status line to the accumulating FileChanges. It is passed by pointer
	// to keep applyNameStatusChange within the parameter limit.
	nameStatusChange struct {
		Client      gitclient.GitClient
		FileChanges *githubgql.FileChanges
		CommitHash  string
		Status      string
		Filenames   []string
	}
)

// BuildFileChanges derives the FileChanges object for a single local commit by
// parsing the raw `git diff --name-status <hash>^ <hash>` output and reading
// each affected file's contents at the commit. Additions and modifications
// become a FileAddition, renames become a FileDeletion of the old path plus a
// FileAddition of the new path, and deletions become a FileDeletion, matching
// the behavior of the Python tool.
//
// File contents are read as raw bytes through the GitClient and base64-encoded
// into a UTF-8 string so binary files are captured faithfully. The raw
// name-status output is logged at debug level, and the detected status and
// filenames for each change are logged separately so they are not gated behind
// the raw-output debug check.
func BuildFileChanges(
	ctx context.Context,
	client gitclient.GitClient,
	commitHash string,
) (githubgql.FileChanges, error) {
	lines, err := client.NameStatus(ctx, commitHash)
	if err != nil {
		return githubgql.FileChanges{}, fmt.Errorf(
			"failed to read the file changes for commit %s: %w",
			commitHash,
			err,
		)
	}

	slog.DebugContext(
		ctx,
		"Retrieved raw git diff name-status output for the local commit",
		logKeyNameStatusOutput, strings.Join(lines, "\n"),
	)

	fileChanges := githubgql.FileChanges{
		Additions: nil,
		Deletions: nil,
	}

	for _, line := range lines {
		fields := strings.Split(line, nameStatusSeparator)
		status := fields[0]
		filenames := fields[1:]

		slog.DebugContext(
			ctx,
			"Detected file change status and filenames for the local commit",
			logKeyChangeStatus, status,
			logKeyFileNames, filenames,
		)

		changeErr := applyNameStatusChange(ctx, &nameStatusChange{
			Client:      client,
			FileChanges: &fileChanges,
			CommitHash:  commitHash,
			Status:      status,
			Filenames:   filenames,
		})
		if changeErr != nil {
			return githubgql.FileChanges{}, fmt.Errorf(
				"failed to derive the file changes for commit %s: %w",
				commitHash,
				changeErr,
			)
		}
	}

	return fileChanges, nil
}

// applyNameStatusChange classifies one parsed name-status line and appends the
// corresponding FileAddition and/or FileDeletion to the accumulating
// FileChanges. Status "A" or "M" adds the file, a status containing "R" models
// a rename as a deletion of the old path plus an addition of the new path, and
// status "D" deletes the file. Any other status is ignored, matching the
// Python tool which only handles A, M, R, and D for full commits.
func applyNameStatusChange(ctx context.Context, change *nameStatusChange) error {
	switch {
	case change.Status == statusAdded || change.Status == statusModified:
		// The path is the last filename field while the contents are read from
		// the first filename field; for A/M there is only one filename so they
		// coincide, but the selection mirrors the Python tool exactly.
		contents, err := encodedContents(ctx, change.Client, change.CommitHash, change.Filenames[0])
		if err != nil {
			return fmt.Errorf("failed to read the added or modified file: %w", err)
		}

		change.FileChanges.Additions = append(change.FileChanges.Additions, githubgql.FileAddition{
			Path:     change.Filenames[len(change.Filenames)-1],
			Contents: contents,
		})
	case strings.Contains(change.Status, statusRenameMarker):
		oldName := change.Filenames[0]
		newName := change.Filenames[renameNewNameIndex]

		contents, err := encodedContents(ctx, change.Client, change.CommitHash, newName)
		if err != nil {
			return fmt.Errorf("failed to read the renamed file: %w", err)
		}

		change.FileChanges.Deletions = append(change.FileChanges.Deletions, githubgql.FileDeletion{
			Path: oldName,
		})
		change.FileChanges.Additions = append(change.FileChanges.Additions, githubgql.FileAddition{
			Path:     newName,
			Contents: contents,
		})
	case change.Status == statusDeleted:
		change.FileChanges.Deletions = append(change.FileChanges.Deletions, githubgql.FileDeletion{
			Path: change.Filenames[0],
		})
	default:
		// Only the A, M, R, and D statuses occur for full commits; any other
		// status is ignored, matching the Python tool.
	}

	return nil
}

// encodedContents reads a file's raw bytes at a commit through the GitClient
// and returns them as a standard-base64-encoded UTF-8 string, matching the
// Python tool's base64.b64encode(...).decode("utf-8") behavior.
func encodedContents(
	ctx context.Context,
	client gitclient.GitClient,
	commitHash, path string,
) (string, error) {
	raw, err := client.FileContentsAtCommit(ctx, commitHash, path)
	if err != nil {
		return "", fmt.Errorf(
			"failed to read the contents of %s at commit %s: %w",
			path,
			commitHash,
			err,
		)
	}

	return base64.StdEncoding.EncodeToString(raw), nil
}
