package analyzer

import (
	"strings"

	"github.com/hajimohammadinet/ci-agent/internal/models"
)

type Service struct {
	rules []Rule
}

type Rule struct {
	Category     string
	RootCause    string
	SuggestedFix string
	RetrySafe    bool
	RiskLevel    string

	// All required patterns must exist in the trace.
	Required []string

	// At least one of these patterns must exist.
	Any []string
}

func NewService() *Service {
	return &Service{
		rules: defaultRules(),
	}
}

func (s *Service) AnalyzeJob(job models.Job, trace string) models.JobAnalysis {
	cleanTrace := NormalizeTrace(RedactSecrets(trace))
	lower := strings.ToLower(cleanTrace)

	findings := make([]models.Finding, 0)

	for _, rule := range s.rules {
		if !rule.Match(lower) {
			continue
		}

		findings = append(findings, models.Finding{
			Category:     rule.Category,
			RootCause:    rule.RootCause,
			Evidence:     extractEvidenceForRule(cleanTrace, rule),
			SuggestedFix: rule.SuggestedFix,
			RetrySafe:    rule.RetrySafe,
			RiskLevel:    rule.RiskLevel,
		})
	}

	if len(findings) == 0 {
		findings = append(findings, models.Finding{
			Category:     "unknown_failure",
			RootCause:    "The job failed, but the current rule engine could not determine a specific root cause.",
			Evidence:     extractEvidence(cleanTrace),
			SuggestedFix: "Review the failed command and the last part of the job log. Consider adding a new rule for this failure pattern.",
			RetrySafe:    false,
			RiskLevel:    "unknown",
		})
	}

	return models.JobAnalysis{
		JobID:    job.ID,
		JobName:  job.Name,
		Stage:    job.Stage,
		Findings: findings,
	}
}

