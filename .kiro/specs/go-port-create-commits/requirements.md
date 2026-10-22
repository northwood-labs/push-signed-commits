# Requirements Document

## Introduction

This feature ports the existing Python CLI tool `create_commits.py` to Go. The tool reads commits from a local git branch that are not yet on a corresponding remote branch and recreates each of those commits on the remote branch using GitHub's GraphQL `createCommitOnBranch` mutation. Because the mutation performs commit signing and authorship attribution on GitHub's side, the recreated commits are signed (verified) commits whose hashes differ from the local commits.

The Go port MUST achieve full behavioral parity with the Python implementation: the command-line interface (named flag arguments, defaults, and validation), the git interactions, the payload chunking logic, the GraphQL request construction, the error handling, and the logging behavior MUST all match the Python tool's observable behavior. The one intentional, documented exception is the rebase step in Requirement 5: the Go port attempts a rebase onto the latest upstream before verifying divergence, whereas the Python implementation performs no rebase. This deviation is deliberate and does not relax parity for any other behavior.

Testing for the Go port uses traditional, table-based Go tests (standard `testing` package with table-driven test cases). Property-based testing is explicitly NOT used for this feature.

## Glossary

* **CLI_Tool**: The Go command-line executable that ports `create_commits.py`.
* **Argument_Parser**: The component of the CLI_Tool that parses named command-line flag arguments and populates the configuration used by the rest of the tool.
* **Branch_Validator**: The component that validates local and remote branch names before any git or network operations occur.
* **Git_Client**: The component that runs local `git` subprocess commands and returns their output.
* **Commit_Collector**: The component that determines which local commits are not present on the remote branch.
* **File_Change_Builder**: The component that derives additions and deletions (a FileChanges object) for a single local commit.
* **Chunker**: The component that splits a FileChanges object into multiple chunks so each GraphQL payload stays under the payload threshold.
* **GraphQL_Client**: The component that sends the `createCommitOnBranch` mutation to the GitHub GraphQL API and interprets the response.
* **Commit_Pusher**: The component that orchestrates creating commits on the remote branch, tracking the expected head OID chain and re-verifying remote state between commits.
* **FileChanges**: An object with an `additions` list (each entry has a `path` and base64-encoded `contents`) and a `deletions` list (each entry has a `path`), matching GitHub's FileChanges GraphQL input object.
* **FileAddition**: An entry in the FileChanges additions list, consisting of a `path` and base64-encoded file `contents`.
* **FileDeletion**: An entry in the FileChanges deletions list, consisting of a `path`.
* **CommitMessage**: An object with a `headline` field and a `body` field, matching GitHub's CommitMessage GraphQL input object.
* **Expected_Head_OID**: The git commit OID that the GitHub mutation expects to be at the head of the remote branch immediately before a new commit is created.
* **Merge_Base_OID**: The commit OID returned by `git merge-base <local_branch> <remote_name>/<remote_branch>`, used as the parent OID for the first created commit.
* **Rebase_Step**: The step in which the Git_Client fetches the latest remote branch state and attempts to rebase the local branch onto `<remote_name>/<remote_branch_name>`, aborting without auto-resolving when the rebase cannot complete cleanly. This step is an intentional deviation from the Python implementation, which performs no rebase.
* **Payload_Threshold**: The maximum estimated JSON payload size per GraphQL request, set to 40 MB (40 * 1024 * 1024 bytes), chosen to stay under GitHub's 45 MB GraphQL payload limit.
* **Log_Level**: The verbosity level for logging, matching Python's logging levels (for example DEBUG, INFO, WARNING, ERROR), defaulting to `WARN`.
* **Partial_Push_Failure**: The error condition raised when some but not all commits were pushed because the remote branch head no longer matches the expected value partway through pushing.
* **Github_API_Error**: The error condition raised when the GitHub GraphQL API returns an error or an unexpected response format.
* **Remote_Branch_Diverged_Error**: The error condition raised when the remote branch head does not equal the merge base before pushing begins.
* **Remote_Branch_Not_Found_Error**: The error condition raised when the remote branch does not exist on the remote.

