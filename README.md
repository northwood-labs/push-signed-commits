# GitHub Actions: Push Signed Commits

This uses the `createCommitOnBranch` GraphQL mutation to allow GitHub Apps to push _verified_ commits.

Since this action changes the content of the commits (by deriving authorship information and signing them), the hashes of the commits pushed to the remote will be different from the ones that were created locally. This is expected and is a result of the commit being signed.

The action is intended to be used by GitHub Actions as a follow-up to a commit step. For example:

```yaml
- name: Generate an app token
  id: generate-token
  uses: actions/create-github-app-token@v1
  with:
    app-id: ${{ env.GITHUB_APP_ID }}
    private-key: ${{ env.GITHUB_APP_PRIVATE_KEY }}
    # etc.

- name: Get GitHub App User ID
  id: get-user-id
  env:
    GH_TOKEN: ${{ steps.gh_token.outputs.token }}
    STEPS_GH_TOKEN_OUTPUTS_APP_SLUG: ${{ steps.gh_token.outputs.app-slug }}
  run: |
    USER_ID=$(gh api "/users/${STEPS_GH_TOKEN_OUTPUTS_APP_SLUG}[bot]" --jq .id)
    echo "user-id=${USER_ID}" >> "$GITHUB_OUTPUT"

- name: Set Git config
  env:
    GH_TOKEN: ${{ steps.gh_token.outputs.token }}
    STEPS_GH_TOKEN_OUTPUTS_APP_SLUG: ${{ steps.gh_token.outputs.app-slug }}
    STEPS_GET_USER_ID_OUTPUTS_USER_ID: ${{ steps.get-user-id.outputs.user-id }}
  run: |
    git config --global url."https://x-access-token:${GH_TOKEN}@github.com/".insteadOf "https://github.com/"
    git config --global advice.detachedHead false
    git config --global user.name "${STEPS_GH_TOKEN_OUTPUTS_APP_SLUG}[bot]"
    git config --global user.email "${STEPS_GET_USER_ID_OUTPUTS_USER_ID}+${STEPS_GH_TOKEN_OUTPUTS_APP_SLUG}[bot]@users.noreply.github.com"
    gh auth setup-git

- name: Create a commit
  id: create-commit
  env:
    GITHUB_TOKEN: ${{ steps.generate-token.outputs.token }}
  run: |
    touch "test.txt"
    echo "Hello, world, from the first commit!" >> "test.txt"

    git add "test.txt"
    git commit -m "Once pushed to the remote, this commit will appear as authored by the GitHub App and verified"

- name: Push commits as verified
  uses: northwood-labs/push-signed-commits@v2
  with:
    github-token: ${{ steps.generate-token.outputs.token }}
    local_branch_name: main
    remote_name: origin
    remote_branch_name: main
```

As a final step, the local branch will be reset to match the remote branch using `git reset --hard`

## Background and context

In 2021, Github released the `createCommitOnBranch` GraphQL mutation, where you no longer need to manually create blobs and trees via separate API calls before creating the commit. This allows you to add, update, or delete multiple files in a single API call.

This GraphQL API extracts authorship information from the credential used for authentication, and automatically marks commits created using GitHub App installation credentials as **verified**.

See <https://github.blog/changelog/2021-09-13-a-simpler-api-for-authoring-commits/> for more information.