func (r Rule) Match(trace string) bool {
	for _, required := range r.Required {
		if !strings.Contains(trace, strings.ToLower(required)) {
			return false
		}
	}

	if len(r.Any) == 0 {
		return true
	}

	for _, pattern := range r.Any {
		if strings.Contains(trace, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

func defaultRules() []Rule {
	return []Rule{
		{
			Category:     "helm_repository_auth_failure",
			Required:     []string{"helm", "401 unauthorized"},
			Any:          []string{"index.yaml", "packages/helm", "failed to fetch"},
			RootCause:    "The Helm repository could not be accessed because authentication failed.",
			SuggestedFix: "Check Helm repository credentials, GitLab package registry token, deploy token permissions, and helm repo add configuration in CI variables.",
			RetrySafe:    false,
			RiskLevel:    "medium",
		},
		{
			Category:     "kubernetes_helm_rollout_timeout",
			Required:     []string{"helm", "resource not ready"},
			Any:          []string{"context deadline exceeded", "deployment", "inprogress", "upgrade failed"},
			RootCause:    "The Helm deployment failed because a Kubernetes resource did not become ready before the timeout.",
			SuggestedFix: "Check the target Deployment pods, events, readiness probes, image pull status, resource limits, and application logs. Increase Helm timeout only after confirming the app is healthy but slow.",
			RetrySafe:    false,
			RiskLevel:    "high",
		},
		{
			Category:     "go_dependency_download_failure",
			Required:     []string{"go mod download"},
			Any:          []string{"exit code: 1", "failed to solve", "did not complete successfully"},
			RootCause:    "The Docker build failed while running 'go mod download'. This is usually caused by private module access, network/DNS problems, invalid Go module configuration, or missing Git credentials inside the Docker build context.",
			SuggestedFix: "Check GOPRIVATE, Git credentials for private Go modules, go.mod module paths, network access from the runner, and whether the Dockerfile has access to required credentials during 'go mod download'.",
			RetrySafe:    true,
			RiskLevel:    "medium",
		},
		{
			Category:     "docker_build_failure",
			Required:     []string{"failed to solve"},
			Any:          []string{"dockerfile", "did not complete successfully", "failed to build", "error:"},
			RootCause:    "The Docker image build failed.",
			SuggestedFix: "Check the Dockerfile step shown in the error, required files in build context, base image availability, dependency installation, and build arguments.",
			RetrySafe:    false,
			RiskLevel:    "medium",
		},
		{
			Category:     "runner_disk_full",
			Required:     []string{"no space left on device"},
			RootCause:    "The GitLab runner or Docker environment appears to be out of disk space.",
			SuggestedFix: "Clean Docker images, build cache, artifacts, temporary files, or expand the runner disk. Retry after freeing disk space.",
			RetrySafe:    true,
			RiskLevel:    "medium",
		},
		{
			Category:     "registry_auth_failure",
			Required:     []string{},
			Any:          []string{"unauthorized: authentication required", "denied: requested access to the resource is denied"},
			RootCause:    "The job failed because Docker registry authentication or permission was rejected.",
			SuggestedFix: "Check registry credentials, deploy token, CI_REGISTRY_USER, CI_REGISTRY_PASSWORD, image repository path, and project registry permissions.",
			RetrySafe:    false,
			RiskLevel:    "medium",
		},
		{
			Category:     "runner_docker_daemon_failure",
			Required:     []string{"cannot connect to the docker daemon"},
			RootCause:    "The job could not connect to Docker daemon on the runner.",
			SuggestedFix: "Check runner executor type, Docker service status, socket permissions, and privileged mode if using Docker-in-Docker.",
			RetrySafe:    false,
			RiskLevel:    "medium",
		},
		{
			Category:     "frontend_dependency_or_build_failure",
			Required:     []string{},
			Any:          []string{"npm err!", "pnpm", "yarn error"},
			RootCause:    "The frontend dependency installation or build command failed.",
			SuggestedFix: "Check Node.js version, package lock file, package registry access, dependency versions, and the exact build output.",
			RetrySafe:    false,
			RiskLevel:    "low",
		},
		{
			Category:     "frontend_typescript_failure",
			Required:     []string{},
			Any:          []string{"typescript error", "ts error", "type error"},
			RootCause:    "The frontend build failed due to a TypeScript/type-checking error.",
			SuggestedFix: "Fix the TypeScript error in the source code or update the related type definitions.",
			RetrySafe:    false,
			RiskLevel:    "low",
		},
		{
			Category:     "go_test_failure",
			Required:     []string{},
			Any:          []string{"go test", "--- fail:", "fail\t"},
			RootCause:    "One or more Go tests failed.",
			SuggestedFix: "Inspect the failing test names and recent code changes. Retry only if the failure is known to be flaky.",
			RetrySafe:    false,
			RiskLevel:    "low",
		},
		{
			Category:     "java_build_failure",
			Required:     []string{},
			Any:          []string{"mvn", "gradle", "build failed"},
			RootCause:    "The Java/Maven/Gradle build failed.",
			SuggestedFix: "Check dependency resolution, test failures, JDK version, Gradle/Maven settings, and repository access.",
			RetrySafe:    false,
			RiskLevel:    "low",
		},
		{
			Category:     "kubernetes_image_pull_failure",
			Required:     []string{},
			Any:          []string{"imagepullbackoff", "errimagepull"},
			RootCause:    "Kubernetes could not pull the container image.",
			SuggestedFix: "Check image tag, registry access, imagePullSecrets, and whether the image was pushed successfully.",
			RetrySafe:    false,
			RiskLevel:    "high",
		},
		{
			Category:     "ssh_deploy_auth_failure",
			Required:     []string{},
			Any:          []string{"permission denied (publickey)", "ssh: handshake failed"},
			RootCause:    "The direct server deployment failed because SSH authentication failed.",
			SuggestedFix: "Check SSH private key, deploy user, authorized_keys, known_hosts, and GitLab CI variables.",
			RetrySafe:    false,
			RiskLevel:    "medium",
		},
		{
			Category:     "network_or_dns_failure",
			Required:     []string{},
			Any:          []string{"connection timed out", "i/o timeout", "temporary failure in name resolution"},
			RootCause:    "The job failed due to a network, DNS, or connectivity problem.",
			SuggestedFix: "Check runner network, DNS, proxy, firewall, registry access, and target service availability.",
			RetrySafe:    true,
			RiskLevel:    "medium",
		},
	}
}

func extractEvidenceForRule(trace string, rule Rule) []string {
	lines := strings.Split(trace, "\n")

	keywords := make([]string, 0)
	keywords = append(keywords, rule.Required...)
	keywords = append(keywords, rule.Any...)

	var evidence []string
	seen := map[string]bool{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		lower := strings.ToLower(trimmed)

		if strings.HasPrefix(lower, "section_start:") || strings.HasPrefix(lower, "section_end:") {
			continue
		}

		for _, keyword := range keywords {
			if keyword == "" {
				continue
			}

			if strings.Contains(lower, strings.ToLower(keyword)) {
				if !seen[trimmed] {
					evidence = append(evidence, trimmed)
					seen[trimmed] = true
				}
				break
			}
		}
	}

	if len(evidence) > 10 {
		return evidence[len(evidence)-10:]
	}

	if len(evidence) == 0 {
		return extractEvidence(trace)
	}

	return evidence
}

func extractEvidence(trace string) []string {
	lines := strings.Split(trace, "\n")

	var evidence []string
	seen := map[string]bool{}

	keywords := []string{
		"error",
		"failed",
		"fatal",
		"denied",
		"unauthorized",
		"timeout",
		"no space left",
		"permission denied",
		"crashloopbackoff",
		"imagepullbackoff",
		"errimagepull",
		"context deadline exceeded",
		"go mod download",
		"failed to solve",
		"resource not ready",
		"upgrade failed",
		"401 unauthorized",
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		lower := strings.ToLower(trimmed)

		if strings.HasPrefix(lower, "section_start:") || strings.HasPrefix(lower, "section_end:") {
			continue
		}

		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				if !seen[trimmed] {
					evidence = append(evidence, trimmed)
					seen[trimmed] = true
				}
				break
			}
		}
	}

	if len(evidence) > 10 {
		return evidence[len(evidence)-10:]
	}

	if len(evidence) == 0 {
		return tail(lines, 10)
	}

	return evidence
}

func tail(lines []string, n int) []string {
	var out []string

	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "section_start:") || strings.HasPrefix(lower, "section_end:") {
			continue
		}

		out = append(out, trimmed)
	}

	return out
}
