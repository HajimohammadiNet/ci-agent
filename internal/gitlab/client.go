package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/hajimohammadinet/ci-agent/internal/models"
)

const maxTraceBytes = 4 * 1024 * 1024 // 4 MB for MVP

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) newRequest(ctx context.Context, method, path string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v4"+path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("gitlab api error: status=%d body=%s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func projectPath(projectID string) string {
	return url.PathEscape(projectID)
}

func (c *Client) GetPipeline(ctx context.Context, projectID string, pipelineID int64) (*models.Pipeline, error) {
	path := fmt.Sprintf("/projects/%s/pipelines/%d", projectPath(projectID), pipelineID)

	req, err := c.newRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}

	var pipeline models.Pipeline
	if err := c.doJSON(req, &pipeline); err != nil {
		return nil, err
	}

	return &pipeline, nil
}

func (c *Client) ListPipelineJobs(ctx context.Context, projectID string, pipelineID int64) ([]models.Job, error) {
	var allJobs []models.Job
	page := 1

	for {
		path := fmt.Sprintf(
			"/projects/%s/pipelines/%d/jobs?per_page=100&page=%d",
			projectPath(projectID),
			pipelineID,
			page,
		)

		req, err := c.newRequest(ctx, http.MethodGet, path)
		if err != nil {
			return nil, err
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			return nil, fmt.Errorf("gitlab api error: status=%d body=%s", resp.StatusCode, string(body))
		}

		var jobs []models.Job
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			resp.Body.Close()
			return nil, err
		}

		nextPage := resp.Header.Get("X-Next-Page")
		resp.Body.Close()

		allJobs = append(allJobs, jobs...)

		if nextPage == "" {
			break
		}

		n, err := strconv.Atoi(nextPage)
		if err != nil {
			break
		}

		page = n
	}

	return allJobs, nil
}

func (c *Client) GetJobTrace(ctx context.Context, projectID string, jobID int64) (string, error) {
	path := fmt.Sprintf("/projects/%s/jobs/%d/trace", projectPath(projectID), jobID)

	req, err := c.newRequest(ctx, http.MethodGet, path)
	if err != nil {
		return "", err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("gitlab trace error: status=%d body=%s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTraceBytes))
	if err != nil {
		return "", err
	}

	return string(data), nil
}
