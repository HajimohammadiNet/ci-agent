package reporter

import (
	"fmt"
	"strings"

	"github.com/hajimohammadinet/ci-agent/internal/models"
)

func Summary(report models.PipelineAnalysis) string {
	var b strings.Builder

	totalFindings := 0
	highestRisk := "unknown"

	for _, job := range report.Jobs {
		for _, finding := range job.Findings {
			totalFindings++
			highestRisk = maxRisk(highestRisk, finding.RiskLevel)
		}
	}

	fmt.Fprintf(&b, "## %s CI/CD Failure Summary\n\n", statusIcon(report.Status))

	fmt.Fprintf(&b, "**Project:** `%s`\n", report.ProjectID)

	if report.WebURL != "" {
		fmt.Fprintf(&b, "**Pipeline:** [%d](%s)\n", report.PipelineID, report.WebURL)
	} else {
		fmt.Fprintf(&b, "**Pipeline:** `%d`\n", report.PipelineID)
	}

	fmt.Fprintf(&b, "**Status:** `%s`\n", report.Status)
	fmt.Fprintf(&b, "**Branch:** `%s`\n", report.Ref)
	fmt.Fprintf(&b, "**Commit:** `%s`\n", shortSHA(report.SHA))
	fmt.Fprintf(&b, "**Failed jobs:** `%d`\n", len(report.Jobs))
	fmt.Fprintf(&b, "**Findings:** `%d`\n", totalFindings)
	fmt.Fprintf(&b, "**Highest risk:** %s `%s`\n\n", riskIcon(highestRisk), highestRisk)

	if report.AI != nil {
		writeAISummarySection(&b, report.AI)
	}

	if len(report.Jobs) == 0 {
		fmt.Fprintf(&b, "No failed jobs were found.\n")
		return b.String()
	}

	if report.AI == nil {
		fmt.Fprintf(&b, "---\n\n")
	}

	fmt.Fprintf(&b, "### Rule-Based Findings\n\n")

	for _, job := range report.Jobs {
		fmt.Fprintf(&b, "#### `%s` — stage: `%s`\n\n", job.JobName, job.Stage)

		if len(job.Findings) == 0 {
			fmt.Fprintf(&b, "- No rule-based findings detected for this job.\n\n")
			continue
		}

		for _, finding := range job.Findings {
			fmt.Fprintf(&b, "- %s `%s`\n", riskIcon(finding.RiskLevel), finding.Category)
			fmt.Fprintf(&b, "  - **Root cause:** %s\n", finding.RootCause)
			fmt.Fprintf(&b, "  - **Retry safe:** `%t`\n", finding.RetrySafe)
			fmt.Fprintf(&b, "  - **Fix:** %s\n", finding.SuggestedFix)
		}

		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "---\n\n")
	fmt.Fprintf(&b, "_For full evidence, run:_ `/ciagent full <gitlab-job-or-pipeline-url>`\n")

	return b.String()
}

func writeAISummarySection(b *strings.Builder, ai *models.AIAnalysis) {
	fmt.Fprintf(b, "### 🤖 AI Analysis\n\n")

	if strings.TrimSpace(ai.Summary) != "" {
		fmt.Fprintf(b, "**Summary:** %s\n\n", ai.Summary)
	}

	if strings.TrimSpace(ai.PrimaryCause) != "" {
		fmt.Fprintf(b, "**Primary cause:** %s\n\n", ai.PrimaryCause)
	}

	if len(ai.SecondaryCauses) > 0 {
		fmt.Fprintf(b, "**Secondary causes:**\n")
		for _, cause := range ai.SecondaryCauses {
			cause = strings.TrimSpace(cause)
			if cause == "" {
				continue
			}
			fmt.Fprintf(b, "- %s\n", cause)
		}
		fmt.Fprintf(b, "\n")
	}

	if len(ai.RecommendedSteps) > 0 {
		fmt.Fprintf(b, "**Recommended next steps:**\n")
		for i, step := range ai.RecommendedSteps {
			step = strings.TrimSpace(step)
			if step == "" {
				continue
			}
			fmt.Fprintf(b, "%d. %s\n", i+1, step)
		}
		fmt.Fprintf(b, "\n")
	}

	if strings.TrimSpace(ai.OwnerHint) != "" {
		fmt.Fprintf(b, "**Owner hint:** `%s`\n", ai.OwnerHint)
	}

	if strings.TrimSpace(ai.Confidence) != "" {
		fmt.Fprintf(b, "**AI confidence:** `%s`\n", ai.Confidence)
	}

	fmt.Fprintf(b, "\n---\n\n")
}

func maxRisk(current, next string) string {
	rank := map[string]int{
		"unknown": 0,
		"low":     1,
		"medium":  2,
		"high":    3,
	}

	current = strings.ToLower(strings.TrimSpace(current))
	next = strings.ToLower(strings.TrimSpace(next))

	if _, ok := rank[current]; !ok {
		current = "unknown"
	}

	if _, ok := rank[next]; !ok {
		next = "unknown"
	}

	if rank[next] > rank[current] {
		return next
	}

	return current
}
