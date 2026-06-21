package gitlab

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type URLKind string

const (
	URLKindPipeline URLKind = "pipeline"
	URLKindJob      URLKind = "job"
)

type ParsedURL struct {
	Kind       URLKind
	ProjectID  string
	PipelineID int64
	JobID      int64
}

func ParseGitLabURL(raw string) (*ParsedURL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid gitlab url: %w", err)
	}

	path := strings.Trim(u.Path, "/")
	if path == "" {
		return nil, fmt.Errorf("invalid gitlab url: empty path")
	}

	parts := strings.Split(path, "/")

	dashIndex := -1
	for i, part := range parts {
		if part == "-" {
			dashIndex = i
			break
		}
	}

	if dashIndex <= 0 {
		return nil, fmt.Errorf("invalid gitlab url: could not find project path before /-/")
	}

	if len(parts) < dashIndex+3 {
		return nil, fmt.Errorf("invalid gitlab url: incomplete path after /-/")
	}

	projectID := strings.Join(parts[:dashIndex], "/")
	resourceType := parts[dashIndex+1]
	resourceIDRaw := parts[dashIndex+2]

	resourceID, err := strconv.ParseInt(resourceIDRaw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid gitlab url: invalid resource id %q", resourceIDRaw)
	}

	switch resourceType {
	case "pipelines":
		return &ParsedURL{
			Kind:       URLKindPipeline,
			ProjectID:  projectID,
			PipelineID: resourceID,
		}, nil

	case "jobs":
		return &ParsedURL{
			Kind:      URLKindJob,
			ProjectID: projectID,
			JobID:     resourceID,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported gitlab url type: %s", resourceType)
	}
}
