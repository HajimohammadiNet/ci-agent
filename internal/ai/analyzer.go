package ai

import (
	"context"
	"log"
	"time"

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

	start := time.Now()

	result, err := a.provider.Analyze(ctx, report)
	if err != nil {
		log.Printf("ai analysis failed duration=%s error=%v", time.Since(start), err)

		return &models.AIAnalysis{
			Summary:      "AI analysis unavailable. Rule-based analysis was returned.",
			Confidence:   "unknown",
			OwnerHint:    "unknown",
			PrimaryCause: "AI provider failed or timed out.",
		}
	}

	log.Printf(
		"ai analysis completed duration=%s confidence=%q owner_hint=%q",
		time.Since(start),
		result.Confidence,
		result.OwnerHint,
	)

	return result
}
