package ai

import (
	"context"

	"github.com/hajimohammadinet/ci-agent/internal/models"
)

type Provider interface {
	Analyze(ctx context.Context, report models.PipelineAnalysis) (*models.AIAnalysis, error)
}

type Analyzer struct {
	enabled  bool
	provider Provider
}

func NewAnalyzer(enabled bool, provider Provider) *Analyzer {
	return &Analyzer{
		enabled:  enabled,
		provider: provider,
	}
}

func (a *Analyzer) Analyze(ctx context.Context, report models.PipelineAnalysis) *models.AIAnalysis {
	if !a.enabled || a.provider == nil {
		return nil
	}

	result, err := a.provider.Analyze(ctx, report)
	if err != nil {
		return &models.AIAnalysis{
			Summary:      "AI analysis unavailable. Rule-based analysis was returned.",
			Confidence:   "unknown",
			OwnerHint:    "unknown",
			PrimaryCause: "AI provider failed or timed out.",
		}
	}

	return result
}