## Requirements

### Requirement 1: Command-Line Interface Parity

**User Story:** As an operator, I want the Go tool to accept the same inputs as the Python tool through named flags, so that existing automation can be updated to named flags and invocations remain order-independent.

#### Acceptance Criteria

1. THE Argument_Parser SHALL accept six named flags: `--github-token`, `--repository-name-with-owner`, `--local-branch-name`, `--remote-name`, `--remote-branch-name`, and `--log-level`.
2. THE Argument_Parser SHALL accept the named flags in any order.
3. WHERE the `--log-level` flag is omitted, THE Argument_Parser SHALL use the default value `WARN`.
4. IF any required flag (`--github-token`, `--repository-name-with-owner`, `--local-branch-name`, `--remote-name`, or `--remote-branch-name`) is omitted, THEN THE Argument_Parser SHALL report a usage error and exit with a non-zero exit code.
5. THE Argument_Parser SHALL treat the `--github-token` flag value as the GitHub personal access token used for GraphQL authentication.
6. THE Argument_Parser SHALL treat the `--repository-name-with-owner` flag value as the GitHub repository identifier in owner/name form used in the GraphQL mutation.
7. THE Argument_Parser SHALL provide each flag value to the component that consumes it without altering the flag value's string value.

### Requirement 2: Branch Name Validation

**User Story:** As an operator, I want branch names validated before any work begins, so that I receive a clear error when I accidentally include a ref prefix or remote name.

#### Acceptance Criteria

1. IF the local_branch_name begins with `origin/`, THEN THE Branch_Validator SHALL report an error stating that `origin/` must not be included in the local branch name, and THE CLI_Tool SHALL exit with a non-zero exit code.
2. IF the local_branch_name begins with `refs/heads/`, THEN THE Branch_Validator SHALL report an error stating that `refs/heads/` must not be included in the local branch name, and THE CLI_Tool SHALL exit with a non-zero exit code.
3. IF the local_branch_name begins with the remote_name followed by `/`, THEN THE Branch_Validator SHALL report an error stating that the remote name must not be included in the local branch name, and THE CLI_Tool SHALL exit with a non-zero exit code.
4. IF the remote_branch_name begins with `origin/`, THEN THE Branch_Validator SHALL report an error stating that `origin/` must not be included in the remote branch name, and THE CLI_Tool SHALL exit with a non-zero exit code.
5. IF the remote_branch_name begins with `refs/heads/`, THEN THE Branch_Validator SHALL report an error stating that `refs/heads/` must not be included in the remote branch name, and THE CLI_Tool SHALL exit with a non-zero exit code.
6. IF the remote_branch_name begins with the remote_name followed by `/`, THEN THE Branch_Validator SHALL report an error stating that the remote name must not be included in the remote branch name, and THE CLI_Tool SHALL exit with a non-zero exit code.
7. WHEN branch name validation completes without detecting a prohibited prefix, THE CLI_Tool SHALL proceed to the remote verification steps.

### Requirement 3: Logging Configuration

**User Story:** As an operator, I want to control log verbosity, so that I can run the tool quietly by default or with detailed diagnostics when troubleshooting.

#### Acceptance Criteria

1. WHEN the CLI_Tool starts, THE CLI_Tool SHALL configure logging at the level named by the log_level argument.
2. THE CLI_Tool SHALL emit informational, debug, warning, and error log messages that correspond to the same events the Python implementation logs at each level.
3. WHERE the log_level is `WARN`, THE CLI_Tool SHALL suppress informational and debug log messages.

### Requirement 4: Remote Branch Existence Verification

**User Story:** As an operator, I want the tool to confirm the remote branch exists before doing work, so that I do not get a confusing failure partway through.

#### Acceptance Criteria

1. WHEN verification begins, THE Git_Client SHALL run `git ls-remote --exit-code --heads <remote_name> <remote_branch_name>` to determine whether the remote branch exists.
2. IF the remote branch existence check returns a non-zero exit code, THEN THE CLI_Tool SHALL raise a Remote_Branch_Not_Found_Error stating that the remote branch does not exist on the remote, and SHALL exit with a non-zero exit code.
3. WHEN the remote branch existence check returns a zero exit code, THE CLI_Tool SHALL proceed to compute the merge base.

