package analyzer

import (
	"strings"

	"github.com/hajimohammadinet/ci-agent/internal/models"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) AnalyzeJob(job models.Job, trace string) models.JobAnalysis {
	cleanTrace := RedactSecrets(trace)
	lower := strings.ToLower(cleanTrace)

	category := "unknown_failure"
	rootCause := "The job failed, but the current rule engine could not determine a specific root cause."
	suggestedFix := "Review the failed command and the last part of the job log."
	retrySafe := false
	risk := "unknown"

	switch {
	case containsAny(lower, "no space left on device"):
		category = "runner_disk_full"
		rootCause = "The GitLab runner or Docker environment appears to be out of disk space."
		suggestedFix = "Clean Docker images, build cache, artifacts, or expand the runner disk. Retry after freeing disk space."
		retrySafe = true
		risk = "medium"

	case containsAny(lower, "unauthorized: authentication required", "denied: requested access to the resource is denied"):
		category = "registry_auth_failure"
		rootCause = "The job failed because Docker registry authentication or permission was rejected."
		suggestedFix = "Check registry credentials, deploy token, CI_REGISTRY_USER, CI_REGISTRY_PASSWORD, and image repository path."
		retrySafe = false
		risk = "medium"

	case containsAny(lower, "cannot connect to the docker daemon"):
		category = "runner_docker_daemon_failure"
		rootCause = "The job could not connect to Docker daemon on the runner."
		suggestedFix = "Check runner executor type, Docker service status, socket permissions, and privileged mode if using Docker-in-Docker."
		retrySafe = false
		risk = "medium"

	case containsAny(lower, "npm err!", "pnpm", "yarn error"):
		category = "frontend_dependency_or_build_failure"
		rootCause = "The frontend dependency installation or build command failed."
		suggestedFix = "Check Node.js version, package lock file, registry access, dependency versions, and build output."
		retrySafe = false
		risk = "low"

	case containsAny(lower, "typescript error", "ts error", "type error"):
		category = "frontend_typescript_failure"
		rootCause = "The frontend build failed due to a TypeScript/type-checking error."
		suggestedFix = "Fix the TypeScript error in the source code or update the related type definitions."
		retrySafe = false
		risk = "low"

	case containsAny(lower, "go test", "--- fail:", "fail\t"):
		category = "go_test_failure"
		rootCause = "One or more Go tests failed."
		suggestedFix = "Inspect the failing test names and recent code changes. Retry only if the failure is known to be flaky."
		retrySafe = false
		risk = "low"

	case containsAny(lower, "mvn", "gradle", "build failed"):
		category = "java_build_failure"
		rootCause = "The Java/Maven/Gradle build failed."
		suggestedFix = "Check dependency resolution, test failures, JDK version, Gradle/Maven settings, and repository access."
		retrySafe = false
		risk = "low"

	case containsAny(lower, "helm upgrade", "upgrade failed", "helm template"):
		category = "kubernetes_helm_deploy_failure"
		rootCause = "The Kubernetes/Helm deployment failed."
		suggestedFix = "Check Helm values, chart templates, Kubernetes events, image tags, ingress, secrets, and readiness probes."
		retrySafe = false
		risk = "high"

	case containsAny(lower, "imagepullbackoff", "errimagepull"):
		category = "kubernetes_image_pull_failure"
		rootCause = "Kubernetes could not pull the container image."
		suggestedFix = "Check image tag, registry access, imagePullSecrets, and whether the image was pushed successfully."
		retrySafe = false
		risk = "high"

	case containsAny(lower, "permission denied (publickey)", "ssh: handshake failed"):
		category = "ssh_deploy_auth_failure"
		rootCause = "The direct server deployment failed because SSH authentication failed."
		suggestedFix = "Check SSH private key, deploy user, authorized_keys, known_hosts, and GitLab CI variables."
		retrySafe = false
		risk = "medium"

	case containsAny(lower, "connection timed out", "i/o timeout", "temporary failure in name resolution"):
		category = "network_or_dns_failure"
		rootCause = "The job failed due to a network, DNS, or connectivity problem."
		suggestedFix = "Check runner network, DNS, proxy, firewall, registry access, and target service availability."
		retrySafe = true
		risk = "medium"
	}

	return models.JobAnalysis{
		JobID:        job.ID,
		JobName:      job.Name,
		Stage:        job.Stage,
		Category:     category,
		RootCause:    rootCause,
		Evidence:     extractEvidence(cleanTrace),
		SuggestedFix: suggestedFix,
		RetrySafe:    retrySafe,
		RiskLevel:    risk,
	}
}

func containsAny(input string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(input, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func extractEvidence(trace string) []string {
	lines := strings.Split(trace, "\n")

	var evidence []string
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
	}

	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					evidence = append(evidence, trimmed)
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
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}
