# Implementation Plan: GitHub Token Git Auth

## Overview

This plan implements token-authenticated git transport over an HTTPS remote URL, following the approved design exactly. The work is surgical: it changes the two authenticating methods on the `GitClient` interface and its production client, updates the hand-written fake, extends the `pusher.Config` carrier, and wires token reading plus HTTPS URL construction into the `cmd` layer behind a new `ErrGithubTokenMissing` sentinel. It then removes the obsolete `github-token` flag/input references from the manifest, the README, and the prior spec documents.

The tasks are sequenced so the code compiles at each step: the interface and options structs change first, then every implementor and caller is updated in the same wave or the next, with tests written alongside the code they cover. Each task cites the requirement numbers and design sections it implements.

Property-based tests use `pgregory.net/rapid` as named in the design's Testing Strategy. The module does not yet depend on `rapid`, so the first property task adds the dependency via `go get pgregory.net/rapid` before writing the test.

## Tasks

* [x] 1. Change the GitClient interface and add the options structs
  * [x] 1.1 Add the options structs and change the two method signatures
    * In `internal/gitclient/gitclient.go`, add `RemoteBranchExistsOptions` and `FetchRemoteBranchOptions` to the existing `type (...)` block, each with fields `RemoteName`, `RemoteBranchName`, `HTTPSRemoteURL`, `Token` (all `string`), matching the `RebaseOptions`/`MergeBaseOptions` doc-comment style and the "passed by pointer" note
    * Change the interface methods to `RemoteBranchExists(ctx context.Context, opts *RemoteBranchExistsOptions) (bool, error)` and `FetchRemoteBranch(ctx context.Context, opts *FetchRemoteBranchOptions) error`
    * Update the two method doc comments to describe authenticating against the HTTPS remote URL with token auth, preserving the `(false, nil)` missing-branch semantics wording
    * _Requirements: 6.1, 6.2_
    * _Design: Components and interfaces — GitClient interface_

* [x] 2. Rewrite the production client to use an in-memory HTTPS remote with basic auth
  * [x] 2.1 Add the auth username constant and basic-auth constructor
    * In `internal/gitclient/client.go`, add the new import `githttp "github.com/go-git/go-git/v5/plumbing/transport/http"`
    * Add a small private constant for the `x-access-token` username (avoid the magic string) in the file's `const` block, creating the block in `const -> var -> type -> func` order if none exists
    * Add a tiny private constructor (for example `newBasicAuth(token string) *githttp.BasicAuth`) returning `&githttp.BasicAuth{Username: <constant>, Password: token}`, so Property 2 can assert on the auth shape without network I/O
    * _Requirements: 4.1, 4.2_
    * _Design: Production client; Correctness properties — Property 2_

  * [x] 2.2 Rewrite `RemoteBranchExists` and `FetchRemoteBranch`
    * Change both method signatures to accept the pointer options structs from task 1
    * In `RemoteBranchExists`, build an in-memory remote via `gogit.NewRemote(c.repo.Storer, &config.RemoteConfig{Name: opts.RemoteName, URLs: []string{opts.HTTPSRemoteURL}})` and call `remote.ListContext(ctx, &gogit.ListOptions{Auth: newBasicAuth(opts.Token)})`
    * Preserve semantics: `transport.ErrEmptyRemoteRepository` returns `(false, nil)`; a non-matching ref list returns `(false, nil)`; wrap other failures with `ErrRemoteBranchList`
    * In `FetchRemoteBranch`, build the same in-memory remote and call `remote.FetchContext(ctx, &gogit.FetchOptions{RefSpecs: []config.RefSpec{refSpec}, Auth: newBasicAuth(opts.Token)})`, keeping the existing refspec `+refs/heads/<branch>:refs/remotes/<remote>/<branch>`
    * Preserve semantics: treat `gogit.NoErrAlreadyUpToDate` as success; wrap other failures with `ErrRemoteFetch`
    * Update both method doc comments to reflect the HTTPS-URL target and token auth
    * _Requirements: 4.1, 4.2, 4.3, 3.2, 3.3, 5.1, 5.2_
    * _Design: Forcing the HTTPS URL with an in-memory remote; Production client; Error handling_

  * [x] 2.3 Write property test for transport auth shape
    * Create `internal/gitclient/client_property_test.go`; run `go get pgregory.net/rapid` first so the module depends on it
    * **Property 2: Transport auth is always token basic auth**
    * **Validates: Requirements 4.1, 4.2**
    * Generate random non-empty token strings with `rapid`, call `newBasicAuth(token)`, assert `Username == "x-access-token"` and `Password == token`; minimum 100 iterations
    * Tag with `// Feature: github-token-git-auth, Property 2: Transport auth is always token basic auth`

  * [x] 2.4 Write unit tests for preserved client semantics
    * In `internal/gitclient/client_test.go` (create if absent), assert a non-matching ref list yields `(false, nil)`, `transport.ErrEmptyRemoteRepository` yields `(false, nil)`, and `gogit.NoErrAlreadyUpToDate` is treated as success by `FetchRemoteBranch`
    * _Requirements: 4.3_
    * _Design: Testing strategy — Preserved semantics_

