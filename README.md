# CI Agent

**CI Agent** is an internal AIOps assistant for analyzing failed GitLab CI/CD pipelines and jobs.

It receives a GitLab pipeline or job URL, fetches failed jobs and traces through the GitLab API, redacts sensitive data, detects failure patterns using a rule-based analyzer, and optionally enriches the result with AI-assisted analysis through OpenRouter.

The project is designed to be used from CLI, HTTP API, Kubernetes, n8n workflows, and Mattermost slash commands.

---

## Features

* Analyze GitLab pipeline URLs
* Analyze GitLab job URLs
* Automatically resolve pipeline ID from job URL
* Fetch failed pipeline jobs
* Fetch failed job traces
* Skip successful jobs to reduce unnecessary processing
* Skip AI calls for successful pipelines or pipelines without findings
* Redact secrets from logs before analysis
* Normalize GitLab CI trace output
* Detect multiple findings per failed job
* Rule-based failure analysis
* AI-assisted root cause analysis through OpenRouter
* JSON output for automation
* Short summary output for chat tools
* Full Markdown output with evidence
* HTTP API with Bearer token authentication
* Docker and Docker Compose support
* Helm chart for Kubernetes deployment
* Kong Ingress compatible
* n8n integration ready
* Mattermost Slash Command compatible

---

## Current Architecture

```text
Developer / DevOps
      |
      | /ciagent <gitlab-job-or-pipeline-url>
      v
Mattermost Slash Command
      |
      v
n8n Webhook
      |
      v
CI Agent API
      |
      +--> GitLab API
      |      - pipeline info
      |      - failed jobs
      |      - failed job traces
      |
      +--> Pre-processing
      |      - trace normalization
      |      - secret redaction
      |      - evidence extraction
      |
      +--> Rule-Based Analyzer
      |      - deterministic failure detection
      |      - category
      |      - root cause
      |      - suggested fix
      |      - risk level
      |
      +--> AI Analyzer
      |      - OpenRouter provider
      |      - primary cause
      |      - secondary causes
      |      - recommended next steps
      |      - owner hint
      |      - confidence
      |
      +--> Reporters
             - JSON
             - Summary
             - Full Markdown
```

---

## How AI Is Used

CI Agent does **not** send every pipeline or every job to AI.

AI is only called when all of these conditions are true:

```text
AI_ENABLED=true
pipeline is not successful
there is at least one failed job
there is at least one rule-based finding
```

Successful pipelines and successful jobs do not trigger AI calls.

This reduces:

* token usage
* cost
* latency
* unnecessary external API calls

The AI receives only structured and redacted context, not raw CI logs.

Recommended flow:

```text
Raw GitLab trace
  ↓
Normalize
  ↓
Redact secrets
  ↓
Rule-based findings
  ↓
Selected evidence only
  ↓
OpenRouter AI analysis
```

---

## Supported Failure Categories

CI Agent currently detects common CI/CD failure patterns such as:

* Helm repository authentication failure
* Kubernetes/Helm rollout timeout
* Go dependency download failure
* Docker build failure
* Docker registry authentication failure
* GitLab runner disk full
* Docker daemon connectivity failure
* Frontend dependency/build failure
* TypeScript build failure
* Go test failure
* Java/Maven/Gradle build failure
* Kubernetes image pull failure
* SSH deploy authentication failure
* Network or DNS failure
* Unknown failure fallback

Each failed job can contain multiple findings.

Example:

```json
{
  "job_name": "deploy-stage-kube",
  "findings": [
    {
      "category": "helm_repository_auth_failure",
      "risk_level": "medium"
    },
    {
      "category": "kubernetes_helm_rollout_timeout",
      "risk_level": "high"
    }
  ]
}
```

---

## AI Analysis Output

When AI is enabled, the output includes an `ai` section:

```json
{
  "ai": {
    "summary": "The pipeline failed because Helm could not authenticate to the chart repository and the deployment later timed out.",
    "primary_cause": "Helm repository authentication failed with 401 Unauthorized.",
    "secondary_causes": [
      "Kubernetes rollout timed out after Helm upgrade"
    ],
    "recommended_steps": [
      "Check Helm repository credentials",
      "Verify deploy token read permissions",
      "Test helm repo update from the runner environment",
      "Inspect Kubernetes pod events if rollout still fails"
    ],
    "owner_hint": "DevOps",
    "confidence": "high"
  }
}
```

