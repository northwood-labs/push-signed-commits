# Requirements Document

## Introduction

This feature changes how the `push-signed-commits` CLI authenticates its git
transport operations. The tool already recreates local commits on a remote
branch as verified commits through GitHub's GraphQL `createCommitOnBranch`
mutation, and it already authenticates that GraphQL side with a token read from
the `GITHUB_TOKEN` environment variable. However, the two git transport
operations the tool performs through go-git — the remote-branch existence check
(the equivalent of `git ls-remote`) and the remote-branch fetch (the equivalent
of `git fetch`) — currently run with no authentication and rely on whatever
transport the configured remote URL happens to use.

This feature makes those two git transport operations authenticate with the
same `GITHUB_TOKEN` credential the tool already requires, using HTTP basic auth
over an HTTPS remote URL. Because the target is a known GitHub repository, the
tool constructs the well-known HTTPS URL from the owner/name pair already
available as `cfg.OwnerRepo`, and never uses an SSH URL for the git remote. This
keeps git authentication consistent with the GraphQL side and makes the tool
behave the same locally and in CI, where an SSH agent is usually absent. The
feature also removes every remaining reference to the obsolete `github-token`
flag/input from the action definition, the README, and prior spec documents, so
the `GITHUB_TOKEN` environment variable is the single documented token source.

## Glossary

* **Tool**: The `push-signed-commits` CLI, including its command layer
  (`cmd` package) and its git client (`internal/gitclient` package).
* **Git_Client**: The production `gitclient.Client` that performs git
  operations through go-git, plus the `gitclient.GitClient` interface it
  satisfies.
* **GITHUB_TOKEN**: The environment variable whose value is the GitHub
  authentication token used by the Tool.
* **Owner_Repo**: The GitHub repository identifier in `owner/name` form, carried
  by `cfg.OwnerRepo` and sourced from the `--owner-repo` /
  `--repository-name-with-owner` flag.
* **HTTPS_Remote_URL**: The well-known GitHub HTTPS clone URL for the repository,
  of the form `https://github.com/<owner>/<repository>.git`.
* **Basic_Auth**: go-git HTTP basic authentication, expressed as
  `githttp.BasicAuth{Username: "x-access-token", Password: <token>}`, supplied
  as the `Auth` field of go-git `ListOptions` and `FetchOptions`.
* **Remote_Branch_Exists**: The `Git_Client` operation that reports whether a
  remote branch exists, performed through go-git `remote.ListContext` with
  `ListOptions` (the equivalent of `git ls-remote`).
* **Fetch_Remote_Branch**: The `Git_Client` operation that fetches a remote
  branch, performed through go-git `repo.FetchContext` with `FetchOptions` (the
  equivalent of `git fetch`).
* **Action_Definition**: The composite-action manifest `action.yml`.
* **Documentation**: The project `README.md`.
* **Prior_Spec_Documents**: The documents under
  `.kiro/specs/go-port-create-commits/` that reference the `--github-token`
  flag.
* **ErrGithubTokenMissing**: A sentinel error (to be defined in
  `cmd/errors.go`) indicating that `GITHUB_TOKEN` is unset or empty.

## Requirements

### Requirement 1: Read the authentication token from GITHUB_TOKEN

**User Story:** As an operator running the Tool, I want the Tool to take its
GitHub authentication token from the `GITHUB_TOKEN` environment variable, so
that git transport operations use the same credential the Tool already requires
for the GraphQL API.

#### Acceptance criteria

1. THE Tool SHALL read the authentication token from the GITHUB_TOKEN
   environment variable.
2. WHERE the GITHUB_TOKEN environment variable holds a non-empty value, THE Tool
   SHALL use that value as the token for the git transport operations.

### Requirement 2: Fail with a specific error when GITHUB_TOKEN is empty

**User Story:** As an operator, I want a clear, specific failure when
`GITHUB_TOKEN` is missing, so that I can correct the misconfiguration instead of
seeing an obscure transport error.

#### Acceptance criteria

1. IF the GITHUB_TOKEN environment variable is unset, THEN THE Tool SHALL fail
   with an error wrapping ErrGithubTokenMissing and SHALL exit with a non-zero
   exit code.
2. IF the GITHUB_TOKEN environment variable is set to an empty string, THEN THE
   Tool SHALL fail with an error wrapping ErrGithubTokenMissing and SHALL exit
   with a non-zero exit code.
