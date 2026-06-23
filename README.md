## AI-assisted analysis

CI Agent supports optional AI-assisted analysis on top of the built-in rule-based analyzer.

The rule-based analyzer always runs first. AI is only used when it can add value, such as summarizing the failure, identifying the most likely owner, and prioritizing the next steps.

### AI modes

The API and Mattermost command support multiple AI modes:

| Mode       | Description                                                                                |
| ---------- | ------------------------------------------------------------------------------------------ |
| `auto`     | Default mode. Runs AI only when the pipeline/job failed and rule-based findings exist.     |
| `off`      | Disables AI and returns rule-based analysis only. Useful for fast checks and cost control. |
| `standard` | Forces the standard AI model.                                                              |
| `premium`  | Uses the premium AI model when configured. Useful for complex incidents.                   |

API example:

```json
{
  "url": "https://gitlab.example.com/group/project/-/jobs/12345",
  "format": "summary",
  "ai_mode": "standard"
}
```

CLI example:

```bash
ci-agent analyze \
  --url "https://gitlab.example.com/group/project/-/jobs/12345" \
  --format summary \
  --ai-mode standard
```

Mattermost examples:

```text
/ciagent https://gitlab.example.com/group/project/-/jobs/12345
/ciagent noai https://gitlab.example.com/group/project/-/jobs/12345
/ciagent standard https://gitlab.example.com/group/project/-/jobs/12345
/ciagent premium https://gitlab.example.com/group/project/-/jobs/12345
/ciagent public premium https://gitlab.example.com/group/project/-/jobs/12345
```

### OpenRouter configuration

CI Agent uses OpenRouter as the AI provider.

Required:

```env
AI_ENABLED=true
AI_PROVIDER=openrouter
OPENROUTER_API_KEY=<openrouter-api-key>
```

Standard model:

```env
OPENROUTER_MODEL=deepseek/deepseek-v4-pro
OPENROUTER_TIMEOUT=75s
OPENROUTER_MAX_EVIDENCE=15
```

Standard fallback model:

```env
OPENROUTER_FALLBACK_MODEL=qwen/qwen3.7-plus
OPENROUTER_FALLBACK_TIMEOUT=45s
OPENROUTER_FALLBACK_MAX_EVIDENCE=10
```

Premium model:

```env
OPENROUTER_PREMIUM_MODEL=deepseek/deepseek-v4-pro
OPENROUTER_PREMIUM_TIMEOUT=90s
OPENROUTER_PREMIUM_MAX_EVIDENCE=25
```

Premium fallback model:

```env
OPENROUTER_PREMIUM_FALLBACK_MODEL=qwen/qwen3.7-plus
OPENROUTER_PREMIUM_FALLBACK_TIMEOUT=60s
OPENROUTER_PREMIUM_FALLBACK_MAX_EVIDENCE=15
```

Optional metadata headers:

```env
OPENROUTER_APP_TITLE=ci-agent
OPENROUTER_HTTP_REFERER=
```

### AI fallback behavior

CI Agent supports model-level fallback.

For `standard` mode:

```text
OPENROUTER_MODEL
  ↓ if failed, timed out, returned empty content, or invalid JSON
OPENROUTER_FALLBACK_MODEL
  ↓ if failed
Rule-based report + AI unavailable message
```

For `premium` mode:

```text
OPENROUTER_PREMIUM_MODEL
  ↓ if failed
OPENROUTER_PREMIUM_FALLBACK_MODEL
  ↓ if failed
Rule-based report + AI unavailable message
```

This makes the system more reliable when a provider route is slow, unavailable, or returns an unexpected response.

### Cost and latency controls

CI Agent reduces AI cost and latency by:

* Running the rule-based analyzer first.
* Skipping AI for successful pipelines.
* Skipping AI when no rule-based findings exist.
* Supporting `ai_mode=off`.
* Sending only selected, redacted evidence to the AI provider.
* Limiting evidence with `OPENROUTER_MAX_EVIDENCE`.
* Supporting cheaper/faster fallback models.
* Returning a useful rule-based report even when AI fails.

### Mattermost async flow

The recommended Mattermost integration is asynchronous:

```text
Mattermost slash command
  ↓
n8n webhook
  ↓
Immediate ephemeral response:
"CI/CD analysis started. I will send the result shortly..."
  ↓
n8n calls ci-agent API
  ↓
n8n sends the final result to Mattermost response_url
```

This avoids Mattermost timeout issues when AI analysis takes longer than a few seconds.

### Security notes

* Never expose `GITLAB_TOKEN`, `CI_AGENT_API_TOKEN`, or `OPENROUTER_API_KEY` in logs.
* Store all tokens in Kubernetes Secrets or a secret manager.
* Use least-privilege GitLab tokens.
* Keep AI evidence redacted.
* Prefer `ai_mode=off` for sensitive jobs if needed.