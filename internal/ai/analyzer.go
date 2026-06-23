package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hajimohammadinet/ci-agent/internal/models"
)

type Provider interface {
	Analyze(ctx context.Context, report models.PipelineAnalysis) (*models.AIAnalysis, error)
}

type Analyzer struct {
	enabled bool

	standard         Provider
	standardFallback Provider

	premium         Provider
	premiumFallback Provider
}

func NewAnalyzer(
	enabled bool,
	standard Provider,
	standardFallback Provider,
	premium Provider,
	premiumFallback Provider,
) *Analyzer {
	return &Analyzer{
		enabled:          enabled,
		standard:         standard,
		standardFallback: standardFallback,
		premium:          premium,
		premiumFallback:  premiumFallback,
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

	primary := a.standard
	fallback := a.standardFallback
	selected := "standard"

	if mode == "premium" {
		selected = "premium"

		if a.premium != nil {
			primary = a.premium
			fallback = a.premiumFallback
		} else {
			log.Printf("ai premium requested but premium provider is not configured; falling back to standard")
			selected = "standard"
		}
	}

	if primary == nil {
		log.Printf("ai analysis skipped: no provider configured mode=%s selected=%s", mode, selected)
		return nil
	}

	result, err := analyzeWithProvider(ctx, primary, report, mode, selected, false)
	if err == nil {
		return result
	}

	if fallback != nil {
		log.Printf("ai fallback started mode=%s selected=%s reason=%v", mode, selected, err)

		fallbackResult, fallbackErr := analyzeWithProvider(ctx, fallback, report, mode, selected, true)
		if fallbackErr == nil {
			return fallbackResult
		}

		log.Printf(
			"ai fallback failed mode=%s selected=%s primary_error=%v fallback_error=%v",
			mode,
			selected,
			err,
			fallbackErr,
		)

		return unavailableAI(fmt.Errorf("primary failed: %w; fallback failed: %w", err, fallbackErr))
	}

	return unavailableAI(err)
}

func analyzeWithProvider(
	ctx context.Context,
	provider Provider,
	report models.PipelineAnalysis,
	mode string,
	selected string,
	isFallback bool,
) (*models.AIAnalysis, error) {
	start := time.Now()

	result, err := provider.Analyze(ctx, report)
	if err != nil {
		log.Printf(
			"ai analysis failed mode=%s selected=%s fallback=%t duration=%s error=%v",
			mode,
			selected,
			isFallback,
			time.Since(start),
			err,
		)
		return nil, err
	}

	log.Printf(
		"ai analysis completed mode=%s selected=%s fallback=%t duration=%s confidence=%q owner_hint=%q",
		mode,
		selected,
		isFallback,
		time.Since(start),
		result.Confidence,
		result.OwnerHint,
	)

	return result, nil
}

func unavailableAI(err error) *models.AIAnalysis {
	if err != nil {
		log.Printf("ai analysis unavailable: %v", err)
	}

	return &models.AIAnalysis{
		Summary:      "AI analysis unavailable. Rule-based analysis was returned.",
		Confidence:   "unknown",
		OwnerHint:    "unknown",
		PrimaryCause: "AI provider failed or timed out.",
	}
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