3. WHEN the Tool fails because GITHUB_TOKEN is unset or empty, THE Tool SHALL
   report the failure before performing any git transport operation.

### Requirement 3: Construct the HTTPS remote URL from the owner/name pair

**User Story:** As an operator, I want the Tool to target the repository's
well-known HTTPS URL built from the owner/name pair I already provide, so that
git authentication does not depend on how the local remote URL happens to be
configured.

#### Acceptance criteria

1. THE Tool SHALL construct the HTTPS_Remote_URL from Owner_Repo as
   `https://github.com/<owner>/<repository>.git`, where `<owner>` and
   `<repository>` are the owner and name segments of Owner_Repo.
2. THE Tool SHALL use the constructed HTTPS_Remote_URL as the remote target for
   the Remote_Branch_Exists operation.
3. THE Tool SHALL use the constructed HTTPS_Remote_URL as the remote target for
   the Fetch_Remote_Branch operation.

### Requirement 4: Authenticate git transport with the token as HTTP basic auth

**User Story:** As an operator, I want both git transport operations to
authenticate with my GitHub token, so that private-repository access and CI runs
succeed without relying on ambient git credentials.

#### Acceptance criteria

1. WHEN the Git_Client performs the Remote_Branch_Exists operation, THE
   Git_Client SHALL supply Basic_Auth, with the username `x-access-token` and
   the GITHUB_TOKEN value as the password, as the `Auth` field of the go-git
   `ListOptions`.
2. WHEN the Git_Client performs the Fetch_Remote_Branch operation, THE
   Git_Client SHALL supply Basic_Auth, with the username `x-access-token` and
   the GITHUB_TOKEN value as the password, as the `Auth` field of the go-git
   `FetchOptions`.
3. THE Git_Client SHALL preserve the existing success semantics of the two
   operations: a missing remote branch is reported as a not-found result rather
   than a transport error, and an "already up to date" fetch is treated as
   success.

### Requirement 5: Never use the SSH URL for the git remote

**User Story:** As an operator, I want the Tool to always use the token-over-
HTTPS route and never an SSH URL, so that the Tool behaves consistently in CI
where an SSH agent is usually absent.

#### Acceptance criteria

1. THE Tool SHALL use the HTTPS_Remote_URL for every git transport operation.
2. THE Tool SHALL exclude any SSH-form remote URL from use in git transport
   operations.

### Requirement 6: Propagate the token through the Git_Client interface

**User Story:** As a developer maintaining the Tool, I want the token delivered
to the Git_Client through its interface, so that the production client and the
test fake both carry the authentication inputs explicitly.

#### Acceptance criteria

1. THE Git_Client interface SHALL accept, for the Remote_Branch_Exists
   operation, the inputs required to authenticate and target the
   HTTPS_Remote_URL: the HTTPS_Remote_URL and the GITHUB_TOKEN value.
2. THE Git_Client interface SHALL accept, for the Fetch_Remote_Branch
   operation, the inputs required to authenticate and target the
   HTTPS_Remote_URL: the HTTPS_Remote_URL and the GITHUB_TOKEN value.
3. THE test fake implementation of the Git_Client interface SHALL record the
   HTTPS_Remote_URL and the GITHUB_TOKEN value passed to the Remote_Branch_Exists
   and Fetch_Remote_Branch operations, so that tests can assert on the values
   supplied.

### Requirement 7: Remove all references to the --github-token flag

**User Story:** As an operator reading the documentation and the action
definition, I want a single documented token source, so that I am not misled by
references to an obsolete `github-token` flag/input.

#### Acceptance criteria

1. THE Action_Definition SHALL source the GITHUB_TOKEN environment variable
   without declaring a `github-token` action input.
2. THE Documentation SHALL describe the token exclusively as the GITHUB_TOKEN
   environment variable and SHALL omit every reference to a `github-token`
   input or `--github-token` flag.
3. THE Prior_Spec_Documents SHALL omit every reference to the `--github-token`
   flag.
4. THE Tool SHALL obtain the token exclusively from the GITHUB_TOKEN environment
   variable, with no `--github-token` command-line flag.
5. IF a `--github-token` command-line flag is supplied, THEN THE Tool SHALL
   reject it with a usage error and SHALL exit with a non-zero exit code.
