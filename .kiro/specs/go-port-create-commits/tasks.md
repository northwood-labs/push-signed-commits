# Implementation Plan: Go Port of create_commits.py

## Overview

This plan ports `create_commits.py` to the Go module `go.nwlabs.dev/push-signed-commits` with full behavioral parity, except the intentional rebase-first step in Requirement 5. Work proceeds in build order: module and command scaffolding, then flags and configuration and logging, then branch validation, then the `GitClient` and its operations, then the genqlient-backed `GraphQLClient`, then file-change and commit-message building, then chunking, then push orchestration, then the remote-verification and top-level wiring, and finally a full verification pass. Every `.go` file carries the Apache 2.0 Northwood Labs license header dated 2026. Tests are traditional table-driven Go tests using the standard `testing` package with testify assertions; property-based testing is not used anywhere in this plan.

## Tasks

- [x] 1. Scaffold the Go module and command skeleton
  - [x] 1.1 Create the module and entrypoint
    - Create `go.mod` declaring module `go.nwlabs.dev/push-signed-commits`
    - Create a minimal `main.go` whose body only calls `cmd.Execute()` and exits non-zero on error
    - Add the Apache 2.0 Northwood Labs license header (year 2026) to every `.go` file created
    - _Requirements: 13.3_

  - [x] 1.2 Create the cmd package scaffolding
    - Create `cmd/doc.go` with the package doc comment and license header
    - Create `cmd/root.go` with the Cobra root command and an empty `RunE` pipeline stub plus `Execute()` that builds `context.Background()` and runs through `fang.Execute()`
    - Create `cmd/errors.go` with a single `var()` block declaring the sentinels `ErrRemoteBranchNotFound`, `ErrRebaseAborted`, `ErrRemoteBranchDiverged`, `ErrGithubAPI`, and `ErrPartialPushFailure`, each with its own doc comment
    - Create `cmd/logkeys.go` with `snake_case` log-key string constants
    - _Requirements: 13.3, 14.1_

  - [x] 1.3 Create the version subcommand
    - Create `cmd/version.go` wiring `clihelpers.VersionScreen()` as the version subcommand
    - _Requirements: 1.1_

  - [x] 1.4 Create the internal package doc stubs
    - Create `internal/gitclient/doc.go`, `internal/pusher/doc.go`, and `internal/githubgql/doc.go`, each with the package doc comment and license header
    - _Requirements: 13.3_

- [x] 2. Implement CLI flags, configuration, and logging
  - [x] 2.1 Register the six named flags and resolve configuration
    - In `cmd/root.go` `init()` (marked `// lint:allow_init`), register `--github-token`, `--repository-name-with-owner`, `--local-branch-name`, `--remote-name`, `--remote-branch-name`, and `--log-level` using `f`-prefixed variables
    - Mark the five required flags with Cobra's required-flag mechanism so an omitted flag reports a usage error and exits non-zero; default `--log-level` to `WARN`
    - Resolve values through koanf in the standard priority order (CLI flag, env, local config, user config, global config) into a `Config` struct, passing flag string values to consumers unaltered
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7_

  - [x] 2.2 Configure logging from the resolved log level
    - Configure `go.nwlabs.dev/x/logutils` at startup to the level named by `--log-level`, defaulting to `WARN` so info and debug are suppressed
    - Use the slog `*Context` variants everywhere and the `snake_case` key constants from `cmd/logkeys.go`
    - _Requirements: 3.1, 3.2, 3.3_

- [x] 3. Implement branch name validation
  - [x] 3.1 Implement ValidateBranchName
    - Add `ValidateBranchName(opts *BranchNameInput)` in the `cmd` package rejecting `origin/`, `refs/heads/`, and `<remote_name>/` prefixes for both local and remote branch names, with a branch-type-specific message
    - Define the `BranchNameInput` options struct (`BranchName`, `BranchType`, `RemoteName`)
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7_

  - [x] 3.2 Write table-driven tests for branch validation
    - Standard table-driven `testing` tests (not property-based) covering local and remote names with `origin/`, `refs/heads/`, and `<remote_name>/` prefixes producing the correct error, and clean names passing
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 15.1, 15.2, 15.3_

