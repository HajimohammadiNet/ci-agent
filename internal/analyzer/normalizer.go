package analyzer

import (
	"regexp"
	"strings"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func NormalizeTrace(input string) string {
	output := input

	output = ansiPattern.ReplaceAllString(output, "")
	output = strings.ReplaceAll(output, "\r", "")
	output = strings.ReplaceAll(output, "\x1b[0K", "")

	return output
}
