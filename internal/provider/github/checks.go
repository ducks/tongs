package github

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ducks/tongs/internal/provider"
)

func (c *Client) Checks(ctx context.Context, repo provider.Repository, sha string) (provider.Checks, error) {
	type apiCheck struct {
		Name        string     `json:"name"`
		Status      string     `json:"status"`
		Conclusion  *string    `json:"conclusion"`
		DetailsURL  string     `json:"details_url"`
		StartedAt   *time.Time `json:"started_at"`
		CompletedAt *time.Time `json:"completed_at"`
	}
	base := repoPath(repo) + "/commits/" + url.PathEscape(sha)
	type apiStatus struct {
		Context     string `json:"context"`
		State       string `json:"state"`
		Description string `json:"description"`
		TargetURL   string `json:"target_url"`
	}

	result := provider.Checks{Overall: "success", Checks: []provider.Check{}, Statuses: []provider.Status{}}
	for page := 1; ; page++ {
		var response struct {
			CheckRuns []apiCheck `json:"check_runs"`
		}
		requestPath := base + "/check-runs?per_page=100&page=" + strconv.Itoa(page)
		if err := c.rest(ctx, http.MethodGet, requestPath, nil, &response); err != nil {
			return provider.Checks{}, err
		}
		for _, check := range response.CheckRuns {
			mapped := provider.Check{
				Name:        check.Name,
				Status:      check.Status,
				Conclusion:  stringValue(check.Conclusion),
				URL:         check.DetailsURL,
				StartedAt:   check.StartedAt,
				CompletedAt: check.CompletedAt,
			}
			result.Checks = append(result.Checks, mapped)
			result.Overall = combineOverall(result.Overall, check.Status, mapped.Conclusion)
		}
		if len(response.CheckRuns) < 100 {
			break
		}
	}
	for page := 1; ; page++ {
		var response struct {
			Statuses []apiStatus `json:"statuses"`
		}
		requestPath := base + "/status?per_page=100&page=" + strconv.Itoa(page)
		if err := c.rest(ctx, http.MethodGet, requestPath, nil, &response); err != nil {
			return provider.Checks{}, err
		}
		for _, status := range response.Statuses {
			result.Statuses = append(result.Statuses, provider.Status{
				Context:     status.Context,
				State:       status.State,
				Description: status.Description,
				URL:         status.TargetURL,
			})
			result.Overall = combineOverall(result.Overall, status.State, status.State)
		}
		if len(response.Statuses) < 100 {
			break
		}
	}
	if len(result.Checks) == 0 && len(result.Statuses) == 0 {
		result.Overall = "none"
	}
	return result, nil
}

func combineOverall(current, status, conclusion string) string {
	status = strings.ToLower(status)
	conclusion = strings.ToLower(conclusion)
	if status == "failure" || status == "error" || conclusion == "failure" || conclusion == "timed_out" || conclusion == "cancelled" || conclusion == "action_required" {
		return "failure"
	}
	if current != "failure" && (status == "queued" || status == "in_progress" || status == "pending" || conclusion == "") {
		return "pending"
	}
	return current
}
