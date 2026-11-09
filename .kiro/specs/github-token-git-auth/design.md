# Design Document

## Overview

This feature changes the two go-git transport operations the tool performs — the remote-branch existence check (`git ls-remote`) and the remote-branch fetch (`git fetch`) — so they authenticate with the `GITHUB_TOKEN` credential the tool already reads for the GraphQL side, over an HTTPS remote URL built from the owner/name pair. Today both operations run with no `Auth` field and target whatever URL the configured remote happens to carry (which may be SSH). After this change, both operations supply HTTP basic auth (`x-access-token` as the username, the token as the password) and target the well-known HTTPS URL `https://github.com/<owner>/<repository>.git`, so the SSH remote URL is never consulted for transport.

The change is deliberately surgical. It touches the two authenticating methods on the `GitClient` interface and its production client, the hand-written fake, the single `pusher` carrier struct that reaches the second fetch call site, and the `cmd` layer where the token is read, the HTTPS URL is constructed, and a new `ErrGithubTokenMissing` sentinel is raised before any transport runs. It also removes every remaining reference to the obsolete `github-token` flag/input from the action manifest, the README, and the prior spec documents. No new third-party dependency is introduced: the go-git basic-auth type (`githttp.BasicAuth`) and the in-memory remote constructor (`gogit.NewRemote`) both ship with the already-vendored `github.com/go-git/go-git/v5` (v5.19.3).

The guiding project rule is minimum code that solves the problem, touching only what must change. Where go-git offers more than one way to apply a URL, this design picks the one that provably satisfies "never use the SSH URL" (Requirement 5) and documents the rejected alternative.

## Architecture

The token and the HTTPS URL originate in the `cmd` layer and flow down two paths to the two transport operations. The `cmd` layer owns all GitHub-specific knowledge (the environment variable name and the URL shape); the `gitclient` layer receives a finished URL and an opaque token and stays free of GitHub URL construction.

```text
                         os.Getenv("GITHUB_TOKEN")
                                   |
                                   v
                      +------------------------------+
                      | cmd/root.go runPipeline      |
                      |  * read token (fail early    |
                      |    with ErrGithubTokenMissing |
                      |    if unset/empty)           |
                      |  * build HTTPS URL from      |
                      |    cfg.OwnerRepo             |
                      +------------------------------+
                           |                     |
            token + URL    |                     |  token + URL
            (verify path)  |                     |  (push path)
                           v                     v
             +--------------------------+   +---------------------------+
             | cmd verifyRemoteState    |   | pusher.Push (PushOptions) |
             |  * RemoteBranchExists    |   |  carries token + URL in   |
             |  * FetchRemoteBranch     |   |  pusher.Config            |
             +--------------------------+   +---------------------------+
                           |                     |
                           |                     v
                           |         +---------------------------+
                           |         | pusher verifyRemoteHead   |
                           |         |  * FetchRemoteBranch      |
                           | +---------------------------+ |
                           |-------------------------------|
                           +----------+----------+
                                      v
                      +-------------------------------------+
                      | gitclient.Client                    |
                      |  * build in-memory remote at the    |
                      |    HTTPS URL                         |
                      |  * ListContext / FetchContext with  |
                      |    Auth = BasicAuth{x-access-token, |
                      |    token}                           |
                      +-------------------------------------+
                                      |
                                      v
                           https://github.com/<owner>/<repo>.git
```

There are exactly two call sites for the authenticating methods:

* `cmd/root.go` `verifyRemoteState` calls both `RemoteBranchExists` and `FetchRemoteBranch`. The `cmd` layer has `cfg.OwnerRepo` and the token directly, so it passes them straight through.
* `internal/pusher/pusher.go` `verifyRemoteHead` calls `FetchRemoteBranch` inside the push loop. The `pusher` package cannot import `cmd` (that would be an import cycle, as noted in the existing `Config` doc comment), so the token and HTTPS URL are added to the existing `pusher.Config` carrier and populated by `cmd` when it calls `pusher.Push`.

### Forcing the HTTPS URL with an in-memory remote

The central go-git mechanism is that transport targets must come from the HTTPS URL, not from the repository's stored remote configuration (which may be SSH). go-git resolves URLs differently for the two operations, so each is handled explicitly:

