package models

type Pipeline struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	Ref    string `json:"ref"`
	SHA    string `json:"sha"`
	WebURL string `json:"web_url"`
}

type Runner struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Online      bool   `json:"online"`
}

type Job struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Stage        string    `json:"stage"`
	Status       string    `json:"status"`
	AllowFailure bool      `json:"allow_failure"`
	WebURL       string    `json:"web_url"`
	TagList      []string  `json:"tag_list"`
	Runner       *Runner   `json:"runner"`
	Pipeline     *Pipeline `json:"pipeline"`
}

type JobAnalysis struct {
	JobID        int64    `json:"job_id"`
	JobName      string   `json:"job_name"`
	Stage        string   `json:"stage"`
	Category     string   `json:"category"`
	RootCause    string   `json:"root_cause"`
	Evidence     []string `json:"evidence"`
	SuggestedFix string   `json:"suggested_fix"`
	RetrySafe    bool     `json:"retry_safe"`
	RiskLevel    string   `json:"risk_level"`
}

type PipelineAnalysis struct {
	ProjectID  string        `json:"project_id"`
	PipelineID int64         `json:"pipeline_id"`
	Status     string        `json:"status"`
	Ref        string        `json:"ref"`
	SHA        string        `json:"sha"`
	WebURL     string        `json:"web_url"`
	Jobs       []JobAnalysis `json:"jobs"`
}
