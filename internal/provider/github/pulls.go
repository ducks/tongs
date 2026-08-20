package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ducks/tongs/internal/provider"
)

type apiUser struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

type apiPull struct {
	Number         int        `json:"number"`
	NodeID         string     `json:"node_id"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	State          string     `json:"state"`
	Draft          bool       `json:"draft"`
	HTMLURL        string     `json:"html_url"`
	User           apiUser    `json:"user"`
	Mergeable      *bool      `json:"mergeable"`
	MergeableState string     `json:"mergeable_state"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	MergedAt       *time.Time `json:"merged_at"`
	ChangedFiles   int        `json:"changed_files"`
	Additions      int        `json:"additions"`
	Deletions      int        `json:"deletions"`
	Base           struct {
		Ref string `json:"ref"`
	} `json:"base"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
}

func (c *Client) FindPullRequest(ctx context.Context, repo provider.Repository, branch string) (provider.PullRequest, error) {
	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", repo.Owner+":"+branch)
	query.Set("per_page", "100")
	var pulls []apiPull
	if err := c.rest(ctx, http.MethodGet, repoPath(repo)+"/pulls?"+query.Encode(), nil, &pulls); err != nil {
		return provider.PullRequest{}, err
	}
	if len(pulls) == 0 {
		return provider.PullRequest{}, &provider.Error{Code: "pull_request_not_found", Message: fmt.Sprintf("no open pull request found for branch %q", branch)}
	}
	if len(pulls) > 1 {
		return provider.PullRequest{}, &provider.Error{Code: "ambiguous_pull_request", Message: fmt.Sprintf("found %d open pull requests for branch %q; provide a number", len(pulls), branch)}
	}
	return mapPull(pulls[0]), nil
}

func (c *Client) GetPullRequest(ctx context.Context, repo provider.Repository, number int) (provider.PullRequest, error) {
	var pull apiPull
	if err := c.rest(ctx, http.MethodGet, repoPath(repo)+"/pulls/"+strconv.Itoa(number), nil, &pull); err != nil {
		return provider.PullRequest{}, err
	}
	return mapPull(pull), nil
}

func (c *Client) Inspect(ctx context.Context, repo provider.Repository, number int) (provider.Inspection, error) {
	pull, err := c.GetPullRequest(ctx, repo, number)
	if err != nil {
		return provider.Inspection{}, err
	}
	reviews, err := c.Reviews(ctx, repo, number)
	if err != nil {
		return provider.Inspection{}, err
	}
	threads, err := c.Threads(ctx, repo, number)
	if err != nil {
		return provider.Inspection{}, err
	}
	checks, err := c.Checks(ctx, repo, pull.HeadSHA)
	if err != nil {
		return provider.Inspection{}, err
	}
	return provider.Inspection{
		Repository:  repo,
		PullRequest: pull,
		Reviews:     reviews,
		Threads:     threads,
		Checks:      checks,
	}, nil
}

func (c *Client) Edit(ctx context.Context, repo provider.Repository, number int, input provider.EditInput) (provider.PullRequest, error) {
	payload := map[string]interface{}{}
	if input.Title != nil {
		payload["title"] = *input.Title
	}
	if input.Body != nil {
		payload["body"] = *input.Body
	}
	if input.State != nil {
		payload["state"] = *input.State
	}
	var pull apiPull
	if err := c.rest(ctx, http.MethodPatch, repoPath(repo)+"/pulls/"+strconv.Itoa(number), payload, &pull); err != nil {
		return provider.PullRequest{}, err
	}
	return mapPull(pull), nil
}

func (c *Client) Merge(ctx context.Context, repo provider.Repository, number int, input provider.MergeInput) (provider.MergeResult, error) {
	payload := map[string]string{"merge_method": input.Method}
	if input.Title != "" {
		payload["commit_title"] = input.Title
	}
	if input.Message != "" {
		payload["commit_message"] = input.Message
	}
	if input.ExpectedSHA != "" {
		payload["sha"] = input.ExpectedSHA
	}
	var result struct {
		SHA     string `json:"sha"`
		Merged  bool   `json:"merged"`
		Message string `json:"message"`
	}
	if err := c.rest(ctx, http.MethodPut, repoPath(repo)+"/pulls/"+strconv.Itoa(number)+"/merge", payload, &result); err != nil {
		return provider.MergeResult{}, err
	}
	return provider.MergeResult{SHA: result.SHA, Merged: result.Merged, Message: result.Message}, nil
}

func (c *Client) Reviews(ctx context.Context, repo provider.Repository, number int) ([]provider.Review, error) {
	type apiReview struct {
		ID          int64   `json:"id"`
		NodeID      string  `json:"node_id"`
		User        apiUser `json:"user"`
		Body        string  `json:"body"`
		State       string  `json:"state"`
		CommitID    string  `json:"commit_id"`
		SubmittedAt string  `json:"submitted_at"`
	}
	reviews := []provider.Review{}
	for page := 1; ; page++ {
		var response []apiReview
		requestPath := repoPath(repo) + "/pulls/" + strconv.Itoa(number) + "/reviews?per_page=100&page=" + strconv.Itoa(page)
		if err := c.rest(ctx, http.MethodGet, requestPath, nil, &response); err != nil {
			return nil, err
		}
		for _, review := range response {
			id := review.NodeID
			if id == "" {
				id = strconv.FormatInt(review.ID, 10)
			}
			reviews = append(reviews, provider.Review{
				ID:          id,
				Author:      provider.User{Login: review.User.Login, Name: review.User.Name},
				State:       normalizeState(review.State),
				Body:        review.Body,
				SubmittedAt: parseTime(review.SubmittedAt),
				CommitID:    review.CommitID,
			})
		}
		if len(response) < 100 {
			break
		}
	}
	return reviews, nil
}

func mapPull(pull apiPull) provider.PullRequest {
	state := pull.State
	if pull.MergedAt != nil {
		state = "merged"
	}
	return provider.PullRequest{
		Number:       pull.Number,
		ID:           pull.NodeID,
		Title:        pull.Title,
		Body:         pull.Body,
		State:        strings.ToLower(state),
		Draft:        pull.Draft,
		URL:          pull.HTMLURL,
		Author:       provider.User{Login: pull.User.Login, Name: pull.User.Name},
		BaseBranch:   pull.Base.Ref,
		HeadBranch:   pull.Head.Ref,
		HeadSHA:      pull.Head.SHA,
		Mergeable:    pull.Mergeable,
		MergeState:   pull.MergeableState,
		CreatedAt:    pull.CreatedAt,
		UpdatedAt:    pull.UpdatedAt,
		MergedAt:     pull.MergedAt,
		ChangedFiles: pull.ChangedFiles,
		Additions:    pull.Additions,
		Deletions:    pull.Deletions,
	}
}