- [x] 4. Define the GitClient interface and options structs
  - [x] 4.1 Declare the GitClient interface and option types
    - In `internal/gitclient/gitclient.go`, declare the package-level `GitClient` interface with all methods from the design (`RemoteBranchExists`, `FetchRemoteBranch`, `RebaseOnto`, `MergeBase`, `RemoteHeadOID`, `CommitsNotOnRemote`, `CommitMessageBody`, `NameStatus`, `FileContentsAtCommit`)
    - Declare the `RebaseOptions`, `MergeBaseOptions`, and `CommitRangeOptions` structs (passed by pointer)
    - _Requirements: 4.1, 5.1, 5.2, 5.4, 5.5, 6.1, 7.1, 8.1, 9.1, 14.1_

  - [x] 4.2 Write the hand-written GitClient fake
    - Add a hand-written fake implementing `GitClient` with table-defined return values and recorded calls, for use by pusher and command tests
    - _Requirements: 15.1, 15.2_

- [x] 5. Implement GitClient operations (go-git with os/exec fallbacks)
  - [x] 5.1 Implement remote branch existence and fetch
    - Implement `RemoteBranchExists` via go-git `Remote.ListContext` filtered to the branch ref, returning `(false, nil)` for a missing branch and interpreting the result like `git ls-remote --exit-code --heads`
    - Implement `FetchRemoteBranch` via go-git `Repository.FetchContext`
    - _Requirements: 4.1, 4.2, 4.3, 14.2_

  - [x] 5.2 Implement rebase onto upstream with abort
    - Implement `RebaseOnto` via `os/exec` running `git rebase <remote>/<branch> <local>`, building the argument vector from discrete validated inputs
    - On rebase failure, run `git rebase --abort` and return an error wrapping `ErrRebaseAborted` instructing manual reconciliation, without auto-resolving conflicts
    - _Requirements: 5.1, 5.2, 5.3_

  - [x] 5.3 Implement merge-base, remote head OID, and commit range
    - Implement `MergeBase` via go-git `Repository.MergeBase`
    - Implement `RemoteHeadOID` via go-git `Repository.ResolveRevision` (equivalent to `git rev-parse <remote>/<branch>`)
    - Implement `CommitsNotOnRemote` via go-git `Repository.Log` over the range, returning hashes oldest-to-newest (reversed)
    - _Requirements: 5.4, 5.5, 6.1, 6.2_

  - [x] 5.4 Implement commit message body, name-status, and file contents
    - Implement `CommitMessageBody` via go-git `CommitObject.Message` (equivalent to `git log --format=%B -n 1 <hash>`)
    - Implement `NameStatus` via `os/exec` running `git diff --name-status <hash>^ <hash>` and returning the raw lines
    - Implement `FileContentsAtCommit` via the go-git commit tree blob reader, returning raw bytes without text-mode decoding
    - _Requirements: 7.1, 8.1, 9.1, 14.1_

  - [x] 5.5 Write table-driven tests for name-status parsing inputs
    - Standard table-driven tests over representative `git diff --name-status` line sets confirming the raw lines are returned unaltered for downstream parsing (behavior that is parseable without a live repo)
    - _Requirements: 8.1, 15.1, 15.2, 15.3_

- [x] 6. Implement the genqlient-backed GraphQL client
  - [x] 6.1 Configure genqlient and declare the operation
    - Create `internal/githubgql/genqlient.yaml` pointing at `schema.graphql`, `operations.graphql`, and `generated.go` in package `githubgql`
    - Commit `internal/githubgql/schema.graphql` covering `createCommitOnBranch` and its input and payload types
    - Create `internal/githubgql/operations.graphql` with the `createCommitOnBranch` mutation selecting `commit { oid }`
    - Add the `//go:generate go run github.com/Khan/genqlient` directive in `generated.go` and generate the client types
    - _Requirements: 11.1, 11.2_

  - [x] 6.2 Declare the GraphQLClient interface, input struct, and fake
    - In `internal/githubgql/client.go`, declare the package-level `GraphQLClient` interface with `CreateCommitOnBranch(ctx, input *CreateCommitInput) (string, error)` and the `CreateCommitInput` struct
    - Add a hand-written fake implementing `GraphQLClient` returning table-defined OIDs or sentinel errors
    - _Requirements: 11.2, 15.1, 15.2_

  - [x] 6.3 Implement the authed client wrapper and response interpretation
    - Build `graphql.NewClient("https://api.github.com/graphql", httpClient)` with a custom `http.RoundTripper` setting `Authorization: Bearer <github_token>` and a 600-second timeout
    - Map `CreateCommitInput` onto the generated input types (branch selector, `expectedHeadOid`, `fileChanges`, `message`) and invoke the generated function
    - Interpret the result in order: transport/HTTP error not convertible to `gqlerror.List` -> `ErrGithubAPI`; `gqlerror.List` with `Extensions["type"] == "STALE_DATA"` (message fallback) -> `ErrPartialPushFailure`; `gqlerror.List` without `STALE_DATA` -> `ErrGithubAPI`; nil error with nil/empty OID -> `ErrGithubAPI`; otherwise return the OID and log the created commit OID and branch
    - _Requirements: 11.1, 11.3, 11.4, 11.5, 11.6, 11.7, 11.8_

  - [x] 6.4 Write table-driven tests for GraphQL response interpretation
    - Standard table-driven tests (not property-based) driving the fake and the interpretation logic through: transport/HTTP error -> `ErrGithubAPI`; `STALE_DATA` -> `ErrPartialPushFailure`; non-`STALE_DATA` list -> `ErrGithubAPI`; nil/empty OID -> `ErrGithubAPI`; success returns OID; match sentinels with `errors.Is`
    - _Requirements: 11.4, 11.5, 11.6, 11.7, 11.8, 15.1, 15.2, 15.3_

