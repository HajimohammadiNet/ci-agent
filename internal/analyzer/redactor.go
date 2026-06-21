package analyzer

import "regexp"

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|token|secret|private-token|ci_job_token|ci_registry_password)\s*[:=]\s*[^ \n\r]+`),
	regexp.MustCompile(`(?i)Authorization:\s*Bearer\s+[A-Za-z0-9._\-]+`),
	regexp.MustCompile(`(?i)PRIVATE-TOKEN:\s*[A-Za-z0-9._\-]+`),
	regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-]+`),
}

func RedactSecrets(input string) string {
	output := input

	for _, pattern := range secretPatterns {
		output = pattern.ReplaceAllString(output, "[REDACTED_SECRET]")
	}

	return output
}
