package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hajimohammadinet/ci-agent/internal/agent"
	"github.com/hajimohammadinet/ci-agent/internal/ai"
	"github.com/hajimohammadinet/ci-agent/internal/gitlab"
	"github.com/hajimohammadinet/ci-agent/internal/llm/openrouter"
	"github.com/hajimohammadinet/ci-agent/internal/models"
	"github.com/hajimohammadinet/ci-agent/internal/reporter"
)

type analyzeHTTPRequest struct {
	URL        string `json:"url"`
	ProjectID  string `json:"project_id"`
	PipelineID int64  `json:"pipeline_id"`
	Format     string `json:"format"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	if len(os.Args) < 2 {
		printUsageAndExit()
	}

	switch os.Args[1] {
	case "analyze":
		runAnalyze(os.Args[2:])

	case "serve":
		runServe(os.Args[2:])

	default:
		printUsageAndExit()
	}
}

func runAnalyze(args []string) {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)

	projectIDFlag := fs.String("project-id", "", "GitLab project ID or project path")
	pipelineIDFlag := fs.Int64("pipeline-id", 0, "GitLab pipeline ID")
	urlFlag := fs.String("url", "", "GitLab pipeline or job URL")
	formatFlag := fs.String("format", "json", "Output format: json, summary, short, markdown, md, text, full")

	_ = fs.Parse(args)

	service, err := newAgentService()
	if err != nil {
		exitErr("init agent failed", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	report, err := service.Analyze(ctx, agent.AnalyzeRequest{
		URL:        *urlFlag,
		ProjectID:  *projectIDFlag,
		PipelineID: *pipelineIDFlag,
	})
	if err != nil {
		exitErr("analyze failed", err)
	}

	printReport(report, *formatFlag)
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)

	listenAddr := fs.String("listen", ":8080", "HTTP listen address")

	_ = fs.Parse(args)

	service, err := newAgentService()
	if err != nil {
		exitErr("init agent failed", err)
	}

	apiToken := strings.TrimSpace(os.Getenv("CI_AGENT_API_TOKEN"))
	if apiToken == "" {
		exitErr("missing api token", fmt.Errorf("CI_AGENT_API_TOKEN env var is required in serve mode"))
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", healthzHandler)

	analyzeHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		defer r.Body.Close()

		var req analyzeHTTPRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}

		log.Printf(
			"analyze request url=%q project_id=%q pipeline_id=%d format=%q",
			req.URL,
			req.ProjectID,
			req.PipelineID,
			req.Format,
		)

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()

		report, err := service.Analyze(ctx, agent.AnalyzeRequest{
			URL:        req.URL,
			ProjectID:  req.ProjectID,
			PipelineID: req.PipelineID,
		})
		if err != nil {
			log.Printf("analyze failed: %v", err)
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		log.Printf(
			"analyze completed project_id=%s pipeline_id=%d jobs=%d ai_enabled=%t",
			report.ProjectID,
			report.PipelineID,
			len(report.Jobs),
			report.AI != nil,
		)

		writeReport(w, report, req.Format)
	}

	mux.HandleFunc("/api/v1/analyze", logRequests(requireBearerToken(analyzeHandler, apiToken)))

	server := &http.Server{
		Addr:              *listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Printf("ci-agent listening on %s\n", *listenAddr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		exitErr("http server failed", err)
	}
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func requireBearerToken(next http.HandlerFunc, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			writeJSONError(w, http.StatusInternalServerError, "CI_AGENT_API_TOKEN is not configured")
			return
		}

		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		expected := "Bearer " + token

		if authHeader != expected {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next(w, r)
	}
}

func newAgentService() (*agent.Service, error) {
	gitlabURL := strings.TrimSpace(os.Getenv("GITLAB_URL"))
	gitlabToken := strings.TrimSpace(os.Getenv("GITLAB_TOKEN"))

	if gitlabURL == "" || gitlabToken == "" {
		return nil, fmt.Errorf("GITLAB_URL and GITLAB_TOKEN env vars are required")
	}

	client := gitlab.NewClient(gitlabURL, gitlabToken)

	aiAnalyzer := buildAIAnalyzer()

	return agent.NewService(client, aiAnalyzer), nil
}

func buildAIAnalyzer() *ai.Analyzer {
	enabled := getenvBool("AI_ENABLED", false)
	if !enabled {
		return ai.NewAnalyzer(false, nil)
	}

	providerName := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
	if providerName == "" {
		providerName = "openrouter"
	}

	switch providerName {
	case "openrouter":
		provider, err := openrouter.New(openrouter.Config{
			APIKey:      os.Getenv("OPENROUTER_API_KEY"),
			Model:       getenvDefault("OPENROUTER_MODEL", "deepseek/deepseek-v4-pro"),
			BaseURL:     getenvDefault("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1/chat/completions"),
			Timeout:     getenvDuration("OPENROUTER_TIMEOUT", 60*time.Second),
			MaxEvidence: getenvInt("OPENROUTER_MAX_EVIDENCE", 40),
			AppTitle:    getenvDefault("OPENROUTER_APP_TITLE", "ci-agent"),
			AppReferer:  os.Getenv("OPENROUTER_HTTP_REFERER"),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "AI disabled: %v\n", err)
			return ai.NewAnalyzer(false, nil)
		}

		return ai.NewAnalyzer(true, provider)

	default:
		fmt.Fprintf(os.Stderr, "AI disabled: unsupported provider %q\n", providerName)
		return ai.NewAnalyzer(false, nil)
	}
}

func printReport(report *models.PipelineAnalysis, format string) {
	format = normalizeFormat(format)

	switch format {
	case "json":
		printJSON(report)

	case "summary":
		fmt.Print(reporter.Summary(*report))

	case "markdown":
		fmt.Print(reporter.Markdown(*report))

	default:
		fmt.Fprintf(os.Stderr, "unsupported format: %s\n", format)
		os.Exit(1)
	}
}

func writeReport(w http.ResponseWriter, report *models.PipelineAnalysis, format string) {
	format = normalizeFormat(format)

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(report)

	case "summary":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(reporter.Summary(*report)))

	case "markdown":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(reporter.Markdown(*report)))

	default:
		writeJSONError(w, http.StatusBadRequest, "unsupported format: "+format)
	}
}

func normalizeFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))

	switch format {
	case "", "json":
		return "json"

	case "summary", "short":
		return "summary"

	case "markdown", "md", "text", "full":
		return "markdown"

	default:
		return format
	}
}

func printJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		exitErr("json marshal failed", err)
	}

	fmt.Println(string(data))
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: msg})
}

func exitErr(msg string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	os.Exit(1)
}

func printUsageAndExit() {
	fmt.Println("Usage:")
	fmt.Println("  ci-agent analyze --url <gitlab_pipeline_or_job_url> [--format json|summary|short|markdown|md|text|full]")
	fmt.Println("  ci-agent analyze --project-id <project_id_or_path> --pipeline-id <pipeline_id> [--format json|summary|short|markdown|md|text|full]")
	fmt.Println("  ci-agent serve [--listen :8080]")
	fmt.Println("")
	fmt.Println("Environment:")
	fmt.Println("  GITLAB_URL=https://gitlab.example.com")
	fmt.Println("  GITLAB_TOKEN=<token>")
	fmt.Println("  GITLAB_HTTP_TIMEOUT=120s")
	fmt.Println("")
	fmt.Println("HTTP server:")
	fmt.Println("  CI_AGENT_API_TOKEN=<token>")
	fmt.Println("")
	fmt.Println("AI / OpenRouter:")
	fmt.Println("  AI_ENABLED=false")
	fmt.Println("  AI_PROVIDER=openrouter")
	fmt.Println("  OPENROUTER_API_KEY=<token>")
	fmt.Println("  OPENROUTER_MODEL=deepseek/deepseek-v4-pro")
	fmt.Println("  OPENROUTER_BASE_URL=https://openrouter.ai/api/v1/chat/completions")
	fmt.Println("  OPENROUTER_TIMEOUT=60s")
	fmt.Println("  OPENROUTER_MAX_EVIDENCE=40")
	fmt.Println("  OPENROUTER_APP_TITLE=ci-agent")
	fmt.Println("  OPENROUTER_HTTP_REFERER=")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  ci-agent analyze --url https://gitlab.example.com/group/project/-/pipelines/9876")
	fmt.Println("  ci-agent analyze --url https://gitlab.example.com/group/project/-/jobs/12345 --format summary")
	fmt.Println("  ci-agent analyze --url https://gitlab.example.com/group/project/-/jobs/12345 --format full")
	fmt.Println("  ci-agent serve --listen :8080")
	os.Exit(1)
}

func getenvDefault(key string, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}

	return v
}

func getenvBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return def
	}

	switch v {
	case "1", "true", "yes", "y", "enabled":
		return true
	case "0", "false", "no", "n", "disabled":
		return false
	default:
		return def
	}
}

func getenvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}

	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}

	return i
}

func getenvDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}

	return d
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}

	n, err := r.ResponseWriter.Write(data)
	r.bytes += n
	return n, err
}

func logRequests(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &statusRecorder{
			ResponseWriter: w,
			status:         0,
		}

		log.Printf(
			"request started method=%s path=%s remote=%s",
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
		)

		next(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}

		log.Printf(
			"request completed method=%s path=%s status=%d bytes=%d duration=%s",
			r.Method,
			r.URL.Path,
			status,
			rec.bytes,
			time.Since(start),
		)
	}
}