---
name: gh-cli
description: GitHub CLI (gh) command reference and usage patterns. Use when working with GitHub from the command line - PRs, issues, repos, workflows, releases, and API calls.
---

# GitHub CLI (gh) Reference

GitHub CLI brings GitHub to your terminal. Use `gh` for pull requests, issues, repos, workflows, and more.

## Quick Start

```bash
# Authenticate
gh auth login

# Check status
gh auth status
```

## Command Reference

### Authentication (`gh auth`)

| Command | Description |
|---------|-------------|
| `gh auth login` | Authenticate with GitHub |
| `gh auth logout` | Log out of GitHub |
| `gh auth refresh` | Refresh stored credentials |
| `gh auth setup-git` | Configure git to use gh as credential helper |
| `gh auth status` | View authentication status |
| `gh auth switch` | Switch between accounts |
| `gh auth token` | Print or manage auth tokens |

### Pull Requests (`gh pr`)

| Command | Description |
|---------|-------------|
| `gh pr create` | Create a pull request |
| `gh pr list` | List pull requests |
| `gh pr status` | Show PR status for current branch |
| `gh pr view [number]` | View PR details |
| `gh pr checkout <number>` | Check out a PR branch |
| `gh pr merge [number]` | Merge a PR |
| `gh pr close [number]` | Close a PR |
| `gh pr reopen [number]` | Reopen a PR |
| `gh pr edit [number]` | Edit PR title, body, labels, etc. |
| `gh pr comment [number]` | Add a comment |
| `gh pr review [number]` | Submit a review |
| `gh pr diff [number]` | View PR diff |
| `gh pr checks [number]` | View CI check status |
| `gh pr ready [number]` | Mark as ready for review |
| `gh pr revert <number>` | Revert a merged PR |
| `gh pr update-branch` | Update PR branch with base |
| `gh pr lock/unlock` | Lock/unlock conversation |

**Common PR workflows:**

```bash
# Create PR interactively
gh pr create

# Create PR with title and body
gh pr create --title "Add feature" --body "Description here"

# Create draft PR
gh pr create --draft

# Merge with squash
gh pr merge --squash

# View PR in browser
gh pr view --web
```

### Issues (`gh issue`)

| Command | Description |
|---------|-------------|
| `gh issue create` | Create an issue |
| `gh issue list` | List issues |
| `gh issue status` | Show issue status |
| `gh issue view <number>` | View issue details |
| `gh issue close <number>` | Close an issue |
| `gh issue reopen <number>` | Reopen an issue |
| `gh issue edit <number>` | Edit issue |
| `gh issue comment <number>` | Add a comment |
| `gh issue delete <number>` | Delete an issue |
| `gh issue transfer <number>` | Transfer to another repo |
| `gh issue develop <number>` | Create linked branch |
| `gh issue pin/unpin` | Pin/unpin issue |
| `gh issue lock/unlock` | Lock/unlock conversation |

**Common issue workflows:**

```bash
# Create issue interactively
gh issue create

# Create with labels and assignee
gh issue create --title "Bug" --label "bug" --assignee "@me"

# List open bugs assigned to me
gh issue list --label "bug" --assignee "@me"

# Create branch from issue
gh issue develop 123 --checkout
```

### Repositories (`gh repo`)

| Command | Description |
|---------|-------------|
| `gh repo create` | Create a repository |
| `gh repo clone <repo>` | Clone a repository |
| `gh repo fork [repo]` | Fork a repository |
| `gh repo view [repo]` | View repo details |
| `gh repo list [owner]` | List repositories |
| `gh repo edit` | Edit repo settings |
| `gh repo delete <repo>` | Delete a repository |
| `gh repo rename <new-name>` | Rename a repository |
| `gh repo sync` | Sync fork with upstream |
| `gh repo archive <repo>` | Archive a repository |
| `gh repo unarchive <repo>` | Unarchive a repository |
| `gh repo set-default` | Set default repository |
| `gh repo deploy-key` | Manage deploy keys |
| `gh repo autolink` | Manage autolinks |
| `gh repo gitignore` | View gitignore templates |
| `gh repo license` | View license templates |

**Common repo workflows:**

```bash
# Create new repo from current directory
gh repo create --source=. --push

# Create private repo
gh repo create my-project --private

# Clone and cd into repo
gh repo clone owner/repo -- --depth=1

# Sync fork with upstream
gh repo sync owner/fork
```

