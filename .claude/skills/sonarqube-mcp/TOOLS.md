# SonarQube MCP Tools Reference

Complete documentation for all available MCP tools.

## Analysis Tools

### analyze_code_snippet
Analyze code before pushing - no CI trigger required.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `code` | Yes | Code string to analyze |
| `language` | Yes | Language: typescript, go, python, java, etc. |

### analyze_file_list
Analyze local files via SonarQube for IDE.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `files` | Yes | List of file paths |

### toggle_automatic_analysis
Enable/disable auto-analysis in SonarQube for IDE.

---

## Issue Management

### search_sonar_issues_in_projects
Find issues with filtering.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `projects` | No | Array of project keys |
| `pullRequestId` | No | PR number as string |
| `severities` | No | Array: BLOCKER, CRITICAL, MAJOR, MINOR, INFO |
| `p` | No | Page number (default: 1) |
| `ps` | No | Page size (1-500, default: 100) |

### change_sonar_issue_status
Modify issue status.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `key` | Yes | Issue key from search results |
| `status` | Yes | Array: ["accept"], ["falsepositive"], or ["reopen"] |

### list_issue_authors
List authors who introduced issues.

### get_issue_changelog
Get history of changes to an issue.

### list_issue_tags
List available issue tags.

---

## Quality Gates

### get_project_quality_gate_status
Check if PR/branch passes.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `projectKey` | No* | Project key (e.g., "org_project") |
| `projectId` | No* | Project ID |
| `analysisId` | No* | Analysis ID |
| `pullRequest` | No | PR number as string |
| `branch` | No | Branch name |

*One of projectKey, projectId, or analysisId required.

**Response:**
```json
{
  "status": "OK" | "ERROR",
  "conditions": [
    {
      "metricKey": "new_duplicated_lines_density",
      "status": "OK" | "ERROR",
      "errorThreshold": "3",
      "actualValue": "0.5"
    }
  ]
}
```

### list_quality_gates
List all configured quality gates.

---

## Project Discovery

### search_my_sonarqube_projects
List all accessible projects.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `page` | No | Page number as string |

### list_enterprises
List organizations (SonarCloud only).

| Parameter | Required | Description |
|-----------|----------|-------------|
| `enterpriseKey` | No | Filter by enterprise |

### list_portfolios
List enterprise portfolios.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `enterpriseId` | No* | Enterprise UUID |
| `favorite` | No* | Only favorites (boolean) |
| `draft` | No | Only drafts (boolean) |
| `pageIndex` | No | Page number |
| `pageSize` | No | Results per page |
| `q` | No | Search query |

*enterpriseId required unless favorite=true

---

## Metrics & Measures

### get_component_measures
Get project/file metrics.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `projectKey` | No | Project key |
| `metricKeys` | No | Array of metric keys |
| `branch` | No | Branch name |
| `pullRequest` | No | PR number |

**Common metric keys:**
- `ncloc` - Lines of code
- `coverage` - Test coverage %
- `duplicated_lines_density` - Duplication %
- `complexity` - Cyclomatic complexity
- `violations` - Total issues
- `security_hotspots` - Security concerns
- `reliability_rating` - Bug rating (1=A, 5=E)
- `security_rating` - Vulnerability rating
- `sqale_rating` - Maintainability rating

### search_metrics
List available metrics with pagination.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `p` | No | Page number |
| `ps` | No | Page size (1-500) |

---

## Rules

### show_rule
Get detailed rule information.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `key` | Yes | Rule key (e.g., "typescript:S1082") |

**Response includes:**
- Rule description
- Why it matters
- How to fix
- Code examples

### list_rule_repositories
List rule repositories by language.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `language` | No | Language key (e.g., "java") |
| `q` | No | Search query |

### list_languages
List supported programming languages.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `q` | No | Filter pattern |

---

## Source Code

### get_raw_source
Get file source code (requires permission).

| Parameter | Required | Description |
|-----------|----------|-------------|
| `key` | Yes | File key (e.g., "project:src/file.ts") |
| `branch` | No | Branch name |
| `pullRequest` | No | PR number |

### get_scm_info
Get SCM/blame information.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `key` | Yes | File key |
| `from` | No | Start line |
| `to` | No | End line |
| `commits_by_line` | No | Group by commit (boolean) |

---

## Webhooks

### create_webhook
Create organization/project webhook.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `name` | Yes | Webhook name (max 100 chars) |
| `url` | Yes | Endpoint URL (max 512 chars) |
| `projectKey` | No | Project key (max 400 chars) |
| `secret` | No | HMAC secret (16-200 chars) |

### list_webhooks
List existing webhooks.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `projectKey` | No | Filter by project |

---

## System (Server Only)

### get_system_health
Returns: GREEN, YELLOW, or RED

### get_system_info
Detailed server configuration.

### get_system_logs
System logs in plain text.

### ping_system
Verify server availability.

### get_system_status
Server state and version.

---

## Setup Reference

### SonarCloud
```bash
claude mcp add sonarqube \
  --env SONARQUBE_TOKEN=your_token \
  --env SONARQUBE_ORG=your_org \
  -- docker run -i --rm \
  -e SONARQUBE_TOKEN -e SONARQUBE_ORG mcp/sonarqube
```

### SonarQube Server
```bash
claude mcp add sonarqube \
  --env SONARQUBE_TOKEN=your_token \
  --env SONARQUBE_URL=https://your-server.com \
  -- docker run -i --rm \
  -e SONARQUBE_TOKEN -e SONARQUBE_URL mcp/sonarqube
```

### Environment Variables
| Variable | Required | Description |
|----------|----------|-------------|
| `SONARQUBE_TOKEN` | Yes | User token |
| `SONARQUBE_ORG` | Cloud | Organization key |
| `SONARQUBE_URL` | Server | Server URL |
| `SONARQUBE_READ_ONLY` | No | Disable writes |
| `SONARQUBE_TOOLSETS` | No | Limit tools |

### Get Token
- SonarCloud: https://sonarcloud.io/account/security
- Server: Profile → Security → Generate Token
