---
name: stacked-pr
description: Use when a change builds on an open, unmerged PR, or when work is split into PRs that must merge in order. Triggers on stacked PRs, basing a branch or PR on another branch instead of development, GitHub's stack object, or pre-push checks failing on a branch whose base is not development.
metadata:
  author: moto-nrw
  version: "1.0.0"
---

# Stacked Pull Requests

A stack is two separate things, and both are needed.

The **base chain** is git: each PR targets the branch below it instead of `development`, so its diff carries only its own commits. The **stack object** is GitHub's, created over REST — GraphQL exposes stacks read-only. With the object GitHub draws the stack bar in both PRs, rebases the upper branches when a lower one merges, and retargets the survivors' bases.

Throughout, **parent** is the branch and PR directly below the one being worked on.

## Build the stack

1. Branch from the parent branch: `git switch <parent-branch> && git switch -c <new-branch>`.
   Branching from `development` puts the parent's commits into the new PR's diff.
2. Open the PR against the parent: `gh pr create --base <parent-branch> …`.
   Name the parent PR in the body and say it merges first.
3. Create the stack object, bottom PR first:
   ```bash
   echo '{"pull_requests": [3337, 3339]}' |
     gh api --method POST '/repos/{owner}/{repo}/stacks' --input -
   ```
   The numbers must be JSON integers, which is why the body goes in on stdin:
   `gh api -f 'pull_requests[]=3337'` sends strings and returns 422.
   To add a PR to an existing stack, POST the same body shape to
   `/repos/{owner}/{repo}/stacks/{stack_number}/add`.
4. Read it back: `gh api '/repos/{owner}/{repo}/stacks/{stack_number}'` lists
   `pull_requests` in stack order.

## Run the pre-push checks against the parent

`scripts/pre-push.sh` defaults its base to `origin/development`. On a stacked
branch the architecture ratchet then reads everything `development` gained since
the parent as a regression introduced by this branch. Give it the stack parent,
which is the base CI compares against:

```bash
bash scripts/pre-push.sh <parent-branch>
```

Green there is the real result, so push with `git push --no-verify` and record in
the PR body that the suite ran against the parent. (The commit hook stays on:
`git commit --no-verify` is blocked and stays blocked.)

## Keep the stack current

When `development` moves, merge it into the **parent** branch, then merge the
parent into the child. Merging `development` straight into the child pulls its
commits into the child's diff, because GitHub diffs from the merge-base with the
parent branch.

## Merge

Merge bottom-up. Through the API a stacked PR needs the asynchronous endpoint,
`PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge-async`; the synchronous
`PUT …/merge` behind `gh pr merge` refuses a stacked PR. The web UI merge button
takes the right path on its own.

## Endpoints

Full reference: [Stacked pull requests APIs and webhooks](https://docs.github.com/en/pull-requests/reference/stacked-pull-requests-apis-and-webhooks).

| Do | Call |
|---|---|
| List stacks | `GET /repos/{owner}/{repo}/stacks` |
| Read one | `GET /repos/{owner}/{repo}/stacks/{stack_number}` |
| Create | `POST /repos/{owner}/{repo}/stacks` + `{"pull_requests": [<bottom>, …, <top>]}` |
| Extend | `POST /repos/{owner}/{repo}/stacks/{stack_number}/add` + same body |
| Dissolve or detach | `POST /repos/{owner}/{repo}/stacks/{stack_number}/unstack` |
| Read from a PR | REST: `stack` on the PR resource. GraphQL: `stack` and `stackEntry` on `PullRequest` |