* `RemoteBranchExists` today calls `c.repo.Remote(remoteName)` and then `remote.ListContext(...)`, which uses the stored remote's URLs. Instead, the design constructs an in-memory remote bound to the HTTPS URL and lists against that:

  ```go
  remote := gogit.NewRemote(c.repo.Storer, &config.RemoteConfig{
      Name: remoteName,
      URLs: []string{httpsRemoteURL},
  })
  refs, err := remote.ListContext(ctx, &gogit.ListOptions{
      Auth: &githttp.BasicAuth{Username: "x-access-token", Password: token},
  })
  ```

  `NewRemote` does not persist anything to the repository config; it is an in-memory handle whose `URLs` are authoritative for the listing. The stored remote (possibly SSH) is never read.

* `FetchRemoteBranch` today calls `c.repo.FetchContext(...)` with a `RemoteName`. `FetchOptions` has no URL field, so `RemoteName` alone would again resolve the stored (possibly SSH) URL. Instead, the design fetches against an in-memory remote too, whose `FetchContext` accepts the same `FetchOptions` (including `RefSpecs` and `Auth`) but sources its URL from the in-memory `RemoteConfig`:

  ```go
  remote := gogit.NewRemote(c.repo.Storer, &config.RemoteConfig{
      Name: remoteName,
      URLs: []string{httpsRemoteURL},
  })
  err := remote.FetchContext(ctx, &gogit.FetchOptions{
      RefSpecs: []config.RefSpec{refSpec},
      Auth:     &githttp.BasicAuth{Username: "x-access-token", Password: token},
  })
  ```

  The refspec is unchanged (`+refs/heads/<branch>:refs/remotes/<remote>/<branch>`), so fetched refs still land under `refs/remotes/<remoteName>/<branch>` and the later `RemoteHeadOID` / `MergeBase` resolutions that use the `<remote>/<branch>` shorthand keep working. `remote.FetchContext` writes into the same object store (`c.repo.Storer`) that `c.repo.FetchContext` would, so the fetched objects are available to the rest of the repository exactly as before.

This is why `remoteName` is still required even though the stored remote's URL is no longer used for transport: the name is used to build the destination refspec and remote-tracking ref namespace that downstream resolution depends on. **Validates: Requirements 3.2, 3.3, 5.1, 5.2.**

**Rejected alternative.** Keeping `c.repo.Remote(remoteName)` / `c.repo.FetchContext(...)` and only adding `Auth` would authenticate correctly but would still use the stored remote URL, which may be SSH — directly violating Requirement 5. Temporarily rewriting the stored remote config to the HTTPS URL was also rejected: it mutates repository state, is not reversible on failure paths, and is more code than the in-memory remote. The in-memory remote is the minimal change that guarantees the HTTPS URL is authoritative.

### Where the HTTPS URL is constructed

The HTTPS URL is constructed in the `cmd` layer from `cfg.OwnerRepo` and passed down as a finished string. The `gitclient` layer receives the URL and the token and performs no GitHub-specific string building. **Rationale:** `cmd` already owns `cfg.OwnerRepo` and the `GITHUB_TOKEN` read for the GraphQL client, so keeping URL construction there co-locates all GitHub knowledge and keeps `gitclient` a transport-only package that can target any HTTPS URL. **Rejected alternative:** having `gitclient` accept owner/name and build the URL itself was rejected because it would push GitHub.com-specific URL shape into an otherwise host-agnostic git client and duplicate parsing of the `owner/name` slug that `cmd` already holds. **Validates: Requirement 3.1.**

## Components and interfaces

### GitClient interface (`internal/gitclient/gitclient.go`)

The two authenticating methods move from positional `remoteName, remoteBranchName` parameters to named options structs, matching the existing `RebaseOptions` / `MergeBaseOptions` / `CommitRangeOptions` pattern (passed by pointer). Both new structs carry the HTTPS URL and token alongside the existing selectors. This keeps each method within the project's parameter limit and makes the authentication inputs explicit at every call site. **Validates: Requirements 6.1, 6.2.**

```go
// RemoteBranchExists reports whether the remote branch exists, mirroring
// `git ls-remote --exit-code --heads <remote> <branch>` against the HTTPS
// remote URL using token authentication. A missing branch is reported as
// (false, nil); only unexpected failures return a non-nil error.
RemoteBranchExists(ctx context.Context, opts *RemoteBranchExistsOptions) (bool, error)

// FetchRemoteBranch runs the equivalent of `git fetch <remote> <branch>`
// against the HTTPS remote URL using token authentication.
FetchRemoteBranch(ctx context.Context, opts *FetchRemoteBranchOptions) error
```

