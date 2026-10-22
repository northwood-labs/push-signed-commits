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
	"errors"
	"fmt"
	"log/slog"

	"go.nwlabs.dev/push-signed-commits/internal/gitclient"
	"go.nwlabs.dev/push-signed-commits/internal/githubgql"
)

const (
	// singleChunkCount is the number of chunks a commit produces when its
	// payload fits in one request; only when there are more than this many
	// chunks is the per-part headline annotation applied.
	singleChunkCount = 1

	// logKeyCreatedCount is the number of commits created on the remote branch.
	// It mirrors the cmd package's log key but is defined locally because the
	// cmd package is not importable here.
	logKeyCreatedCount = "created_count"

	// logKeyCommitHash is the local commit hash currently being recreated on
	// the remote branch. It mirrors the cmd package's log key but is defined
	// locally because the cmd package is not importable here.
	logKeyCommitHash = "commit_hash"

	// logKeyCommitOID is the OID of the commit just created on the remote
	// branch. It mirrors the cmd package's log key but is defined locally
	// because the cmd package is not importable here.
	logKeyCommitOID = "commit_oid"

	// logKeyBranchName is the name of the remote branch being operated on. It
	// mirrors the cmd package's log key but is defined locally because the cmd
	// package is not importable here.
	logKeyBranchName = "branch_name"

	// logKeyErrorContent is the error detail attached to a failure log. It
	// mirrors the cmd package's log key but is defined locally because the cmd
	// package is not importable here.
	logKeyErrorContent = "error_content"
)

// ErrPartialPushFailure indicates that, partway through recreating commits,
// the remote branch head no longer matched the last commit this tool
// pushed. The remote is left in a partially-updated state that may require
// manual reconciliation. The cmd package compares against its own boundary
// sentinel via [errors.Is].
var ErrPartialPushFailure = errors.New("push completed only partially")

type (
	// Config holds the per-run values the pusher needs to address the remote
	// branch. It is a pusher-local carrier rather than the cmd package's
	// configuration type: the cmd package imports pusher, so pusher cannot
	// import cmd without creating an import cycle. The cmd package populates
	// these fields from its own resolved configuration when calling Push.
	Config struct {
		// RepositoryNameWithOwner is the "owner/name" slug of the target
		// repository, used in the commit branch selector.
		RepositoryNameWithOwner string

		// RemoteName is the configured git remote, used to re-fetch and
		// re-verify the remote head between commits.
		RemoteName string

		// RemoteBranchName is the unqualified remote branch name the commits
		// are created on.
		RemoteBranchName string
	}

	// PushOptions carries everything Push needs beyond the git and GraphQL
	// clients and the request context. It is passed by pointer.
	PushOptions struct {
		// Config carries the remote-addressing values for this run.
		Config *Config

		// MergeBaseOID is the merge-base OID used as the expected head for the
		// very first commit created.
		MergeBaseOID string

		// LocalHashes is the oldest-to-newest list of local commit hashes to
		// recreate on the remote branch.
		LocalHashes []string
	}

	// pushState tracks the progress of the push loop across local commits so
	// the per-commit and per-chunk helpers stay within the parameter limit. It
	// is passed by pointer.
	pushState struct {
		// LastCommitPushed is the OID of the most recently created commit, used
		// as the expected head for the next commit and as the re-verification
		// target. It is empty until the first commit is created.
		LastCommitPushed string

		// Created is the running list of OIDs created on the remote branch, in
		// creation order.
		Created []string
	}

	// pushCommitInput carries the inputs for recreating a single local commit.
	// It is passed by pointer to keep pushLocalCommit within the parameter
	// limit.
	pushCommitInput struct {
		Git       gitclient.GitClient
		GQL       githubgql.GraphQLClient
		Opts      *PushOptions
		State     *pushState
		LocalHash string
	}

	// pushChunkInput carries the inputs for creating a single chunk of a local
	// commit. It is passed by pointer to keep createChunk within the parameter
	// limit.
	pushChunkInput struct {
		GQL         githubgql.GraphQLClient
		Opts        *PushOptions
		State       *pushState
		Message     githubgql.CommitMessage
		FileChanges githubgql.FileChanges
	}
)

// Push recreates each local commit on the remote branch in oldest-to-newest
// order through GitHub's createCommitOnBranch mutation, chaining the expected
// head OID from one commit to the next.
//
// The first commit uses the merge-base OID as its expected head; every
// subsequent commit uses the OID of the commit this tool most recently created.
// Before each commit after the first, the remote head is re-fetched and
// compared to that last-created OID; a mismatch means the remote changed out
// from under us, so Push returns an error wrapping [ErrPartialPushFailure] that
// reports the commits already pushed and the local commits not yet pushed.
//
// A commit whose payload is split into multiple chunks has each chunk's
// headline annotated with " (part <i+1>/<n>)" while the body is left unchanged;
// a single-chunk commit uses its message unmodified.
func Push(
	ctx context.Context,
	git gitclient.GitClient,
	gql githubgql.GraphQLClient,
	opts *PushOptions,
) error {
	state := &pushState{
		LastCommitPushed: "",
		Created:          nil,
	}

	for _, localHash := range opts.LocalHashes {
		err := pushLocalCommit(ctx, &pushCommitInput{
			Git:       git,
			GQL:       gql,
			Opts:      opts,
			State:     state,
			LocalHash: localHash,
		})
		if err != nil {
			return fmt.Errorf("failed to recreate commit %s on the remote branch: %w", localHash, err)
		}
	}

	slog.InfoContext(
		ctx,
		"Finished creating commits on the remote branch",
		logKeyCreatedCount, len(opts.LocalHashes),
		logKeyBranchName, opts.Config.RemoteBranchName,
	)

	return nil
}