* [x] 3. Update the test fake to the new signatures and recorded fields
  * In `internal/gitclient/gitclientfake/fake.go`, add `HTTPSRemoteURL` and `Token` string fields to `RemoteBranchExistsCall` and `FetchRemoteBranchCall`
  * Change `Fake.RemoteBranchExists` and `Fake.FetchRemoteBranch` to accept the pointer options structs and record `opts.RemoteName`, `opts.RemoteBranchName`, `opts.HTTPSRemoteURL`, `opts.Token`
  * Keep the compile-time `var _ gitclient.GitClient = (*Fake)(nil)` assertion satisfied
  * _Requirements: 6.3_
  * _Design: Test fake_

* [x] 4. Extend the pusher carrier and re-verify path
  * [x] 4.1 Add token and HTTPS URL to `pusher.Config` and update `verifyRemoteHead`
    * In `internal/pusher/pusher.go`, add `HTTPSRemoteURL` and `Token` string fields to the `Config` struct with doc comments matching the existing field style
    * In `verifyRemoteHead`, build `&gitclient.FetchRemoteBranchOptions{RemoteName: opts.Config.RemoteName, RemoteBranchName: opts.Config.RemoteBranchName, HTTPSRemoteURL: opts.Config.HTTPSRemoteURL, Token: opts.Config.Token}` and pass it to `git.FetchRemoteBranch`
    * Leave all other pusher logic unchanged
    * _Requirements: 6.2_
    * _Design: Pusher carrier_

  * [x] 4.2 Update pusher tests for the new fetch call shape
    * In `internal/pusher/pusher_test.go`, populate `Config.HTTPSRemoteURL` and `Config.Token` in the test `PushOptions`, and assert the recorded `FetchRemoteBranchCalls[0]` carries the expected `HTTPSRemoteURL` and `Token` in addition to the existing `RemoteName`/`RemoteBranchName` assertions
    * _Requirements: 6.2, 6.3_
    * _Design: Testing strategy — Fake records URL and token (pusher re-verify path)_

* [x] 5. Add the ErrGithubTokenMissing sentinel
  * In `cmd/errors.go`, add `ErrGithubTokenMissing` inside the single `var (...)` block with its own doc comment starting with the variable name (revive), using a user-facing message such as `"the GITHUB_TOKEN environment variable is not set"`
  * _Requirements: 2.1, 2.2_
  * _Design: Sentinel error (cmd/errors.go)_

* [x] 6. Wire token reading and HTTPS URL construction into the cmd layer
  * [x] 6.1 Add the `buildHTTPSRemoteURL` helper
    * In `cmd/root.go`, add `buildHTTPSRemoteURL(ownerRepo string) string` returning `"https://github.com/" + ownerRepo + ".git"`, with a doc comment describing the fixed HTTPS shape
    * _Requirements: 3.1_
    * _Design: Where the HTTPS URL is constructed; Correctness properties — Property 1_

  * [x] 6.2 Write property test for the constructed URL shape
    * Create `cmd/root_property_test.go`
    * **Property 1: Constructed URL is always the well-known HTTPS form**
    * **Validates: Requirements 3.1, 5.1, 5.2**
    * Generate random `owner` and `name` segments (hyphens, dots, mixed case) with `rapid`, assert `buildHTTPSRemoteURL(owner+"/"+name) == "https://github.com/"+owner+"/"+name+".git"`, that it starts with `https://`, and contains neither `ssh://` nor `git@`; minimum 100 iterations
    * Tag with `// Feature: github-token-git-auth, Property 1: Constructed URL is always the well-known HTTPS form`

  * [x] 6.3 Read GITHUB_TOKEN early in `runPipeline` and thread it down
    * In `runPipeline`, read `os.Getenv("GITHUB_TOKEN")` once; if the value is empty, return an error wrapping `ErrGithubTokenMissing`, placed before `verifyRemoteState` and before the GraphQL client is constructed so no git transport runs first
    * Reuse the read token value for `githubgql.NewClient(token)` instead of reading the environment a second time
    * Compute `httpsURL := buildHTTPSRemoteURL(cfg.OwnerRepo)` and pass `token` and `httpsURL` into `verifyRemoteState` and into `pusher.Config` (set `HTTPSRemoteURL` and `Token`) when calling `pusher.Push`
    * _Requirements: 1.1, 1.2, 2.1, 2.2, 2.3, 3.1_
    * _Design: Command layer (cmd/root.go); Error handling — Missing token_

  * [x] 6.4 Update `verifyRemoteState` to build the options structs
    * Change `verifyRemoteState` to receive the token and HTTPS URL, grouping its parameters into a pointer input struct if it would exceed the 4-parameter limit (per the revive argument-limit rule), defined in `cmd/root.go`'s `type (...)` block
    * Construct `&gitclient.RemoteBranchExistsOptions{...}` and `&gitclient.FetchRemoteBranchOptions{...}` with all four fields (`RemoteName`, `RemoteBranchName`, `HTTPSRemoteURL`, `Token`) and pass them to the two client calls
    * _Requirements: 3.2, 3.3, 5.1, 5.2, 6.1, 6.2_
    * _Design: Command layer (cmd/root.go)_

