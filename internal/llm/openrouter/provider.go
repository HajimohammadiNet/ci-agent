package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hajimohammadinet/ci-agent/internal/models"
)

const defaultBaseURL = "https://openrouter.ai/api/v1/chat/completions"

type Provider struct {
	apiKey      string
	model       string
	baseURL     string
	httpClient  *http.Client
	maxEvidence int
	appTitle    string
	appReferer  string
}

type Config struct {
	APIKey      string
	Model       string
	BaseURL     string
	Timeout     time.Duration
	MaxEvidence int
	AppTitle    string
	AppReferer  string
}

func New(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("openrouter api key is required")
	}

	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = "deepseek/deepseek-v4-pro"
	}

	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultBaseURL
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}

	if cfg.MaxEvidence <= 0 {
		cfg.MaxEvidence = 40
	}

	if cfg.AppTitle == "" {
		cfg.AppTitle = "ci-agent"
	}

	return &Provider{
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		baseURL:     cfg.BaseURL,
		httpClient:  &http.Client{Timeout: cfg.Timeout},
		maxEvidence: cfg.MaxEvidence,
		appTitle:    cfg.AppTitle,
		appReferer:  cfg.AppReferer,
	}, nil
}

func (p *Provider) Analyze(ctx context.Context, report models.PipelineAnalysis) (*models.AIAnalysis, error) {
	payload := p.buildRequest(report)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal openrouter request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create openrouter request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenRouter-Title", p.appTitle)

	if p.appReferer != "" {
		req.Header.Set("HTTP-Referer", p.appReferer)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call openrouter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody bytes.Buffer
		_, _ = errBody.ReadFrom(resp.Body)
		return nil, fmt.Errorf("openrouter returned status %d: %s", resp.StatusCode, truncate(errBody.String(), 1000))
	}

	var out chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode openrouter response: %w", err)
	}

	if len(out.Choices) == 0 {
		return nil, errors.New("openrouter response has no choices")
	}

	content := strings.TrimSpace(out.Choices[0].Message.Content)
	if content == "" {
		return nil, errors.New("openrouter response content is empty")
	}

	var ai models.AIAnalysis
	if err := json.Unmarshal([]byte(content), &ai); err != nil {
		return nil, fmt.Errorf("parse ai json: %w; content=%s", err, truncate(content, 1000))
	}

	normalizeAI(&ai)

	return &ai, nil
}

type chatCompletionRequest struct {
	Model          string                 `json:"model"`
	Messages       []message              `json:"messages"`
	Temperature    float64                `json:"temperature"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	ResponseFormat map[string]interface{} `json:"response_format,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage map[string]interface{} `json:"usage,omitempty"`
}

func (p *Provider) buildRequest(report models.PipelineAnalysis) chatCompletionRequest {
	systemPrompt := `You are an expert DevOps, SRE, CI/CD and Kubernetes incident analysis assistant.

You analyze failed GitLab CI/CD pipelines.

Rules:
- Use only the provided pipeline data, findings, and evidence.
- Do not invent logs, commands, services, or facts.
- Do not expose or reconstruct secrets.
- Be practical and operational.
- Prefer concise but useful analysis.
- Return only valid JSON matching the schema.
- No markdown.
- No extra text outside JSON.`

	userPrompt := fmt.Sprintf(`Analyze this failed GitLab CI/CD pipeline.

Pipeline context:
%s

Return:
- summary: concise human-readable explanation
- primary_cause: the most likely main root cause
- secondary_causes: other important causes if any
- recommended_steps: ordered troubleshooting/fix steps
- owner_hint: likely responsible team, for example "DevOps", "Backend", "Frontend", "DevOps + Backend"
- confidence: one of "low", "medium", "high"

Important:
- Findings are rule-based and may include multiple related failures.
- Evidence is already redacted.
- If evidence is not enough, say confidence is low.
`, buildPipelineContext(report, p.maxEvidence))

	return chatCompletionRequest{
		Model:       p.model,
		Temperature: 0.2,
		MaxTokens:   800,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseFormat: aiResponseFormat(),
	}
}

func aiResponseFormat() map[string]interface{} {
	return map[string]interface{}{
		"type": "json_schema",
		"json_schema": map[string]interface{}{
			"name":   "ci_agent_ai_analysis",
			"strict": true,
			"schema": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"summary": map[string]interface{}{
						"type":        "string",
						"description": "Short explanation of why the pipeline failed.",
					},
					"primary_cause": map[string]interface{}{
						"type":        "string",
						"description": "Most likely main root cause.",
					},
					"secondary_causes": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"recommended_steps": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"owner_hint": map[string]interface{}{
						"type":        "string",
						"description": "Likely owner team.",
					},
					"confidence": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high"},
					},
				},
				"required": []string{
					"summary",
					"primary_cause",
					"secondary_causes",
					"recommended_steps",
					"owner_hint",
					"confidence",
				},
			},
		},
	}
}

func buildPipelineContext(report models.PipelineAnalysis, maxEvidence int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Project: %s\n", report.ProjectID)
	fmt.Fprintf(&b, "Pipeline ID: %d\n", report.PipelineID)
	fmt.Fprintf(&b, "Status: %s\n", report.Status)
	fmt.Fprintf(&b, "Ref: %s\n", report.Ref)
	fmt.Fprintf(&b, "SHA: %s\n", report.SHA)
	fmt.Fprintf(&b, "Web URL: %s\n\n", report.WebURL)

	evidenceCount := 0

	for _, job := range report.Jobs {
		fmt.Fprintf(&b, "Failed job: %s\n", job.JobName)
		fmt.Fprintf(&b, "Stage: %s\n", job.Stage)
		fmt.Fprintf(&b, "Job ID: %d\n", job.JobID)

		for _, finding := range job.Findings {
			fmt.Fprintf(&b, "- Finding category: %s\n", finding.Category)
			fmt.Fprintf(&b, "  Root cause: %s\n", finding.RootCause)
			fmt.Fprintf(&b, "  Risk: %s\n", finding.RiskLevel)
			fmt.Fprintf(&b, "  Retry safe: %t\n", finding.RetrySafe)
			fmt.Fprintf(&b, "  Suggested fix: %s\n", finding.SuggestedFix)

			for _, ev := range finding.Evidence {
				if evidenceCount >= maxEvidence {
					continue
				}

				ev = strings.TrimSpace(ev)
				if ev == "" {
					continue
				}

				fmt.Fprintf(&b, "  Evidence: %s\n", truncate(ev, 600))
				evidenceCount++
			}
		}

		fmt.Fprintf(&b, "\n")
	}

	return b.String()
}

func normalizeAI(ai *models.AIAnalysis) {
	ai.Summary = strings.TrimSpace(ai.Summary)
	ai.PrimaryCause = strings.TrimSpace(ai.PrimaryCause)
	ai.OwnerHint = strings.TrimSpace(ai.OwnerHint)
	ai.Confidence = strings.ToLower(strings.TrimSpace(ai.Confidence))

	if ai.Confidence != "low" && ai.Confidence != "medium" && ai.Confidence != "high" {
		ai.Confidence = "low"
	}

	if ai.OwnerHint == "" {
		ai.OwnerHint = "unknown"
	}

	if ai.Summary == "" {
		ai.Summary = "AI analysis did not provide a summary."
	}
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}

	return s[:max] + "..."
}
