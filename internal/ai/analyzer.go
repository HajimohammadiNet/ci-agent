package ai

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/hajimohammadinet/ci-agent/internal/models"
)

type Provider interface {
	Analyze(ctx context.Context, report models.PipelineAnalysis) (*models.AIAnalysis, error)
}

type Analyzer struct {
	enabled  bool
	standard Provider
	premium  Provider
}

func NewAnalyzer(enabled bool, standard Provider, premium Provider) *Analyzer {
	return &Analyzer{
		enabled:  enabled,
		standard: standard,
		premium:  premium,
	}
}

func (a *Analyzer) Analyze(ctx context.Context, report models.PipelineAnalysis, mode string) *models.AIAnalysis {
	if !a.enabled {
		return nil
	}

	mode = normalizeMode(mode)
	if mode == "off" {
		return nil
	}

	provider := a.standard
	selected := "standard"

	if mode == "premium" {
		if a.premium != nil {
			provider = a.premium
			selected = "premium"
		} else {
			log.Printf("ai premium requested but premium provider is not configured; falling back to standard")
		}
	}

	if provider == nil {
		log.Printf("ai analysis skipped: no provider configured mode=%s selected=%s", mode, selected)
		return nil
	}

	start := time.Now()

	result, err := provider.Analyze(ctx, report)
	if err != nil {
		log.Printf("ai analysis failed mode=%s selected=%s duration=%s error=%v", mode, selected, time.Since(start), err)

		return &models.AIAnalysis{
			Summary:      "AI analysis unavailable. Rule-based analysis was returned.",
			Confidence:   "unknown",
			OwnerHint:    "unknown",
			PrimaryCause: "AI provider failed or timed out.",
		}
	}

	log.Printf(
		"ai analysis completed mode=%s selected=%s duration=%s confidence=%q owner_hint=%q",
		mode,
		selected,
		time.Since(start),
		result.Confidence,
		result.OwnerHint,
	)

	return result
}

func normalizeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))

	switch mode {
	case "", "auto":
		return "auto"
	case "off", "noai", "none", "false", "disabled":
		return "off"
	case "standard", "default":
		return "standard"
	case "premium", "pro":
		return "premium"
	default:
		return "auto"
	}
}