---

## Repository Structure

```text
ci-agent/
  cmd/
    ci-agent/
      main.go

  internal/
    agent/
      service.go

    ai/
      analyzer.go

    analyzer/
      analyzer.go
      normalizer.go
      redactor.go

    gitlab/
      client.go
      url_parser.go

    llm/
      openrouter/
        provider.go

    models/
      models.go

    reporter/
      markdown.go
      summary.go

  deployments/
    helm/
      ci-agent/
        Chart.yaml
        values.yaml
        values-rahkar.yaml
        templates/

  Dockerfile
  docker-compose.yml
  .env.example
  README.md
```

---

## Requirements

For local development:

* Go 1.23+
* Git
* Access to GitLab API
* GitLab personal/project/group access token

For AI-assisted analysis:

* OpenRouter account
* OpenRouter API key

For containerized deployment:

* Docker
* Docker Compose

For Kubernetes deployment:

* Kubernetes cluster
* Helm 3
* Kong Ingress Controller
* Container registry access

---

## Environment Variables

### GitLab

| Variable              | Required | Description                                     |
| --------------------- | -------: | ----------------------------------------------- |
| `GITLAB_URL`          |      Yes | GitLab base URL                                 |
| `GITLAB_TOKEN`        |      Yes | GitLab API token used to read pipeline/job data |
| `GITLAB_HTTP_TIMEOUT` |       No | GitLab API timeout, for example `120s`          |

### HTTP API

| Variable             |             Required | Description                                    |
| -------------------- | -------------------: | ---------------------------------------------- |
| `CI_AGENT_API_TOKEN` | Yes for `serve` mode | Bearer token required to call the CI Agent API |

### AI / OpenRouter

| Variable                  |          Required | Description                                       |
| ------------------------- | ----------------: | ------------------------------------------------- |
| `AI_ENABLED`              |                No | Enable or disable AI-assisted analysis            |
| `AI_PROVIDER`             |                No | AI provider name. Currently supports `openrouter` |
| `OPENROUTER_API_KEY`      | Yes if AI enabled | OpenRouter API key                                |
| `OPENROUTER_MODEL`        |                No | OpenRouter model name                             |
| `OPENROUTER_BASE_URL`     |                No | OpenRouter chat completions endpoint              |
| `OPENROUTER_TIMEOUT`      |                No | OpenRouter request timeout                        |
| `OPENROUTER_MAX_EVIDENCE` |                No | Maximum evidence lines sent to AI                 |
| `OPENROUTER_APP_TITLE`    |                No | Application title sent to OpenRouter              |
| `OPENROUTER_HTTP_REFERER` |                No | Optional referer header                           |

Example:

```env
GITLAB_URL=https://gitlab.rahkar.team
GITLAB_TOKEN=replace-me
GITLAB_HTTP_TIMEOUT=120s

CI_AGENT_API_TOKEN=replace-me-with-a-long-random-token

AI_ENABLED=true
AI_PROVIDER=openrouter
OPENROUTER_API_KEY=replace-me
OPENROUTER_MODEL=deepseek/deepseek-v4-pro
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1/chat/completions
OPENROUTER_TIMEOUT=60s
OPENROUTER_MAX_EVIDENCE=40
OPENROUTER_APP_TITLE=ci-agent
OPENROUTER_HTTP_REFERER=
```

Generate a strong API token:

```bash
openssl rand -hex 32
```

---

## Recommended Models

Default production model:

```env
OPENROUTER_MODEL=deepseek/deepseek-v4-pro
```

Fast and cheaper option:

```env
OPENROUTER_MODEL=deepseek/deepseek-v4-flash
```

Premium option for difficult cases:

```env
OPENROUTER_MODEL=anthropic/claude-opus-4.7
```

The model can be changed without code changes.

---

## Local CLI Usage

Export environment variables:

```bash
export GITLAB_URL="https://gitlab.rahkar.team"
export GITLAB_TOKEN="YOUR_GITLAB_TOKEN"
```

Optional AI variables:

```bash
export AI_ENABLED=true
export AI_PROVIDER=openrouter
export OPENROUTER_API_KEY="YOUR_OPENROUTER_API_KEY"
export OPENROUTER_MODEL="deepseek/deepseek-v4-pro"
```