### Requirement 5: Merge Base and Divergence Verification

**User Story:** As an operator, I want the tool to first try rebasing my local branch onto the latest upstream and to abort without auto-resolving when that is not safe, so that I pull in recent remote changes when possible and never silently push on top of unexpected remote commits or rewrite already-pushed history.

#### Acceptance Criteria

1. WHEN the remote branch is confirmed to exist, THE Git_Client SHALL fetch the latest remote branch state by running `git fetch <remote_name> <remote_branch_name>` before attempting any rebase.
2. WHEN pulling in upstream changes, THE Git_Client SHALL attempt to rebase the local branch onto `<remote_name>/<remote_branch_name>` by running `git rebase <remote_name>/<remote_branch_name> <local_branch_name>`, so that local commits are replayed on top of the latest upstream commits. (This Rebase_Step is an intentional deviation from the Python implementation, which performs no rebase.)
3. IF the rebase reports conflicts or cannot complete cleanly, THEN THE Git_Client SHALL abort the rebase by running `git rebase --abort` to restore the local branch to its pre-rebase state, and THE CLI_Tool SHALL raise an error instructing the operator to reconcile the branches manually, and SHALL exit with a non-zero exit code, without auto-resolving any conflict.
4. WHEN the Rebase_Step completes successfully, THE Git_Client SHALL compute the Merge_Base_OID by running `git merge-base <local_branch_name> <remote_name>/<remote_branch_name>` and using the trimmed standard output.
5. WHEN verifying divergence, THE Git_Client SHALL run `git rev-parse <remote_name>/<remote_branch_name>` to obtain the current remote head OID as trimmed standard output.
6. IF the current remote head OID is not equal to the Merge_Base_OID, THEN THE CLI_Tool SHALL raise a Remote_Branch_Diverged_Error stating that the remote branch has diverged from the local branch, and SHALL exit with a non-zero exit code.
7. WHEN the current remote head OID equals the Merge_Base_OID, THE CLI_Tool SHALL proceed to collect local commits not on the remote branch.
8. IF the CLI_Tool cannot proceed to collect local commits not on the remote branch, THEN THE CLI_Tool SHALL report an explicit error and SHALL exit with a non-zero exit code.

### Requirement 6: Collect Local Commits Not On Remote

**User Story:** As an operator, I want the tool to identify exactly the commits that need to be pushed, in the correct order, so that the remote branch history is reconstructed faithfully.

#### Acceptance Criteria

1. WHEN collecting commits to push, THE Commit_Collector SHALL run `git rev-list <remote_name>/<remote_branch_name>..<local_branch_name>` to obtain the list of local commit hashes not present on the remote branch.
2. THE Commit_Collector SHALL order the resulting commit hashes from oldest to newest by reversing the order returned by `git rev-list`.
3. THE Commit_Collector SHALL log the count of commits found on the local branch that are not on the remote branch.
4. WHERE the Log_Level permits debug output, THE Commit_Collector SHALL log the list of commit hashes to be created on the remote branch.

### Requirement 7: Build Commit Message For Each Local Commit

**User Story:** As an operator, I want each recreated commit to preserve the original commit message and reference its source commit, so that the remote history is traceable back to the local commits.

#### Acceptance Criteria

1. WHEN building a CommitMessage for a local commit, THE File_Change_Builder SHALL obtain the full commit message by running `git log --format=%B -n 1 <commit_hash>` and trimming surrounding whitespace.
2. THE File_Change_Builder SHALL set the CommitMessage headline to the first line of the commit message.
3. THE File_Change_Builder SHALL set the CommitMessage body to the remaining lines of the commit message joined by newline characters.
4. THE File_Change_Builder SHALL append to the CommitMessage body two newline characters followed by the text `This commit was created from the local commit with hash <commit_hash>.`, substituting the actual local commit hash.

### Requirement 8: Derive File Changes For Each Local Commit

**User Story:** As an operator, I want each recreated commit to contain the same file changes as the source commit, so that the remote branch content matches the local branch.