// pushLocalCommit recreates a single local commit on the remote branch. It
// first re-verifies the remote head when a previous commit has been pushed,
// then builds the commit message and file changes, chunks the payload, and
// creates one commit per chunk while chaining the expected head OID.
func pushLocalCommit(ctx context.Context, input *pushCommitInput) error {
	err := verifyRemoteHead(ctx, input.Git, input.Opts, input.State)
	if err != nil {
		return fmt.Errorf("remote branch re-verification failed: %w", err)
	}

	body, err := input.Git.CommitMessageBody(ctx, input.LocalHash)
	if err != nil {
		return fmt.Errorf(
			"failed to read the commit message for commit %s: %w",
			input.LocalHash,
			err,
		)
	}

	commitMessage := BuildCommitMessage(body, input.LocalHash)

	fileChanges, err := BuildFileChanges(ctx, input.Git, input.LocalHash)
	if err != nil {
		return fmt.Errorf(
			"failed to build the file changes for commit %s: %w",
			input.LocalHash,
			err,
		)
	}

	chunks := ChunkFileChanges(ctx, fileChanges)

	for chunkIndex := range chunks {
		message := chunkMessage(commitMessage, chunkIndex, len(chunks))

		chunkErr := createChunk(ctx, &pushChunkInput{
			GQL:         input.GQL,
			Opts:        input.Opts,
			State:       input.State,
			Message:     message,
			FileChanges: chunks[chunkIndex],
		})
		if chunkErr != nil {
			return fmt.Errorf("failed to create chunk %d of %d: %w", chunkIndex+1, len(chunks), chunkErr)
		}
	}

	slog.InfoContext(
		ctx,
		"Created a commit on the remote branch from a local commit",
		logKeyCommitOID, input.State.LastCommitPushed,
		logKeyCommitHash, input.LocalHash,
		logKeyBranchName, input.Opts.Config.RemoteBranchName,
	)

	return nil
}

// verifyRemoteHead re-fetches the remote branch head and compares it to the
// most recently created commit OID. It is a no-op until the first commit has
// been pushed. A mismatch means the remote branch moved while this tool was
// mid-push, so it returns an error wrapping [ErrPartialPushFailure] reporting
// the commits already pushed and the local commits not yet pushed.
func verifyRemoteHead(
	ctx context.Context,
	git gitclient.GitClient,
	opts *PushOptions,
	state *pushState,
) error {
	if state.LastCommitPushed == "" {
		return nil
	}

	err := git.FetchRemoteBranch(ctx, opts.Config.RemoteName, opts.Config.RemoteBranchName)
	if err != nil {
		return fmt.Errorf(
			"failed to re-fetch the remote branch before the next commit: %w",
			err,
		)
	}

	head, err := git.RemoteHeadOID(ctx, opts.Config.RemoteName, opts.Config.RemoteBranchName)
	if err != nil {
		return fmt.Errorf(
			"failed to read the remote branch head before the next commit: %w",
			err,
		)
	}

	if head != state.LastCommitPushed {
		return fmt.Errorf(
			"%w: pushed %d commit(s) with hashes %v; the local commits not pushed were %v",
			ErrPartialPushFailure,
			len(state.Created),
			state.Created,
			opts.LocalHashes[len(state.Created):],
		)
	}

	return nil
}

// chunkMessage returns the commit message to use for one chunk. When a commit
// is split across more than one chunk, the headline is annotated with the
// one-based part index and total count while the body is left unchanged; a
// single-chunk commit returns the message unmodified.
func chunkMessage(message githubgql.CommitMessage, chunkIndex, chunkCount int) githubgql.CommitMessage {
	if chunkCount <= singleChunkCount {
		return message
	}

	return githubgql.CommitMessage{
		Headline: fmt.Sprintf("%s (part %d/%d)", message.Headline, chunkIndex+1, chunkCount),
		Body:     message.Body,
	}
}

// createChunk creates one commit for a single chunk of a local commit, using
// the merge-base OID as the expected head for the very first commit and the
// most recently created OID thereafter. On success it records the new OID as
// the most recent commit and appends it to the created list.
func createChunk(ctx context.Context, input *pushChunkInput) error {
	expectedHeadOID := input.State.LastCommitPushed
	if expectedHeadOID == "" {
		expectedHeadOID = input.Opts.MergeBaseOID
	}

	oid, err := input.GQL.CreateCommitOnBranch(ctx, &githubgql.CreateCommitInput{
		Message:                 input.Message,
		RepositoryNameWithOwner: input.Opts.Config.RepositoryNameWithOwner,
		RemoteBranchName:        input.Opts.Config.RemoteBranchName,
		ExpectedHeadOid:         expectedHeadOID,
		FileChanges:             input.FileChanges,
	})
	if err != nil {
		// When the very first commit fails on a GitHub API or branch
		// divergence error, no commit has been created yet, so the remote
		// branch is untouched. Record that fact explicitly before propagating
		// so an operator knows no reconciliation is required. Once a commit has
		// been pushed the remote is no longer guaranteed unchanged, so the same
		// error class is propagated without that claim.
		if input.State.LastCommitPushed == "" && errors.Is(err, githubgql.ErrGithubAPI) {
			slog.ErrorContext(
				ctx,
				"An error occurred while creating the first commit; no changes were made to the remote branch",
				logKeyBranchName, input.Opts.Config.RemoteBranchName,
				logKeyErrorContent, err,
			)
		}

		return fmt.Errorf(
			"failed to create a commit on remote branch %s/%s: %w",
			input.Opts.Config.RepositoryNameWithOwner,
			input.Opts.Config.RemoteBranchName,
			err,
		)
	}

	input.State.LastCommitPushed = oid
	input.State.Created = append(input.State.Created, oid)

	return nil
}
