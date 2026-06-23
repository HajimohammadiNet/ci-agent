package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hajimohammadinet/ci-agent/internal/models"
)

const defaultBaseURL = "https://openrouter.ai/api/v1/chat/completions"

type Config struct {
	APIKey      string
	Model       string
	BaseURL     string
	Timeout     time.Duration
	MaxEvidence int
	AppTitle    string
	AppReferer  string
}

type Provider struct {
	apiKey      string
	model       string
	baseURL     string
	maxEvidence int
	appTitle    string
	appReferer  string
	client      *http.Client
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model          string                 `json:"model"`
	Messages       []message              `json:"messages"`
	Temperature    float64                `json:"temperature"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	ResponseFormat map[string]interface{} `json:"response_format,omitempty"`
}

type chatCompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string          `json:"role"`
			Content   json.RawMessage `json:"content"`
			Reasoning json.RawMessage `json:"reasoning,omitempty"`
			Refusal   json.RawMessage `json:"refusal,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string      `json:"message"`
		Type    string      `json:"type"`
		Code    interface{} `json:"code"`
	} `json:"error,omitempty"`
}

func New(cfg Config) (*Provider, error) {
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY is required")
	}

	if cfg.Model == "" {
		cfg.Model = "deepseek/deepseek-v4-pro"
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 75 * time.Second
	}

	if cfg.MaxEvidence <= 0 {
		cfg.MaxEvidence = 15
	}

	if strings.TrimSpace(cfg.AppTitle) == "" {
		cfg.AppTitle = "ci-agent"
	}

	return &Provider{
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		baseURL:     cfg.BaseURL,
		maxEvidence: cfg.MaxEvidence,
		appTitle:    cfg.AppTitle,
		appReferer:  cfg.AppReferer,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

func (p *Provider) Analyze(ctx context.Context, report models.PipelineAnalysis) (*models.AIAnalysis, error) {
	systemPrompt := `You are a senior DevOps and CI/CD incident analyst.

You receive a structured, redacted GitLab CI/CD failure report.
Analyze only the provided evidence.
Do not invent unavailable facts.
Return a concise JSON object with these exact fields:
summary, primary_cause, secondary_causes, recommended_steps, owner_hint, confidence.

confidence must be one of: low, medium, high.
owner_hint should be one of: DevOps, Backend, Frontend, QA, Security, Platform, Unknown.`

	userPrompt := p.buildUserPrompt(report)

	// First try: strict structured output.
	result, err := p.call(ctx, systemPrompt, userPrompt, true)
	if err == nil {
		return result, nil
	}

	// Second try: some OpenRouter/model routes return empty content with response_format.
	// Retry without response_format and force JSON in the prompt.
	fallbackPrompt := userPrompt + `

IMPORTANT:
Return only a valid JSON object.
Do not wrap it in markdown.
Do not include explanations outside JSON.
The JSON schema is:
{
  "summary": "string",
  "primary_cause": "string",
  "secondary_causes": ["string"],
  "recommended_steps": ["string"],
  "owner_hint": "string",
  "confidence": "low|medium|high"
}`

	fallbackResult, fallbackErr := p.call(ctx, systemPrompt, fallbackPrompt, false)
	if fallbackErr == nil {
		return fallbackResult, nil
	}

	return nil, fmt.Errorf("openrouter primary failed: %v; fallback failed: %v", err, fallbackErr)
}

func (p *Provider) call(ctx context.Context, systemPrompt string, userPrompt string, useResponseFormat bool) (*models.AIAnalysis, error) {
	reqPayload := chatCompletionRequest{
		Model:       p.model,
		Temperature: 0.2,
		MaxTokens:   900,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	if useResponseFormat {
		reqPayload.ResponseFormat = aiResponseFormat()
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal openrouter request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create openrouter request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	if p.appReferer != "" {
		httpReq.Header.Set("HTTP-Referer", p.appReferer)
	}

	if p.appTitle != "" {
		httpReq.Header.Set("X-Title", p.appTitle)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call openrouter: %w", err)
	}
	defer resp.Body.Close()

	rawResp, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read openrouter response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openrouter status=%d body=%s", resp.StatusCode, truncate(string(rawResp), 1200))
	}

	var decoded chatCompletionResponse
	if err := json.Unmarshal(rawResp, &decoded); err != nil {
		return nil, fmt.Errorf("decode openrouter response: %w body=%s", err, truncate(string(rawResp), 1200))
	}

	if decoded.Error != nil {
		return nil, fmt.Errorf("openrouter error type=%s code=%v message=%s", decoded.Error.Type, decoded.Error.Code, decoded.Error.Message)
	}

	if len(decoded.Choices) == 0 {
		return nil, fmt.Errorf("openrouter response has no choices body=%s", truncate(string(rawResp), 1200))
	}

	choice := decoded.Choices[0]
	content := extractContentText(choice.Message.Content)

	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf(
			"openrouter response content is empty model=%s finish_reason=%s raw=%s",
			p.model,
			choice.FinishReason,
			truncate(string(rawResp), 1600),
		)
	}

	jsonText := extractJSONObject(content)

	var result models.AIAnalysis
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return nil, fmt.Errorf("decode ai json: %w content=%s", err, truncate(content, 1600))
	}

	normalizeAIResult(&result)

	return &result, nil
}

