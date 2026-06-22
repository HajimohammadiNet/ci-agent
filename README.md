# CI Agent

**CI Agent** is an internal AIOps assistant for analyzing failed GitLab CI/CD pipelines and jobs.

It receives a GitLab pipeline or job URL, fetches failed jobs and traces through the GitLab API, redacts sensitive data, detects failure patterns, and returns a structured analysis in JSON, Markdown, or short summary format.

The project is designed to be used from CLI, HTTP API, Kubernetes, n8n workflows, and Mattermost slash commands.

---

## Features

* Analyze GitLab pipeline URLs
* Analyze GitLab job URLs
* Automatically resolve pipeline ID from job URL
* Fetch failed pipeline jobs
* Fetch GitLab job traces
* Redact secrets from logs
* Normalize GitLab CI trace output
* Detect multiple findings per failed job
* Generate JSON output for automation
* Generate Markdown output for human-readable reports
* Generate short summary output for chat tools
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
      |      - job traces
      |
      +--> Analyzer
      |      - secret redaction
      |      - trace normalization
      |      - rule-based failure detection
      |
      +--> Reporters
             - JSON
             - Markdown
             - Summary
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

## Repository Structure

```text
ci-agent/
  cmd/
    ci-agent/
      main.go

  internal/
    agent/
      service.go

    analyzer/
      analyzer.go
      normalizer.go
      redactor.go

    gitlab/
      client.go
      url_parser.go

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

| Variable              |            Required | Description                                     |
| --------------------- | ------------------: | ----------------------------------------------- |
| `GITLAB_URL`          |                 Yes | GitLab base URL                                 |
| `GITLAB_TOKEN`        |                 Yes | GitLab API token used to read pipeline/job data |
| `GITLAB_HTTP_TIMEOUT` |                  No | GitLab API timeout, for example `120s`          |
| `CI_AGENT_API_TOKEN`  | Yes for HTTP server | Bearer token required to call the CI Agent API  |

Example:

```env
GITLAB_URL=https://gitlab.rahkar.team
GITLAB_TOKEN=replace-me
GITLAB_HTTP_TIMEOUT=120s
CI_AGENT_API_TOKEN=replace-me-with-a-long-random-token
```

Generate a strong API token:

```bash
openssl rand -hex 32
```

---

## Local CLI Usage

Export environment variables:

```bash
export GITLAB_URL="https://gitlab.rahkar.team"
export GITLAB_TOKEN="YOUR_GITLAB_TOKEN"
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

### Full Markdown

Complete report with evidence.

```bash
go run ./cmd/ci-agent analyze \
  --url "https://gitlab.rahkar.team/loan/loanly/-/jobs/87717" \
  --format markdown
```

Alias:

```bash
--format full
```

---

## HTTP API

Start the server:

```bash
export GITLAB_URL="https://gitlab.rahkar.team"
export GITLAB_TOKEN="YOUR_GITLAB_TOKEN"
export CI_AGENT_API_TOKEN="YOUR_CI_AGENT_API_TOKEN"

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
    "format": "markdown"
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

Body:

```json
{
  "url": "https://gitlab.rahkar.team/group/project/-/jobs/12345",
  "format": "summary"
}
```

Alternative body:

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

Create Kubernetes secret:

```bash
kubectl -n aiops create secret generic ci-agent-secret \
  --from-literal=GITLAB_TOKEN='YOUR_GITLAB_TOKEN' \
  --from-literal=CI_AGENT_API_TOKEN='YOUR_CI_AGENT_API_TOKEN' \
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

Example external health check:

```bash
curl https://ci-agent.rahkar.team/healthz
```

Example external API call:

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

Example user command:

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

For Mattermost internal integrations, allow the n8n host if Mattermost blocks internal connections:

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
Extract GitLab URL
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
* Restrict external access to the API through Kong, firewall, or internal DNS
* Keep `/healthz` unauthenticated but do not expose sensitive data from it
* Keep Mattermost response mode `ephemeral` by default
* Avoid allowing broad internal CIDR ranges in Mattermost `AllowedUntrustedInternalConnections`

---

## Secret Redaction

The analyzer redacts common secret patterns before storing or returning evidence, including:

* passwords
* tokens
* secrets
* bearer tokens
* private tokens
* CI job tokens
* registry passwords

This is a safety layer and should not replace proper secret hygiene in CI/CD logs.

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

Run CLI:

```bash
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

## Roadmap

Planned improvements:

* GitLab webhook mode
* Mattermost async response using `response_url`
* Project profiles
* More rule packs
* Configurable rules
* PostgreSQL storage for historical failures
* Similar failure detection
* Known fixes database
* LLM-assisted root-cause analysis
* GitLab MR comments
* GitLab issue creation
* Safe retry action with approval
* MCP server for exposing CI Agent tools
* Prometheus metrics
* OpenTelemetry tracing

---

## Project Status

CI Agent is currently in early internal MVP stage.

Current recommended usage:

```text
Developer sends a GitLab failed job/pipeline URL in Mattermost using /ciagent.
n8n forwards the request to CI Agent.
CI Agent analyzes GitLab CI/CD logs and returns a summary or full report.
```

---

## License

Internal project. Add license information according to your organization policy.
