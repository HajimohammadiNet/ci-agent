package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajimohammadinet/ci-agent/internal/analyzer"
	"github.com/hajimohammadinet/ci-agent/internal/gitlab"
	"github.com/hajimohammadinet/ci-agent/internal/models"
)

type AnalyzeRequest struct {
	URL        string `json:"url"`
	ProjectID  string `json:"project_id"`
	PipelineID int64  `json:"pipeline_id"`
	AIMode     string `json:"ai_mode"`
}

type AIAnalyzer interface {
	Analyze(ctx context.Context, report models.PipelineAnalysis, mode string) *models.AIAnalysis
}

type Service struct {
	gitlabClient    *gitlab.Client
	analysisService *analyzer.Service
	ai              AIAnalyzer
}

func NewService(gitlabClient *gitlab.Client, aiAnalyzer AIAnalyzer) *Service {
	return &Service{
		gitlabClient:    gitlabClient,
		analysisService: analyzer.NewService(),
		ai:              aiAnalyzer,
	}
}

func (s *Service) Analyze(ctx context.Context, req AnalyzeRequest) (*models.PipelineAnalysis, error) {
	if s == nil {
		return nil, fmt.Errorf("agent service is nil")
	}

	if s.gitlabClient == nil {
		return nil, fmt.Errorf("gitlab client is not configured")
	}

	if s.analysisService == nil {
		s.analysisService = analyzer.NewService()
	}

	projectID, pipelineID, err := s.resolveTarget(ctx, req)
	if err != nil {
		return nil, err
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
		Jobs:       make([]models.JobAnalysis, 0),
	}

	for _, job := range jobs {
		if strings.ToLower(strings.TrimSpace(job.Status)) != "failed" {
			continue
		}

		trace, err := s.gitlabClient.GetJobTrace(ctx, projectID, job.ID)
		if err != nil {
			report.Jobs = append(report.Jobs, traceFetchFailedJobAnalysis(job.ID, job.Name, job.Stage, err))
			continue
		}

		jobAnalysis := s.analysisService.AnalyzeJob(job, trace)
		report.Jobs = append(report.Jobs, jobAnalysis)
	}

	if s.ai != nil && shouldRunAI(report, req.AIMode) {
		report.AI = s.ai.Analyze(ctx, *report, req.AIMode)
	}

	return report, nil
}

func (s *Service) resolveTarget(ctx context.Context, req AnalyzeRequest) (string, int64, error) {
	projectID := strings.TrimSpace(req.ProjectID)
	pipelineID := req.PipelineID

	if strings.TrimSpace(req.URL) == "" {
		if projectID == "" || pipelineID == 0 {
			return "", 0, fmt.Errorf("either url or both project_id and pipeline_id are required")
		}

		return projectID, pipelineID, nil
	}

	parsed, err := gitlab.ParseGitLabURL(req.URL)
	if err != nil {
		return "", 0, fmt.Errorf("parse gitlab url failed: %w", err)
	}

	projectID = strings.TrimSpace(parsed.ProjectID)
	if projectID == "" {
		return "", 0, fmt.Errorf("parse gitlab url failed: project id is empty")
	}

	switch parsed.Kind {
	case gitlab.URLKindPipeline:
		if parsed.PipelineID == 0 {
			return "", 0, fmt.Errorf("parse gitlab url failed: pipeline id is empty")
		}

		return projectID, parsed.PipelineID, nil

	case gitlab.URLKindJob:
		if parsed.JobID == 0 {
			return "", 0, fmt.Errorf("parse gitlab url failed: job id is empty")
		}

		job, err := s.gitlabClient.GetJob(ctx, projectID, parsed.JobID)
		if err != nil {
			return "", 0, fmt.Errorf("get job failed: %w", err)
		}

		if job.Pipeline == nil || job.Pipeline.ID == 0 {
			return "", 0, fmt.Errorf("resolve pipeline from job failed: job does not contain pipeline information")
		}

		return projectID, job.Pipeline.ID, nil

	default:
		return "", 0, fmt.Errorf("unsupported gitlab url kind: %s", parsed.Kind)
	}
}

func traceFetchFailedJobAnalysis(jobID int64, jobName, stage string, err error) models.JobAnalysis {
	return models.JobAnalysis{
		JobID:   jobID,
		JobName: jobName,
		Stage:   stage,
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
	}
}

func shouldRunAI(report *models.PipelineAnalysis, mode string) bool {
	if report == nil {
		return false
	}

	mode = normalizeAIMode(mode)
	if mode == "off" {
		return false
	}

	// Do not spend AI tokens on successful pipelines.
	switch strings.ToLower(strings.TrimSpace(report.Status)) {
	case "success", "passed":
		return false
	}

	// No failed jobs were added to the report.
	if len(report.Jobs) == 0 {
		return false
	}

	// Run AI only when there is at least one rule-based finding.
	for _, job := range report.Jobs {
		if len(job.Findings) > 0 {
			return true
		}
	}

	return false
}

func normalizeAIMode(mode string) string {
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