New options structs, added to the existing `type (...)` block:

```go
// RemoteBranchExistsOptions carries the selectors and authentication inputs
// for RemoteBranchExists. It is passed by pointer.
RemoteBranchExistsOptions struct {
    RemoteName       string
    RemoteBranchName string
    HTTPSRemoteURL   string
    Token            string
}

// FetchRemoteBranchOptions carries the selectors and authentication inputs
// for FetchRemoteBranch. It is passed by pointer.
FetchRemoteBranchOptions struct {
    RemoteName       string
    RemoteBranchName string
    HTTPSRemoteURL   string
    Token            string
}
```

`RemoteName` is retained in both structs because it is still used to build the fetch refspec and the remote-tracking ref namespace, as explained in the Architecture section.

### Production client (`internal/gitclient/client.go`)

`RemoteBranchExists` and `FetchRemoteBranch` are rewritten to build an in-memory remote at `opts.HTTPSRemoteURL` and supply `Auth: &githttp.BasicAuth{Username: "x-access-token", Password: opts.Token}` on the `ListOptions` / `FetchOptions`. The new import is `githttp "github.com/go-git/go-git/v5/plumbing/transport/http"`.

Preserved success semantics (Requirement 4.3):

* `RemoteBranchExists` still treats `transport.ErrEmptyRemoteRepository` as `(false, nil)` and still reports a non-matching ref list as `(false, nil)`.
* `FetchRemoteBranch` still treats `gogit.NoErrAlreadyUpToDate` as success.
* The existing boundary sentinels `ErrRemoteBranchList` and `ErrRemoteFetch` continue to wrap unexpected failures; the `x-access-token` constant may be a small private constant to avoid a magic string. **Validates: Requirements 4.1, 4.2, 4.3.**

### Test fake (`internal/gitclient/gitclientfake/fake.go`)

The fake's two method signatures change to match the interface, and the two recorded-call structs gain the new fields so tests can assert on the HTTPS URL and token supplied:

```go
// RemoteBranchExistsCall records one invocation of Fake.RemoteBranchExists.
RemoteBranchExistsCall struct {
    RemoteName       string
    RemoteBranchName string
    HTTPSRemoteURL   string
    Token            string
}

// FetchRemoteBranchCall records one invocation of Fake.FetchRemoteBranch.
FetchRemoteBranchCall struct {
    RemoteName       string
    RemoteBranchName string
    HTTPSRemoteURL   string
    Token            string
}
```

The method bodies record `opts.RemoteName`, `opts.RemoteBranchName`, `opts.HTTPSRemoteURL`, and `opts.Token` from the passed options struct. **Validates: Requirement 6.3.**

### Pusher carrier (`internal/pusher/pusher.go`)

The second fetch call site, `verifyRemoteHead`, needs the token and HTTPS URL. The minimal, cycle-free way to deliver them is to extend the existing `pusher.Config` carrier (which `cmd` already populates) with two fields:

```go
Config struct {
    RepositoryNameWithOwner string
    RemoteName              string
    RemoteBranchName        string
    HTTPSRemoteURL          string // new: HTTPS clone URL for token-authenticated fetch
    Token                   string // new: GITHUB_TOKEN value for basic auth
}
```

`verifyRemoteHead` then builds a `*gitclient.FetchRemoteBranchOptions` from `opts.Config`:

```go
err := git.FetchRemoteBranch(ctx, &gitclient.FetchRemoteBranchOptions{
    RemoteName:       opts.Config.RemoteName,
    RemoteBranchName: opts.Config.RemoteBranchName,
    HTTPSRemoteURL:   opts.Config.HTTPSRemoteURL,
    Token:            opts.Config.Token,
})
```

No other pusher logic changes. **Validates: Requirement 6.2.**

### Command layer (`cmd/root.go`)

`runPipeline` gains three responsibilities, performed before any git transport operation:

1. Read the token once with `os.Getenv("GITHUB_TOKEN")`. If it is empty (covering both unset and set-to-empty, since `os.Getenv` returns `""` for both), return an error wrapping `ErrGithubTokenMissing`. This happens before `verifyRemoteState` and before the GraphQL client is used, satisfying the "before any transport" ordering. The existing `githubgql.NewClient(os.Getenv("GITHUB_TOKEN"))` call is updated to reuse the already-read token value rather than reading the environment a second time.
2. Construct the HTTPS URL from `cfg.OwnerRepo`. `cfg.OwnerRepo` is a single `owner/name` string; the URL is `"https://github.com/" + cfg.OwnerRepo + ".git"`. A tiny helper (for example `buildHTTPSRemoteURL(ownerRepo string) string`) keeps the shape in one place and is trivially unit-testable.
3. Thread the token and HTTPS URL into `verifyRemoteState` (which builds the two options structs) and into `pusher.Config` when calling `pusher.Push`.

`verifyRemoteState`'s signature gains the token and HTTPS URL (grouped per the parameter-limit rule if needed) and constructs `*gitclient.RemoteBranchExistsOptions` and `*gitclient.FetchRemoteBranchOptions` with all four fields.

No `--github-token` flag is added. Cobra rejects unknown flags by default with a usage error and non-zero exit, so Requirement 7.5 is satisfied by the flag's continued absence with no extra code. **Validates: Requirements 1.1, 1.2, 2.1, 2.2, 2.3, 3.1, 7.4, 7.5.**

### Sentinel error (`cmd/errors.go`)

A new sentinel is added inside the file's single `var (...)` block, with its own doc comment starting with the variable name (revive) and a user-understandable message (not referencing internal identifiers):

```go
// ErrGithubTokenMissing indicates that the GITHUB_TOKEN environment variable
// is not set or is empty, so the tool has no credential for authenticating
// its git and GraphQL operations.
ErrGithubTokenMissing = errors.New("the GITHUB_TOKEN environment variable is not set")
```

The call site wraps it with `fmt.Errorf("%w: ...", ErrGithubTokenMissing, ...)` if additional detail is warranted, and the error propagates up through `Execute` so the entrypoint exits non-zero. **Validates: Requirements 2.1, 2.2.**

### Documentation and manifest edits

These are non-Go edits required by Requirement 7. The `GITHUB_TOKEN` environment variable must still be provided to the composite step; only the obsolete flag/input references are removed.

* `action.yml`: remove the `github-token` input declaration. The composite step must still set `GITHUB_TOKEN` in its `env:` block; sourcing it from the standard `${{ github.token }}` (or a documented alternative) keeps the token available without the removed input. **Validates: Requirement 7.1.**
* `README.md`: remove the `github-token:` line from the `with:` example and describe the token solely as the `GITHUB_TOKEN` environment variable. **Validates: Requirement 7.2.**
* `.kiro/specs/go-port-create-commits/` (design.md, tasks.md, requirements.md): remove references to the `--github-token` flag. **Validates: Requirement 7.3.**

## Data models

The feature adds no persistent data. The in-memory data it introduces is:

* `RemoteBranchExistsOptions` and `FetchRemoteBranchOptions` — selectors plus authentication inputs (`HTTPSRemoteURL`, `Token`), passed by pointer.
* Two new fields on `pusher.Config` (`HTTPSRemoteURL`, `Token`).
* The go-git `githttp.BasicAuth{Username: "x-access-token", Password: token}` value constructed per operation and the in-memory `config.RemoteConfig{Name, URLs}` used to target the HTTPS URL.

The HTTPS URL is a derived value with a fixed shape: `https://github.com/<owner>/<repository>.git`, where `<owner>` and `<repository>` are the two segments of `cfg.OwnerRepo`.

## Correctness properties

_A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees._

The feature has two pure, input-varying pieces of logic worth testing as properties: the HTTPS URL construction from the owner/name pair, and the shape of the basic-auth credential handed to go-git. Both vary meaningfully with input (any owner/name, any token), are cheap to run many times, and test the tool's own logic rather than go-git or the network. The remaining acceptance criteria (token read, early failure, wiring the URL/token to both operations, preserved fetch semantics, doc/manifest edits) are verified by example-based unit tests and integration-style fake assertions, as recorded in the prework and detailed in the Testing Strategy.

### Property 1: Constructed URL is always the well-known HTTPS form

_For any_ owner segment and repository segment forming an `owner/name` pair, the constructed remote URL is exactly `https://github.com/<owner>/<repository>.git`, begins with `https://`, and never takes an `ssh://` or `git@` form.

**Validates: Requirements 3.1, 5.1, 5.2.**

### Property 2: Transport auth is always token basic auth