#### Acceptance Criteria

1. WHEN deriving file changes for a commit, THE File_Change_Builder SHALL run `git diff --name-status <commit_hash>^ <commit_hash>` and process one change per output line.
2. THE File_Change_Builder SHALL split each change line on the tab character into a status field and one or more filename fields.
3. WHEN the status field is `A` or `M`, THE File_Change_Builder SHALL add a FileAddition whose path is the last filename field and whose contents are the base64-encoded contents of that file at the commit.
4. WHEN the status field contains `R`, THE File_Change_Builder SHALL add a FileDeletion for the first filename field (the old name) and a FileAddition whose path is the second filename field (the new name) and whose contents are the base64-encoded contents of the new file at the commit.
5. WHEN the status field is `D`, THE File_Change_Builder SHALL add a FileDeletion whose path is the first filename field.
6. WHERE the Log_Level permits debug output, THE File_Change_Builder SHALL log the raw `git diff --name-status` output.
7. THE File_Change_Builder SHALL log the detected status and filenames for each change independently of the debug-level check that gates the raw `git diff --name-status` output logging.

### Requirement 9: Read File Contents At A Commit

**User Story:** As an operator, I want file contents captured exactly, including binary files, so that recreated commits are byte-for-byte faithful.

#### Acceptance Criteria

1. WHEN reading a file's contents at a commit, THE Git_Client SHALL run `git show <commit_hash>:<filename>` and capture the standard output as raw bytes without text-mode decoding.
2. THE File_Change_Builder SHALL base64-encode the raw file bytes and represent the encoded result as a UTF-8 string for inclusion in a FileAddition.

### Requirement 10: Payload Size Estimation And Chunking

**User Story:** As an operator, I want large commits split across multiple GraphQL requests, so that pushes do not fail due to GitHub's payload size limit.

#### Acceptance Criteria

1. THE Chunker SHALL estimate a FileChanges payload size as the byte length of the UTF-8 encoded JSON serialization of the FileChanges object.
2. WHERE the estimated payload size of a FileChanges object is less than or equal to the Payload_Threshold, THE Chunker SHALL return a single chunk equal to the original FileChanges object.
3. WHILE splitting a FileChanges object that exceeds the Payload_Threshold, THE Chunker SHALL place all deletions in the first chunk and distribute additions across chunks.
4. WHEN adding an addition to the current chunk would make the current chunk's estimated size exceed the Payload_Threshold and the current chunk already contains at least one addition, THE Chunker SHALL finalize the current chunk and start a new chunk before adding the addition.
5. WHEN chunking completes, THE Chunker SHALL include the final chunk if and only if the final chunk contains at least one addition or at least one deletion, and THE Chunker SHALL never include a final chunk that contains no additions and no deletions.
6. WHEN a FileChanges object exceeds the Payload_Threshold, THE Chunker SHALL emit a warning log reporting the approximate size in megabytes and that the commit will be split, and SHALL emit an informational log reporting the number of chunks produced.

### Requirement 11: Create Commit Via GitHub GraphQL Mutation

**User Story:** As an operator, I want commits created through GitHub's signing mutation, so that the recreated commits are verified and properly attributed.

#### Acceptance Criteria

1. WHEN creating a commit, THE GraphQL_Client SHALL send an HTTP POST request to `https://api.github.com/graphql` with an `Authorization` header of `Bearer <github_token>`.
2. THE GraphQL_Client SHALL send the `createCommitOnBranch` mutation with a variables object whose input contains the branch repositoryNameWithOwner and branchName, the expectedHeadOid, the fileChanges object, and the message object.
3. THE GraphQL_Client SHALL use a request timeout of 600 seconds.
4. IF the response body does not contain a `data` field, THEN THE GraphQL_Client SHALL raise a Github_API_Error stating that the response was of an unexpected format and including the response content.
5. IF the response contains an `errors` array with any error whose `type` is `STALE_DATA`, THEN THE GraphQL_Client SHALL raise a Partial_Push_Failure stating that the expected head OID is stale and the operation is aborting.
6. IF the response contains an `errors` array without a `STALE_DATA` error, THEN THE GraphQL_Client SHALL raise a Github_API_Error that includes the error messages.
7. IF the response does not contain a created commit OID at data.createCommitOnBranch.commit.oid, THEN THE GraphQL_Client SHALL raise a Github_API_Error stating that the response was of an unexpected format and including the response content.
8. WHEN the response contains a created commit OID, THE GraphQL_Client SHALL return that commit OID and SHALL log the created commit OID and branch.