- [x] 7. Implement file-change derivation and commit-message construction
  - [x] 7.1 Implement File_Change_Builder name-status derivation
    - In `internal/pusher/changes.go`, parse each `NameStatus` line on tab characters into a status field and filename fields
    - Map `A`/`M` to a `FileAddition` using the last filename with base64-encoded contents; status containing `R` to a `FileDeletion` of the first filename plus a `FileAddition` of the second filename with base64-encoded contents; `D` to a `FileDeletion` of the first filename
    - Read file bytes via `FileContentsAtCommit` and base64-encode them into a UTF-8 string; emit the debug raw-output log and the per-change status/filenames log (the latter independent of the debug gate)
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6, 8.7, 9.1, 9.2_

  - [x] 7.2 Implement CommitMessage construction
    - Build the headline from the first line of the trimmed `CommitMessageBody`, the body from the remaining lines joined by `\n`, then append `\n\n` plus `This commit was created from the local commit with hash <commit_hash>.`
    - _Requirements: 7.1, 7.2, 7.3, 7.4_

  - [x] 7.3 Write table-driven tests for file-change derivation
    - Standard table-driven tests over `A`, `M`, `R` (including scored `R100`), and `D` status lines producing the correct additions and deletions, including the rename deletion-plus-addition case and last-vs-first filename selection
    - _Requirements: 8.2, 8.3, 8.4, 8.5, 9.2, 15.1, 15.2, 15.3_

  - [x] 7.4 Write table-driven tests for commit-message construction
    - Standard table-driven tests over single-line and multi-line messages verifying headline, body, and the two-newline provenance line naming the local commit hash
    - _Requirements: 7.2, 7.3, 7.4, 15.1, 15.2, 15.3_

- [x] 8. Implement the Chunker
  - [x] 8.1 Implement chunking with threshold and logging
    - In `internal/pusher/chunker.go`, define `MaxPayloadBytes = 40 * 1024 * 1024` and estimate size as `len(json.Marshal(fc))`
    - Return a single chunk when at or below the threshold; otherwise place all deletions in the first chunk, start a new chunk only when the next addition would exceed the threshold and the current chunk already holds an addition, and append the final chunk only when it holds an addition or deletion
    - Emit the warning log with approximate megabytes on exceeding the threshold and the info log reporting the number of chunks produced
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6_

  - [x] 8.2 Write table-driven tests for chunking at and above the threshold
    - Standard table-driven tests confirming a single chunk at or below the threshold equal to the original, and above-threshold splitting that keeps deletions in the first chunk, starts a new chunk only when needed, and never appends an empty final chunk
    - _Requirements: 10.2, 10.3, 10.4, 10.5, 15.1, 15.2, 15.3_