_For any_ non-empty token value, the credential supplied to both the remote-branch existence check and the fetch is HTTP basic auth whose username is exactly `x-access-token` and whose password is exactly the token value.

**Validates: Requirements 4.1, 4.2.**

## Error handling

* **Missing token.** When `GITHUB_TOKEN` is unset or empty, `runPipeline` returns an error wrapping `ErrGithubTokenMissing` before any git transport operation runs, and the tool exits non-zero via the existing `Execute` → `main` error path. `os.Exit` stays in `main`. **Validates: Requirements 2.1, 2.2, 2.3.**
* **Preserved boundary errors.** `ErrRemoteBranchList` and `ErrRemoteFetch` continue to wrap unexpected transport failures from the two operations. The "missing branch → `(false, nil)`" and "already-up-to-date → success" semantics are preserved, including the `transport.ErrEmptyRemoteRepository` special case. **Validates: Requirement 4.3.**
* **Unknown flag.** A supplied `--github-token` flag is rejected by Cobra's default unknown-flag handling with a usage error and non-zero exit; no custom handling is added. **Validates: Requirement 7.5.**
* **Authentication failures** from go-git (for example an invalid or unauthorized token) surface as wrapped `ErrRemoteBranchList` / `ErrRemoteFetch` errors through the existing boundary, unchanged by this feature.

## Testing strategy

The feature mixes pure logic (URL construction, auth shape) with wiring and I/O, so it uses a dual approach: property-based tests for the two pure, input-varying properties, and example/integration-style tests (using the existing fake) for wiring, early-failure ordering, and preserved semantics.

### Property-based tests

A property-based testing library for Go is used (the project's existing `pgregory.net/rapid` usage is the natural choice; no new PBT library is implemented from scratch). Each property test runs a minimum of 100 iterations and is tagged with a comment referencing the design property.

* **Property 1 (URL shape).** Generate random `owner` and `name` segments (including hyphens, dots, and mixed case) and assert the constructed URL equals `https://github.com/<owner>/<name>.git`, starts with `https://`, and contains neither `ssh://` nor `git@`. Tag: `// Feature: github-token-git-auth, Property 1: Constructed URL is always the well-known HTTPS form`.
* **Property 2 (auth shape).** Generate random non-empty token strings, build the basic-auth value the client uses, and assert the username is exactly `x-access-token` and the password equals the generated token. Tag: `// Feature: github-token-git-auth, Property 2: Transport auth is always token basic auth`. Where the auth value is constructed inside the client methods, this is factored into a tiny constructor so the property can assert on it without performing real network I/O.

### Unit and integration-style tests (using the fake)

* **Fake records URL and token.** Table-driven tests drive the code paths that call `RemoteBranchExists` and `FetchRemoteBranch` through the fake and assert that each recorded call carries the expected `HTTPSRemoteURL` (`https://github.com/<owner>/<repo>.git`) and `Token`. This covers both the `cmd` verify path and the `pusher` re-verify path. **Validates: Requirements 6.1, 6.2, 6.3.**
* **Missing-token failure.** A `cmd`-level test sets `GITHUB_TOKEN` empty/unset and asserts `runPipeline` (or the helper that reads the token) returns an error for which `errors.Is(err, ErrGithubTokenMissing)` is true, and that no transport call is recorded on the fake (confirming the "before any transport operation" ordering). **Validates: Requirements 2.1, 2.2, 2.3.**
* **Preserved semantics.** Client-level tests confirm a non-matching ref list yields `(false, nil)`, `transport.ErrEmptyRemoteRepository` yields `(false, nil)`, and `gogit.NoErrAlreadyUpToDate` is treated as success. **Validates: Requirement 4.3.**

### Test gotchas (project conventions)

If a `cmd`-level test drives `RunE` directly rather than through Cobra's `Execute`, two project-specific gotchas apply: the command context is nil unless `cmd.SetContext(context.Background())` is called first (otherwise `context.WithTimeout` panics), and any `huh` prompt will block without a TTY — so all config values must be supplied via viper/koanf so no prompt is reached. Prefer testing the extracted token-read and URL-construction helpers directly to avoid both issues where possible.

### Integration note

End-to-end verification that a real token authenticates against GitHub over HTTPS is out of scope for automated unit tests (it depends on an external service and a live credential). That is left to manual/CI validation; the unit and property tests cover the tool's own logic — URL shape, auth shape, wiring, and preserved semantics.