### Requirement 12: Expected Head OID Chaining And Push Orchestration

**User Story:** As an operator, I want commits pushed in order with each built on the previous one, so that the remote branch history is a faithful linear reconstruction of the local commits.

#### Acceptance Criteria

1. WHEN creating the first commit, THE Commit_Pusher SHALL use the Merge_Base_OID as the Expected_Head_OID.
2. WHEN creating each subsequent commit, THE Commit_Pusher SHALL use the OID of the most recently created commit as the Expected_Head_OID.
3. WHILE a prior commit has been pushed, THE Commit_Pusher SHALL, before creating the next commit, re-fetch the remote branch head OID and compare it to the OID of the most recently created commit.
4. IF the re-fetched remote branch head OID does not equal the OID of the most recently created commit, THEN THE Commit_Pusher SHALL raise a Partial_Push_Failure that reports the count and hashes of commits already pushed and the local commits that were not pushed, and SHALL exit with a non-zero exit code.
5. WHEN a commit's file changes are split into multiple chunks, THE Commit_Pusher SHALL annotate each chunk's headline with ` (part <chunk_index+1>/<chunk_count>)` while leaving the body unchanged.
6. WHEN a commit's file changes form a single chunk, THE Commit_Pusher SHALL use the unmodified CommitMessage for that commit.
7. WHEN each chunk is created, THE Commit_Pusher SHALL record the returned commit OID as the most recently created commit and append it to the list of created remote commit hashes.
8. WHEN all commits have been created, THE Commit_Pusher SHALL log the total number of commits created on the remote branch from the local branch.

### Requirement 13: Error Handling During Push

**User Story:** As an operator, I want clear, distinct errors depending on where a failure occurs, so that I know whether the remote was left in a partial state requiring reconciliation.

#### Acceptance Criteria

1. IF a Remote_Branch_Diverged_Error or Github_API_Error occurs while creating a commit and no commit has yet been pushed, THEN THE Commit_Pusher SHALL log an error stating that the failure occurred on the first commit and that no changes were made to the remote branch, and SHALL propagate the error.
2. IF a Remote_Branch_Diverged_Error or Github_API_Error occurs while creating a commit after at least one commit has been pushed, THEN THE Commit_Pusher SHALL propagate the error without claiming the remote branch is unchanged.
3. WHEN any unrecoverable error defined in this specification is propagated to the top level, THE CLI_Tool SHALL exit with a non-zero exit code.

### Requirement 14: Git Command Failure Handling

**User Story:** As an operator, I want git command failures surfaced rather than silently ignored, so that I can trust the tool's output.

#### Acceptance Criteria

1. IF a git command that the Python implementation runs with check enabled exits with a non-zero status, THEN THE Git_Client SHALL treat the failure as an error and THE CLI_Tool SHALL exit with a non-zero exit code.
2. WHEN running the remote branch existence check, THE Git_Client SHALL interpret the command's exit code rather than treating a non-zero exit code as a fatal error, consistent with Requirement 4.

### Requirement 15: Testing Approach

**User Story:** As a maintainer, I want the Go port covered by table-based tests, so that behavior is verified in a style consistent with the project's conventions.

#### Acceptance Criteria

1. THE CLI_Tool test suite SHALL use traditional table-based Go tests built on the standard `testing` package.
2. THE CLI_Tool test suite SHALL NOT use property-based testing.
3. THE CLI_Tool test suite SHALL cover branch name validation, file change derivation for A, M, R, and D status codes, payload chunking at and above the Payload_Threshold, commit message construction, and GraphQL response interpretation including STALE_DATA, other errors, missing data, and success.