### Workflows & Actions (`gh workflow`, `gh run`)

**Workflows:**

| Command | Description |
|---------|-------------|
| `gh workflow list` | List workflows |
| `gh workflow view [id]` | View workflow details |
| `gh workflow run <workflow>` | Trigger a workflow |
| `gh workflow enable <workflow>` | Enable a workflow |
| `gh workflow disable <workflow>` | Disable a workflow |

**Runs:**

| Command | Description |
|---------|-------------|
| `gh run list` | List recent runs |
| `gh run view [id]` | View run details |
| `gh run watch [id]` | Watch run in real-time |
| `gh run rerun [id]` | Re-run a workflow |
| `gh run cancel [id]` | Cancel a run |
| `gh run delete [id]` | Delete a run |
| `gh run download [id]` | Download artifacts |

**Common workflow patterns:**

```bash
# Trigger workflow with inputs
gh workflow run deploy.yml -f environment=staging

# Watch current branch's latest run
gh run watch

# Download artifacts from run
gh run download <run-id>

# Re-run failed jobs only
gh run rerun <run-id> --failed
```

### Releases (`gh release`)

| Command | Description |
|---------|-------------|
| `gh release create <tag>` | Create a release |
| `gh release list` | List releases |
| `gh release view [tag]` | View release details |
| `gh release edit <tag>` | Edit a release |
| `gh release delete <tag>` | Delete a release |
| `gh release download [tag]` | Download release assets |
| `gh release upload <tag>` | Upload assets to release |
| `gh release delete-asset` | Delete an asset |
| `gh release verify <tag>` | Verify release authenticity |
| `gh release verify-asset` | Verify asset authenticity |

**Common release patterns:**

```bash
# Create release with auto-generated notes
gh release create v1.0.0 --generate-notes

# Create release with assets
gh release create v1.0.0 ./dist/*.tar.gz

# Download specific asset
gh release download v1.0.0 --pattern "*.tar.gz"

# Create draft release
gh release create v1.0.0 --draft
```

### Search (`gh search`)

| Command | Description |
|---------|-------------|
| `gh search repos <query>` | Search repositories |
| `gh search issues <query>` | Search issues |
| `gh search prs <query>` | Search pull requests |
| `gh search commits <query>` | Search commits |
| `gh search code <query>` | Search code |

**Search examples:**

```bash
# Find repos by topic
gh search repos --topic=cli --language=go

# Search issues across GitHub
gh search issues "memory leak" --state=open

# Search your PRs
gh search prs --author=@me --state=open

# Search code (requires auth)
gh search code "function authenticate" --repo=owner/repo
```

### Codespaces (`gh codespace` / `gh cs`)

| Command | Description |
|---------|-------------|
| `gh cs create` | Create a codespace |
| `gh cs list` | List codespaces |
| `gh cs code` | Open in VS Code |
| `gh cs ssh` | SSH into codespace |
| `gh cs stop` | Stop a codespace |
| `gh cs delete` | Delete a codespace |
| `gh cs cp` | Copy files to/from codespace |
| `gh cs ports` | Manage port forwarding |
| `gh cs logs` | View codespace logs |
| `gh cs rebuild` | Rebuild codespace |
| `gh cs jupyter` | Open Jupyter |
| `gh cs view` | View codespace details |
| `gh cs edit` | Edit codespace settings |

### Secrets & Variables (`gh secret`, `gh variable`)

**Secrets:**

| Command | Description |
|---------|-------------|
| `gh secret set <name>` | Set a secret |
| `gh secret list` | List secrets |
| `gh secret delete <name>` | Delete a secret |

**Variables:**

| Command | Description |
|---------|-------------|
| `gh variable set <name>` | Set a variable |
| `gh variable get <name>` | Get variable value |
| `gh variable list` | List variables |
| `gh variable delete <name>` | Delete a variable |

**Examples:**

```bash
# Set secret from prompt
gh secret set API_KEY

# Set secret from file
gh secret set CERT < cert.pem

# Set environment secret
gh secret set API_KEY --env production

# Set org-level variable
gh variable set NODE_ENV --org my-org -v "production"
```

### API (`gh api`)

Make authenticated requests to GitHub's REST or GraphQL API.