func (p *Provider) buildUserPrompt(report models.PipelineAnalysis) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Project: %s\n", report.ProjectID)
	fmt.Fprintf(&b, "Pipeline ID: %d\n", report.PipelineID)
	fmt.Fprintf(&b, "Status: %s\n", report.Status)
	fmt.Fprintf(&b, "Ref: %s\n", report.Ref)
	fmt.Fprintf(&b, "SHA: %s\n", report.SHA)
	fmt.Fprintf(&b, "URL: %s\n\n", report.WebURL)

	evidenceCount := 0

	for _, job := range report.Jobs {
		fmt.Fprintf(&b, "Failed job: %s\n", job.JobName)
		fmt.Fprintf(&b, "Stage: %s\n", job.Stage)
		fmt.Fprintf(&b, "Job ID: %d\n", job.JobID)

		for _, finding := range job.Findings {
			fmt.Fprintf(&b, "- Category: %s\n", finding.Category)
			fmt.Fprintf(&b, "  Root cause: %s\n", finding.RootCause)
			fmt.Fprintf(&b, "  Suggested fix: %s\n", finding.SuggestedFix)
			fmt.Fprintf(&b, "  Retry safe: %t\n", finding.RetrySafe)
			fmt.Fprintf(&b, "  Risk level: %s\n", finding.RiskLevel)

			for _, evidence := range finding.Evidence {
				if evidenceCount >= p.maxEvidence {
					continue
				}

				evidence = strings.TrimSpace(evidence)
				if evidence == "" {
					continue
				}

				fmt.Fprintf(&b, "  Evidence: %s\n", evidence)
				evidenceCount++
			}
		}

		fmt.Fprintf(&b, "\n")
	}

	return b.String()
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
				"required": []string{
					"summary",
					"primary_cause",
					"secondary_causes",
					"recommended_steps",
					"owner_hint",
					"confidence",
				},
				"properties": map[string]interface{}{
					"summary": map[string]interface{}{
						"type": "string",
					},
					"primary_cause": map[string]interface{}{
						"type": "string",
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
						"type": "string",
					},
					"confidence": map[string]interface{}{
						"type": "string",
						"enum": []string{
							"low",
							"medium",
							"high",
						},
					},
				},
			},
		},
	}
}

func extractContentText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)

	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}

	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}

	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, part := range parts {
			if strings.TrimSpace(part.Text) != "" {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(part.Text)
			}
		}
		return b.String()
	}

	var generic interface{}
	if err := json.Unmarshal(raw, &generic); err == nil {
		return fmt.Sprintf("%v", generic)
	}

	return string(raw)
}

func extractJSONObject(content string) string {
	content = strings.TrimSpace(content)

	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")

	if start >= 0 && end >= start {
		return content[start : end+1]
	}

	return content
}

func normalizeAIResult(result *models.AIAnalysis) {
	if result.Summary == "" {
		result.Summary = "AI analysis completed, but no summary was returned."
	}

	if result.PrimaryCause == "" {
		result.PrimaryCause = "Unknown"
	}

	if result.OwnerHint == "" {
		result.OwnerHint = "Unknown"
	}

	result.Confidence = strings.ToLower(strings.TrimSpace(result.Confidence))
	switch result.Confidence {
	case "low", "medium", "high":
	default:
		result.Confidence = "medium"
	}

	if result.SecondaryCauses == nil {
		result.SecondaryCauses = []string{}
	}

	if result.RecommendedSteps == nil {
		result.RecommendedSteps = []string{}
	}
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)

	if max <= 0 || len(s) <= max {
		return s
	}

	return s[:max] + "...<truncated>"
}
