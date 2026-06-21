package agent

import (
	"context"
	"fmt"

	"github.com/hajimohammadinet/ci-agent/internal/analyzer"
	"github.com/hajimohammadinet/ci-agent/internal/gitlab"
	"github.com/hajimohammadinet/ci-agent/internal/models"
)

type AnalyzeRequest struct {
	URL        string `json:"url"`
	ProjectID  string `json:"project_id"`
	PipelineID int64  `json:"pipeline_id"`
}

type Service struct {
	gitlabClient    *gitlab.Client
	analysisService *analyzer.Service
}

func NewService(gitlabClient *gitlab.Client) *Service {
	return &Service{
		gitlabClient:    gitlabClient,
		analysisService: analyzer.NewService(),
	}
}

func (s *Service) Analyze(ctx context.Context, req AnalyzeRequest) (*models.PipelineAnalysis, error) {
	projectID := req.ProjectID
	pipelineID := req.PipelineID

	if req.URL != "" {
		parsed, err := gitlab.ParseGitLabURL(req.URL)
		if err != nil {
			return nil, fmt.Errorf("parse gitlab url failed: %w", err)
		}

		projectID = parsed.ProjectID

		switch parsed.Kind {
		case gitlab.URLKindPipeline:
			pipelineID = parsed.PipelineID

		case gitlab.URLKindJob:
			job, err := s.gitlabClient.GetJob(ctx, projectID, parsed.JobID)
			if err != nil {
				return nil, fmt.Errorf("get job failed: %w", err)
			}

			if job.Pipeline == nil || job.Pipeline.ID == 0 {
				return nil, fmt.Errorf("resolve pipeline from job failed: job does not contain pipeline information")
			}

			pipelineID = job.Pipeline.ID

		default:
			return nil, fmt.Errorf("unsupported gitlab url kind: %s", parsed.Kind)
		}
	}

	if projectID == "" || pipelineID == 0 {
		return nil, fmt.Errorf("either url or both project_id and pipeline_id are required")
	}

	pipeline, err := s.gitlabClient.GetPipeline(ctx, projectID, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("get pipeline failed: %w", err)
	}

	jobs, err := s.gitlabClient.ListPipelineJobs(ctx, projectID, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("list pipeline jobs failed: %w", err)
	}

	report := &models.PipelineAnalysis{
		ProjectID:  projectID,
		PipelineID: pipelineID,
		Status:     pipeline.Status,
		Ref:        pipeline.Ref,
		SHA:        pipeline.SHA,
		WebURL:     pipeline.WebURL,
		Jobs:       []models.JobAnalysis{},
	}

	for _, job := range jobs {
		if job.Status != "failed" {
			continue
		}

		trace, err := s.gitlabClient.GetJobTrace(ctx, projectID, job.ID)
		if err != nil {
			report.Jobs = append(report.Jobs, models.JobAnalysis{
				JobID:   job.ID,
				JobName: job.Name,
				Stage:   job.Stage,
				Findings: []models.Finding{
					{
						Category:     "trace_fetch_failed",
						RootCause:    "The agent could not fetch the failed job trace.",
						Evidence:     []string{err.Error()},
						SuggestedFix: "Check GitLab token permissions and job trace availability.",
						RetrySafe:    false,
						RiskLevel:    "unknown",
					},
				},
			})

			continue
		}

		jobAnalysis := s.analysisService.AnalyzeJob(job, trace)
		report.Jobs = append(report.Jobs, jobAnalysis)
	}

	return report, nil
}