* [x] 7. Checkpoint - Ensure all tests pass
  * Ensure all tests pass, ask the user if questions arise.

* [x] 8. Add cmd-level wiring and failure tests using the fake
  * [x] 8.1 Write fake-based test for the cmd verify path URL and token
    * In a `cmd` test (for example `cmd/root_test.go`), drive the verify path through `gitclientfake.Fake` and assert the recorded `RemoteBranchExistsCalls[0]` and `FetchRemoteBranchCalls[0]` carry `HTTPSRemoteURL == "https://github.com/<owner>/<repo>.git"` and the expected `Token`
    * Honor the test gotchas: call `cmd.SetContext(context.Background())` before invoking `RunE` directly, and supply all config via viper/koanf so no `huh` prompt blocks
    * _Requirements: 6.1, 6.2, 6.3_
    * _Design: Testing strategy — Fake records URL and token (cmd verify path); Test gotchas_

  * [x] 8.2 Write missing-token failure and ordering test
    * With `GITHUB_TOKEN` empty or unset, assert the pipeline returns an error for which `errors.Is(err, ErrGithubTokenMissing)` is true, and that no transport call is recorded on the fake (no `RemoteBranchExistsCalls`, no `FetchRemoteBranchCalls`), confirming the before-any-transport ordering
    * _Requirements: 2.1, 2.2, 2.3_
    * _Design: Error handling — Missing token; Testing strategy — Missing-token failure_

* [x] 9. Remove --github-token references from the manifest and documentation
  * [x] 9.1 Update `action.yml`
    * Remove the `github-token` input declaration; keep `GITHUB_TOKEN` in the composite step's `env:` block sourced appropriately (for example `${{ github.token }}` or a documented alternative)
    * _Requirements: 7.1_
    * _Design: Documentation and manifest edits_

  * [x] 9.2 Update `README.md`
    * Remove the `github-token:` line from the `with:` example and describe the token solely as the `GITHUB_TOKEN` environment variable
    * _Requirements: 7.2_
    * _Design: Documentation and manifest edits_

  * [x] 9.3 Scrub prior spec documents
    * Remove every reference to the `--github-token` flag from `.kiro/specs/go-port-create-commits/requirements.md`, `design.md`, and `tasks.md`
    * _Requirements: 7.3_
    * _Design: Documentation and manifest edits_

* [x] 10. Final checkpoint - Verify zero diagnostics and passing tests
  * Run `getDiagnostics` on every touched Go file and resolve all reported issues
  * Run `golangci-lint run --fix ./...` from the project root, then `go vet ./...`, confirming a clean build
  * Run `go test` for `internal/gitclient`, `internal/gitclient/gitclientfake`, `internal/pusher`, and `cmd`, confirming all tests pass
  * Ensure all tests pass, ask the user if questions arise.

## Notes

* Tasks marked with `*` are optional and can be skipped for faster MVP; they are the unit, property, and integration tests.
* Each task references specific requirements for traceability and the design sections it implements.
* Checkpoints ensure incremental validation at natural breaks.
* Property tests validate the two universal correctness properties (URL shape, auth shape); unit and fake-based tests validate wiring, early-failure ordering, and preserved semantics.
* `pgregory.net/rapid` is added to the module by the first property task, matching the design's named PBT library.

## Task dependency graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["2.1", "3", "5", "6.1"] },
    { "id": 2, "tasks": ["2.2", "4.1", "6.3"] },
    { "id": 3, "tasks": ["2.3", "2.4", "4.2", "6.2", "6.4"] },
    { "id": 4, "tasks": ["8.1", "8.2", "9.1", "9.2", "9.3"] }
  ]
}
```