- [x] 9. Implement Commit_Pusher orchestration
  - [x] 9.1 Implement push loop with expectedHeadOid chaining and re-verification
    - In `internal/pusher/pusher.go`, use `Merge_Base_OID` as `expectedHeadOid` for the first commit and the most recently created OID for subsequent commits
    - Before each subsequent commit, re-fetch the remote head and compare to the last created OID; on mismatch raise `ErrPartialPushFailure` reporting the count and hashes already pushed and the local hashes not pushed (`localHashes[len(created):]`)
    - Annotate multi-chunk headlines with ` (part <i+1>/<n>)` leaving the body unchanged; use the unmodified `CommitMessage` for single-chunk commits
    - Record each returned OID as the most recent commit, append to the created-hashes list, and log the total commits created after all are pushed
    - Define the `PushOptions` struct (`Config`, `MergeBaseOID`, `LocalHashes`)
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 12.7, 12.8_

  - [x] 9.2 Implement first-commit-vs-later error handling
    - On a `Remote_Branch_Diverged_Error` or `Github_API_Error` while no commit has yet been pushed, log an error stating the failure occurred on the first commit and the remote is unchanged, then propagate the error
    - On the same error classes after at least one commit has been pushed, propagate without claiming the remote is unchanged
    - _Requirements: 13.1, 13.2, 13.3_

  - [x] 9.3 Write table-driven tests for push orchestration and error handling
    - Standard table-driven tests driving the `GitClient` and `GraphQLClient` fakes covering: first commit uses merge base; subsequent commits use the previous OID; a re-verified remote-head mismatch yields `ErrPartialPushFailure` with pushed and unpushed hashes; multi-chunk part annotation and single-chunk passthrough; first-commit failure logs the no-changes message and propagates; later failure propagates without the no-changes claim
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 13.1, 13.2, 15.1, 15.2, 15.3_

- [x] 10. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. Wire the remote-verification flow and Commit_Collector into RunE
  - [x] 11.1 Implement Commit_Collector logging and wire collection
    - Wire `CommitsNotOnRemote` into the pipeline, log the count at INFO, and log the list of commit hashes at DEBUG
    - _Requirements: 6.1, 6.2, 6.3, 6.4_

  - [x] 11.2 Wire the remote-verification sequence in RunE
    - In `cmd/root.go` `RunE`, after branch validation run existence check (raise `ErrRemoteBranchNotFound` when absent) -> fetch -> rebase (raise `ErrRebaseAborted` on failure) -> merge-base -> `rev-parse` remote head -> divergence check (raise `ErrRemoteBranchDiverged` when head != merge base) -> proceed to collection; report an explicit error and exit non-zero if collection cannot proceed
    - _Requirements: 4.2, 4.3, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8, 14.1_

- [x] 12. Complete end-to-end wiring and final verification
  - [x] 12.1 Wire the full pipeline and top-level exit behavior
    - In `cmd/root.go` `RunE`, construct the `GitClient` and `GraphQLClient`, then run validation, remote verification, collection, per-commit build and chunk, and push in order, returning wrapped errors
    - Confine `os.Exit(1)` to `main`/the Execute wrapper so every propagated unrecoverable error yields a non-zero exit code
    - _Requirements: 2.7, 5.7, 13.3, 14.1_

  - [x] 12.2 Run the project verification workflow
    - Run `go vet ./...`, `golangci-lint run --fix ./...`, and `go test ./...` and resolve all diagnostics and test failures to a zero-diagnostics state
    - _Requirements: 15.1, 15.2, 15.3_

## Notes

* Tasks marked with `*` are optional test sub-tasks and can be skipped for a faster MVP; they use standard table-driven `testing` tests with testify assertions, never property-based testing.
* Each task references the specific requirement clauses it implements for traceability.
* Checkpoints ensure incremental validation before the integration phase.
* The single intentional deviation from the Python tool is the rebase-first step in Requirement 5, implemented in task 5.2 and wired in task 11.2.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.4"] },
    { "id": 2, "tasks": ["1.3", "2.1", "4.1", "6.1"] },
    { "id": 3, "tasks": ["2.2", "3.1", "4.2", "6.2"] },
    { "id": 4, "tasks": ["3.2", "5.1", "5.2", "5.3", "5.4", "6.3", "7.2", "8.1"] },
    { "id": 5, "tasks": ["5.5", "6.4", "7.1", "7.4", "8.2"] },
    { "id": 6, "tasks": ["7.3", "9.1"] },
    { "id": 7, "tasks": ["9.2", "11.1"] },
    { "id": 8, "tasks": ["9.3", "11.2"] },
    { "id": 9, "tasks": ["12.1"] },
    { "id": 10, "tasks": ["12.2"] }
  ]
}
```
