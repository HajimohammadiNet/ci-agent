package reporter

import (
	"fmt"
	"strings"

	"github.com/hajimohammadinet/ci-agent/internal/models"
)

func Markdown(report models.PipelineAnalysis) string {
	var b strings.Builder

	totalFindings := 0
	for _, job := range report.Jobs {
		totalFindings += len(job.Findings)
	}

	fmt.Fprintf(&b, "## %s CI/CD Failure Analysis\n\n", statusIcon(report.Status))

	fmt.Fprintf(&b, "**Project:** `%s`\n\n", report.ProjectID)

	if report.WebURL != "" {
		fmt.Fprintf(&b, "**Pipeline:** [%d](%s)\n\n", report.PipelineID, report.WebURL)
	} else {
		fmt.Fprintf(&b, "**Pipeline:** `%d`\n\n", report.PipelineID)
	}

	fmt.Fprintf(&b, "**Status:** `%s`\n\n", report.Status)
	fmt.Fprintf(&b, "**Branch/Ref:** `%s`\n\n", report.Ref)
	fmt.Fprintf(&b, "**Commit:** `%s`\n\n", shortSHA(report.SHA))
	fmt.Fprintf(&b, "**Failed jobs:** `%d`\n\n", len(report.Jobs))
	fmt.Fprintf(&b, "**Findings:** `%d`\n\n", totalFindings)

	if len(report.Jobs) == 0 {
		fmt.Fprintf(&b, "---\n\n")
		fmt.Fprintf(&b, "No failed jobs were found in this pipeline.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "---\n\n")

	for _, job := range report.Jobs {
		fmt.Fprintf(&b, "### Failed Job: `%s`\n\n", job.JobName)
		fmt.Fprintf(&b, "- **Stage:** `%s`\n", job.Stage)
		fmt.Fprintf(&b, "- **Job ID:** `%d`\n", job.JobID)
		fmt.Fprintf(&b, "- **Findings:** `%d`\n\n", len(job.Findings))

		for i, finding := range job.Findings {
			fmt.Fprintf(&b, "#### %d. %s `%s`\n\n", i+1, riskIcon(finding.RiskLevel), finding.Category)

			fmt.Fprintf(&b, "**Root cause:**\n\n")
			fmt.Fprintf(&b, "%s\n\n", finding.RootCause)

			fmt.Fprintf(&b, "**Risk:** `%s`\n\n", finding.RiskLevel)
			fmt.Fprintf(&b, "**Retry safe:** `%t`\n\n", finding.RetrySafe)

			fmt.Fprintf(&b, "**Suggested fix:**\n\n")
			fmt.Fprintf(&b, "%s\n\n", finding.SuggestedFix)

			if len(finding.Evidence) > 0 {
				fmt.Fprintf(&b, "**Evidence:**\n\n")

				for _, ev := range finding.Evidence {
					fmt.Fprintf(&b, "- `%s`\n", sanitizeInlineCode(truncate(ev, 400)))
				}

				fmt.Fprintf(&b, "\n")
			}
		}

		fmt.Fprintf(&b, "---\n\n")
	}

	return b.String()
}

func statusIcon(status string) string {
	switch strings.ToLower(status) {
	case "success", "passed":
		return "✅"
	case "failed":
		return "❌"
	case "running":
		return "🏃"
	case "canceled", "cancelled":
		return "🚫"
	default:
		return "ℹ️"
	}
}

func riskIcon(risk string) string {
	switch strings.ToLower(risk) {
	case "high":
		return "🔴"
	case "medium":
		return "🟠"
	case "low":
		return "🟢"
	default:
		return "⚪"
	}
}

func shortSHA(sha string) string {
	if len(sha) <= 8 {
		return sha
	}

	return sha[:8]
}

func truncate(input string, max int) string {
	input = strings.TrimSpace(input)

	if len(input) <= max {
		return input
	}

	return input[:max] + "..."
}

func sanitizeInlineCode(input string) string {
	return strings.ReplaceAll(input, "`", "'")
}
