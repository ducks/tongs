package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ducks/tongs/internal/provider"
)

func testClient(server *httptest.Server) *Client {
	return &Client{
		host:       "github.com",
		token:      "test-token",
		apiURL:     server.URL,
		graphqlURL: server.URL + "/graphql",
		http:       server.Client(),
	}
}

func TestGetPullRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/acme/widget/pulls/12" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("authorization header is missing")
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
          "number": 12,
          "node_id": "PR_node",
          "title": "Useful change",
          "body": "Details",
          "state": "open",
          "draft": false,
          "html_url": "https://github.com/acme/widget/pull/12",
          "user": {"login": "ducks"},
          "base": {"ref": "main"},
          "head": {"ref": "feature", "sha": "abc123"},
          "created_at": "2026-08-20T00:00:00Z",
          "updated_at": "2026-08-20T01:00:00Z"
        }`))
	}))
	defer server.Close()

	pull, err := testClient(server).GetPullRequest(context.Background(), provider.Repository{Owner: "acme", Name: "widget"}, 12)
	if err != nil {
		t.Fatalf("GetPullRequest returned an error: %v", err)
	}
	if pull.Number != 12 || pull.HeadSHA != "abc123" || pull.Author.Login != "ducks" {
		t.Fatalf("unexpected pull request: %#v", pull)
	}
}

func TestCreatePullRequestUsesRepositoryDefaultBranch(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/repos/acme/widget":
			_, _ = response.Write([]byte(`{"default_branch":"trunk"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/repos/acme/widget/pulls":
			var payload map[string]interface{}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload["title"] != "Add feature" || payload["head"] != "feature" || payload["base"] != "trunk" || payload["draft"] != true {
				t.Fatalf("unexpected payload: %#v", payload)
			}
			_, _ = response.Write([]byte(`{
            "number": 13,
            "node_id": "PR_created",
            "title": "Add feature",
            "state": "open",
            "draft": true,
            "html_url": "https://github.com/acme/widget/pull/13",
            "user": {"login": "ducks"},
            "base": {"ref": "trunk"},
            "head": {"ref": "feature", "sha": "def456"},
            "created_at": "2026-08-20T00:00:00Z",
            "updated_at": "2026-08-20T00:00:00Z"
          }`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	pull, err := testClient(server).Create(context.Background(), provider.Repository{Owner: "acme", Name: "widget"}, provider.CreateInput{
		Title: "Add feature", HeadBranch: "feature", Draft: true,
	})
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if requests != 2 || pull.Number != 13 || pull.BaseBranch != "trunk" || !pull.Draft {
		t.Fatalf("unexpected pull request: %#v", pull)
	}
}

func TestApprovePullRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/repos/acme/widget/pulls/12/reviews" {
			http.NotFound(response, request)
			return
		}
		var payload map[string]string
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["event"] != "APPROVE" || payload["body"] != "Looks good" || payload["commit_id"] != "abc123" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
          "id": 15,
          "node_id": "PRR_approved",
          "user": {"login": "ducks"},
          "body": "Looks good",
          "state": "APPROVED",
          "commit_id": "abc123",
          "submitted_at": "2026-08-20T00:00:00Z"
        }`))
	}))
	defer server.Close()

	review, err := testClient(server).Approve(context.Background(), provider.Repository{Owner: "acme", Name: "widget"}, 12, provider.ApprovalInput{
		Body: "Looks good", ExpectedSHA: "abc123",
	})
	if err != nil {
		t.Fatalf("Approve returned an error: %v", err)
	}
	if review.State != "approved" || review.CommitID != "abc123" || review.Author.Login != "ducks" {
		t.Fatalf("unexpected review: %#v", review)
	}
}

func TestChecksCombinesCheckRunsAndStatuses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/repos/acme/widget/commits/abc/check-runs":
			_, _ = response.Write([]byte(`{"check_runs":[{"name":"test","status":"completed","conclusion":"success"}]}`))
		case "/repos/acme/widget/commits/abc/status":
			_, _ = response.Write([]byte(`{"statuses":[{"context":"lint","state":"failure","description":"bad format"}]}`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	checks, err := testClient(server).Checks(context.Background(), provider.Repository{Owner: "acme", Name: "widget"}, "abc")
	if err != nil {
		t.Fatalf("Checks returned an error: %v", err)
	}
	if checks.Overall != "failure" || len(checks.Checks) != 1 || len(checks.Statuses) != 1 {
		t.Fatalf("unexpected checks result: %#v", checks)
	}
}

func TestThreadsMapsGraphQLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/graphql" {
			http.NotFound(response, request)
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[{"id":"thread-1","isResolved":false,"isOutdated":false,"path":"main.go","line":7,"comments":{"nodes":[{"id":"comment-1","databaseId":9,"body":"Please change this","path":"main.go","line":7,"url":"https://example.test/comment","createdAt":"2026-08-20T00:00:00Z","author":{"login":"reviewer"}}]}}]}}}}}`))
	}))
	defer server.Close()

	threads, err := testClient(server).Threads(context.Background(), provider.Repository{Owner: "acme", Name: "widget"}, 12)
	if err != nil {
		t.Fatalf("Threads returned an error: %v", err)
	}
	if len(threads) != 1 || threads[0].ID != "thread-1" || threads[0].Comments[0].Author.Login != "reviewer" {
		t.Fatalf("unexpected threads: %#v", threads)
	}
}

func TestRESTErrorIsStructured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNotFound)
		_, _ = response.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer server.Close()

	_, err := testClient(server).GetPullRequest(context.Background(), provider.Repository{Owner: "acme", Name: "widget"}, 99)
	apiError, ok := err.(*provider.Error)
	if !ok || apiError.Code != "not_found" || apiError.StatusCode != 404 {
		t.Fatalf("unexpected error: %#v", err)
	}
}