Analyze a GitLab job URL:

```bash
go run ./cmd/ci-agent analyze \
  --url "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717"
```

Analyze a GitLab pipeline URL:

```bash
go run ./cmd/ci-agent analyze \
  --url "https://gitlab.rahkar.team/loan/loanly/-/pipelines/40663"
```

Analyze by project and pipeline ID:

```bash
go run ./cmd/ci-agent analyze \
  --project-id "loan/loanly" \
  --pipeline-id 40663
```

---

## Output Formats

### JSON

Default output format.

```bash
go run ./cmd/ci-agent analyze \
  --url "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717" \
  --format json
```

### Summary

Short human-readable report for chat tools.

```bash
go run ./cmd/ci-agent analyze \
  --url "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717" \
  --format summary
```

Alias:

```bash
--format short
```

### Full Markdown

Complete report with evidence.

```bash
go run ./cmd/ci-agent analyze \
  --url "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717" \
  --format markdown
```

Aliases:

```bash
--format md
--format text
--format full
```

---

## Example Summary Output

```markdown
## ❌ CI/CD Failure Summary

**Project:** `loan/bnpl`
**Pipeline:** [40769](https://gitlab.rahkar.team/loan/bnpl/-/pipelines/40769)
**Status:** `failed`
**Branch:** `test`
**Commit:** `dd8f113d`
**Failed jobs:** `1`
**Findings:** `2`
**Highest risk:** 🔴 `high`

### 🤖 AI Analysis

**Summary:** The deploy-test job failed because Helm could not authenticate to the chart repository.

**Primary cause:** Helm repository authentication failure prevented fetching the chart index.

**Owner hint:** `DevOps`
**AI confidence:** `high`

### Rule-Based Findings

#### `deploy-test` — stage: `deploy`

- 🟠 `helm_repository_auth_failure`
- 🔴 `kubernetes_helm_rollout_timeout`
```

---

## HTTP API

Start the server:

```bash
export GITLAB_URL="https://gitlab.rahkar.team"
export GITLAB_TOKEN="YOUR_GITLAB_TOKEN"
export CI_AGENT_API_TOKEN="YOUR_CI_AGENT_API_TOKEN"

export AI_ENABLED=true
export AI_PROVIDER=openrouter
export OPENROUTER_API_KEY="YOUR_OPENROUTER_API_KEY"
export OPENROUTER_MODEL="deepseek/deepseek-v4-pro"

go run ./cmd/ci-agent serve --listen ":8080"
```

Health check:

```bash
curl http://127.0.0.1:8080/healthz
```

Expected response:

```json
{"status":"ok"}
```

Analyze a job:

```bash
curl -s -X POST http://127.0.0.1:8080/api/v1/analyze \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CI_AGENT_API_TOKEN" \
  -d '{
    "url": "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717",
    "format": "summary"
  }'
```

Analyze a pipeline:

```bash
curl -s -X POST http://127.0.0.1:8080/api/v1/analyze \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CI_AGENT_API_TOKEN" \
  -d '{
    "url": "https://gitlab.rahkar.team/loan/loanly/-/pipelines/40663",
    "format": "full"
  }'
```

---

## API Request

Endpoint:

```http
POST /api/v1/analyze
```

Headers:

```http
Content-Type: application/json
Authorization: Bearer <CI_AGENT_API_TOKEN>
```

Body with URL:

```json
{
  "url": "https://gitlab.rahkar.team/group/project/-/jobs/12345",
  "format": "summary"
}
```

Alternative body with project and pipeline ID:

```json
{
  "project_id": "group/project",
  "pipeline_id": 12345,
  "format": "json"
}
```

Supported formats:

| Format     | Description                          |
| ---------- | ------------------------------------ |
| `json`     | Structured machine-readable output   |
| `summary`  | Short Markdown report for chat tools |
| `short`    | Alias for `summary`                  |
| `markdown` | Full Markdown report                 |
| `md`       | Alias for `markdown`                 |
| `text`     | Alias for `markdown`                 |
| `full`     | Alias for `markdown`                 |

---

## Docker Usage

Create `.env`:

```bash
cp .env.example .env
```

Edit `.env`:

```bash
nano .env
```

Build and start:

```bash
docker compose build
docker compose up -d
```

Check health:

```bash
curl http://127.0.0.1:8080/healthz
```

Analyze:

```bash
export CI_AGENT_API_TOKEN="$(grep '^CI_AGENT_API_TOKEN=' .env | cut -d= -f2-)"

curl -s -X POST http://127.0.0.1:8080/api/v1/analyze \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CI_AGENT_API_TOKEN" \
  -d '{
    "url": "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717",
    "format": "summary"
  }'
```

---

## Kubernetes Deployment

The Helm chart is located at:

```text
deployments/helm/ci-agent
```

Recommended namespace:

```text
aiops
```

Create namespace:

```bash
kubectl create namespace aiops --dry-run=client -o yaml | kubectl apply -f -
```

Create base secret:

```bash
kubectl -n aiops create secret generic ci-agent-secret \
  --from-literal=GITLAB_TOKEN='YOUR_GITLAB_TOKEN' \
  --from-literal=CI_AGENT_API_TOKEN='YOUR_CI_AGENT_API_TOKEN' \
  --dry-run=client -o yaml | kubectl apply -f -
```

Create OpenRouter secret:

```bash
kubectl -n aiops create secret generic ci-agent-openrouter-secret \
  --from-literal=OPENROUTER_API_KEY='YOUR_OPENROUTER_API_KEY' \
  --dry-run=client -o yaml | kubectl apply -f -
```

Deploy with Helm:

```bash
helm upgrade --install ci-agent deployments/helm/ci-agent \
  -n aiops \
  --create-namespace \
  -f deployments/helm/ci-agent/values-rahkar.yaml
```

Check rollout:

```bash
kubectl -n aiops rollout status deploy/ci-agent
```

Check resources:

```bash
kubectl -n aiops get pods
kubectl -n aiops get svc
kubectl -n aiops get ingress
```

Logs:

```bash
kubectl -n aiops logs -l app.kubernetes.io/name=ci-agent -f
```

---

## Helm Values

Example AI configuration:

```yaml
ai:
  enabled: true
  provider: openrouter

openrouter:
  model: deepseek/deepseek-v4-pro
  baseUrl: https://openrouter.ai/api/v1/chat/completions
  timeout: 60s
  maxEvidence: 40
  appTitle: ci-agent
  httpReferer: ""
  existingSecret: ci-agent-openrouter-secret
  secretKey: OPENROUTER_API_KEY
```

Example image configuration:

```yaml
image:
  repository: registry.rahkar.team/agentops/ci-agent
  tag: "0.3.0"
```

---

## Kong Ingress

The Helm chart supports Kong Ingress through:

```yaml
ingress:
  enabled: true
  className: kong
  annotations:
    konghq.com/strip-path: "false"
    konghq.com/preserve-host: "true"
  hosts:
    - host: ci-agent.rahkar.team
      paths:
        - path: /
          pathType: Prefix
```

External health check:

```bash
curl https://ci-agent.rahkar.team/healthz
```

External API call:

```bash
curl -s -X POST https://ci-agent.rahkar.team/api/v1/analyze \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CI_AGENT_API_TOKEN" \
  -d '{
    "url": "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717",
    "format": "summary"
  }'
```

---

## Mattermost Slash Command

CI Agent can be used from Mattermost through n8n.

Summary report:

```text
/ciagent https://gitlab.rahkar.team/loan/loanly/-/jobs/87717
```

Full report:

```text
/ciagent full https://gitlab.rahkar.team/loan/loanly/-/jobs/87717
```

Recommended Mattermost Slash Command settings:

| Setting                  | Value                                                 |
| ------------------------ | ----------------------------------------------------- |
| Trigger Word             | `ciagent`                                             |
| Request URL              | `https://n8n.rahkar.team/webhook/mattermost/ci-agent` |
| Request Method           | `POST`                                                |
| Autocomplete             | `true`                                                |
| Autocomplete Hint        | `<gitlab job/pipeline url>`                           |
| Autocomplete Description | `Analyze failed GitLab CI/CD job or pipeline`         |

If Mattermost blocks internal integrations, allow the n8n host:

```json
"AllowedUntrustedInternalConnections": "n8n.rahkar.team"
```

or with environment variable:

```env
MM_SERVICESETTINGS_ALLOWEDUNTRUSTEDINTERNALCONNECTIONS=n8n.rahkar.team
```

