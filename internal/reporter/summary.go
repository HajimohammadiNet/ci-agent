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

	fmt.Fprintf(&b, "**Branch:** `%s`\n", report.Ref)
	fmt.Fprintf(&b, "**Commit:** `%s`\n", shortSHA(report.SHA))
	fmt.Fprintf(&b, "**Failed jobs:** `%d`\n", len(report.Jobs))
	fmt.Fprintf(&b, "**Findings:** `%d`\n", totalFindings)
	fmt.Fprintf(&b, "**Highest risk:** %s `%s`\n\n", riskIcon(highestRisk), highestRisk)

	if len(report.Jobs) == 0 {
		fmt.Fprintf(&b, "No failed jobs were found.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "---\n\n")

	for _, job := range report.Jobs {
		fmt.Fprintf(&b, "### `%s` — stage: `%s`\n\n", job.JobName, job.Stage)

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

func maxRisk(current, next string) string {
	rank := map[string]int{
		"unknown": 0,
		"low":     1,
		"medium":  2,
		"high":    3,
	}

	current = strings.ToLower(current)
	next = strings.ToLower(next)

	if rank[next] > rank[current] {
		return next
	}

	return current
}