```bash
gh api <endpoint> [flags]
```

**Key flags:**

| Flag | Description |
|------|-------------|
| `-X, --method` | HTTP method (GET, POST, etc.) |
| `-f, --raw-field` | Add string parameter |
| `-F, --field` | Add typed parameter |
| `-H, --header` | Add HTTP header |
| `-q, --jq` | Filter response with jq |
| `-t, --template` | Format with Go template |
| `--paginate` | Fetch all pages |
| `--slurp` | Combine paginated results |
| `-i, --include` | Show response headers |

**Placeholders:** `{owner}`, `{repo}`, `{branch}` auto-populate from current repo.

**Examples:**

```bash
# Get current user
gh api user

# List repo issues
gh api repos/{owner}/{repo}/issues

# Create issue comment
gh api repos/{owner}/{repo}/issues/123/comments -f body="Comment text"

# GraphQL query
gh api graphql -f query='{ viewer { login } }'

# Paginate and filter
gh api repos/{owner}/{repo}/issues --paginate -q '.[].title'

# POST with JSON body
gh api repos/{owner}/{repo}/labels -f name=bug -f color=ff0000
```

### Other Commands

| Command | Description |
|---------|-------------|
| `gh browse` | Open repo/issue/PR in browser |
| `gh status` | Show GitHub status for repos you care about |
| `gh gist create` | Create a gist |
| `gh gist list` | List your gists |
| `gh gist view` | View a gist |
| `gh gist edit` | Edit a gist |
| `gh gist clone` | Clone a gist |
| `gh gist delete` | Delete a gist |
| `gh label create` | Create a label |
| `gh label list` | List labels |
| `gh label edit` | Edit a label |
| `gh label delete` | Delete a label |
| `gh label clone` | Clone labels from another repo |
| `gh project create` | Create a project |
| `gh project list` | List projects |
| `gh project view` | View project details |
| `gh project edit` | Edit a project |
| `gh project close` | Close a project |
| `gh project delete` | Delete a project |
| `gh cache list` | List workflow caches |
| `gh cache delete` | Delete workflow caches |
| `gh ssh-key add` | Add SSH key |
| `gh ssh-key list` | List SSH keys |
| `gh ssh-key delete` | Delete SSH key |
| `gh gpg-key add` | Add GPG key |
| `gh gpg-key list` | List GPG keys |
| `gh gpg-key delete` | Delete GPG key |
| `gh org list` | List organizations |
| `gh ruleset list` | List rulesets |
| `gh ruleset view` | View ruleset details |
| `gh attestation verify` | Verify artifact attestation |
| `gh alias set` | Create command alias |
| `gh alias list` | List aliases |
| `gh alias delete` | Delete alias |
| `gh config set` | Set configuration |
| `gh config get` | Get configuration |
| `gh config list` | List configuration |
| `gh extension install` | Install extension |
| `gh extension list` | List extensions |
| `gh extension upgrade` | Upgrade extensions |
| `gh extension remove` | Remove extension |
| `gh completion` | Generate shell completions |

## Global Flags

These work with most commands:

| Flag | Description |
|------|-------------|
| `-R, --repo [HOST/]OWNER/REPO` | Target a specific repository |
| `--help` | Show help |
| `--version` | Show version |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | Auth token (alternative to `gh auth login`) |
| `GH_TOKEN` | Same as GITHUB_TOKEN |
| `GH_HOST` | GitHub hostname (for GHES) |
| `GH_REPO` | Default repository |
| `GH_EDITOR` | Editor for composing text |
| `GH_BROWSER` | Browser for opening URLs |
| `GH_PAGER` | Pager for output |
| `NO_COLOR` | Disable color output |

## Tips

1. **Use `--web` or `-w`** to open in browser: `gh pr view --web`
2. **Use `@me`** for current user: `gh issue list --assignee @me`
3. **Use jq filtering** for JSON output: `gh api user -q '.login'`
4. **Tab completion** - run `gh completion` and follow instructions
5. **Aliases** - create shortcuts: `gh alias set pv 'pr view'`
6. **Extensions** - extend functionality: `gh extension install owner/repo`

## Resources

- [Official Manual](https://cli.github.com/manual/)
- [GitHub Docs](https://docs.github.com/en/github-cli)
- [CLI Repository](https://github.com/cli/cli)