---

## n8n Workflow

Recommended n8n workflow:

```text
Mattermost Slash Command
        ↓
n8n Webhook
        ↓
Validate Mattermost token
        ↓
Extract GitLab URL and format
        ↓
Call CI Agent API
        ↓
Respond to Mattermost
```

Example request to CI Agent from n8n:

```json
{
  "url": "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717",
  "format": "summary"
}
```

For full report:

```json
{
  "url": "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717",
  "format": "full"
}
```

---

## Security Notes

CI Agent can read GitLab pipeline and job traces using `GITLAB_TOKEN`.

Recommended security controls:

* Use a dedicated GitLab bot token
* Use least-privilege GitLab permissions
* Do not commit `.env` files
* Do not store tokens in Helm values
* Use Kubernetes Secrets
* Keep `CI_AGENT_API_TOKEN` strong and private
* Keep `OPENROUTER_API_KEY` strong and private
* Restrict external access to the API through Kong, firewall, or internal DNS
* Keep `/healthz` unauthenticated but do not expose sensitive data from it
* Keep Mattermost response mode `ephemeral` by default
* Avoid allowing broad internal CIDR ranges in Mattermost `AllowedUntrustedInternalConnections`
* Never send raw CI traces directly to AI providers
* Always redact secrets before AI analysis

---

## Secret Redaction

The analyzer redacts common secret patterns before evidence is returned or sent to AI, including:

* passwords
* tokens
* secrets
* bearer tokens
* private tokens
* CI job tokens
* registry passwords

This is a safety layer and should not replace proper secret hygiene in CI/CD logs.

---

## Token and Cost Optimization

CI Agent is designed to avoid unnecessary AI token usage.

The following data is not sent to AI:

* successful pipelines
* successful jobs
* jobs without findings
* raw full traces
* unrelated pipeline metadata
* empty analysis results

AI receives only:

* pipeline metadata
* failed job names
* stages
* rule-based findings
* selected redacted evidence

---

## Development

Format:

```bash
go fmt ./...
```

Build:

```bash
go build ./cmd/ci-agent
```

Run CLI without AI:

```bash
export AI_ENABLED=false

go run ./cmd/ci-agent analyze \
  --url "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717" \
  --format summary
```

Run CLI with AI:

```bash
export AI_ENABLED=true
export AI_PROVIDER=openrouter
export OPENROUTER_API_KEY="YOUR_OPENROUTER_API_KEY"
export OPENROUTER_MODEL="deepseek/deepseek-v4-pro"

go run ./cmd/ci-agent analyze \
  --url "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717" \
  --format summary
```

Run server:

```bash
go run ./cmd/ci-agent serve --listen ":8080"
```

---

## Git Remotes

This repository can be pushed to both GitHub and the internal GitLab repository.

Example remote setup:

```bash
git remote set-url origin git@github.com:HajimohammadiNet/ci-agent.git

git remote set-url --push origin git@github.com:HajimohammadiNet/ci-agent.git
git remote set-url --add --push origin git@gitlab-rahkar:agentops/ci-agent.git
```

Push to both:

```bash
git push origin main
```

---

## Release

Suggested version for OpenRouter AI support:

```bash
git tag -a v0.3.0 -m "Add AI-assisted CI analysis with OpenRouter"
git push origin v0.3.0
```

---

## Roadmap

Planned improvements:

* Mattermost async response using `response_url`
* GitLab webhook mode
* Project-specific profiles
* Configurable rule packs
* More CI/CD failure rules
* PostgreSQL storage for historical failures
* Similar failure detection
* Known fixes database
* LLM prompt versioning
* LLM provider abstraction for other providers
* GitLab MR comments
* GitLab issue creation
* Safe retry action with approval
* MCP server for exposing CI Agent tools
* Prometheus metrics
* OpenTelemetry tracing

---

## Project Status

CI Agent is currently in internal MVP stage.

Current recommended usage:

```text
Developer sends a GitLab failed job/pipeline URL in Mattermost using /ciagent.
n8n forwards the request to CI Agent.
CI Agent analyzes GitLab CI/CD logs using rule-based detection.
If enabled and useful, CI Agent enriches the result with OpenRouter AI analysis.
CI Agent returns a summary or full report back to Mattermost.
```

---

## License

Internal project. Add license information according to your organization policy.
