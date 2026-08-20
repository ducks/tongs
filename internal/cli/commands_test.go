package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ducks/tongs/internal/provider"
)

type commandTestProvider struct {
	provider.Provider
	createInput  provider.CreateInput
	approveInput provider.ApprovalInput
	approvePR    int
	createCalls  int
	approveCalls int
}

func (p *commandTestProvider) Name() string { return "test" }

func (p *commandTestProvider) Create(_ context.Context, _ provider.Repository, input provider.CreateInput) (provider.PullRequest, error) {
	p.createCalls++
	p.createInput = input
	return provider.PullRequest{Number: 12, Title: input.Title, HeadBranch: input.HeadBranch, BaseBranch: input.BaseBranch}, nil
}

func (p *commandTestProvider) Approve(_ context.Context, _ provider.Repository, number int, input provider.ApprovalInput) (provider.Review, error) {
	p.approveCalls++
	p.approvePR = number
	p.approveInput = input
	return provider.Review{ID: "review-1", State: "approved", Body: input.Body, CommitID: input.ExpectedSHA}, nil
}

func commandTestApp(adapter *commandTestProvider, dryRun bool) (*app, *bytes.Buffer, *bytes.Buffer) {
	registry := provider.NewRegistry()
	registry.Register("test", func(string) (provider.Provider, error) { return adapter, nil })
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	return &app{
		ctx:      context.Background(),
		stdin:    strings.NewReader(""),
		out:      newOutput(stdout, stderr, false),
		registry: registry,
		opts: globalOptions{
			provider: "test",
			host:     "forge.test",
			repo:     "acme/widget",
			dryRun:   dryRun,
		},
	}, stdout, stderr
}

func TestCreateCommand(t *testing.T) {
	adapter := &commandTestProvider{}
	application, stdout, _ := commandTestApp(adapter, false)

	err := application.create([]string{
		"--title", "Add feature",
		"--body", "Details",
		"--head", "feature",
		"--base", "trunk",
		"--draft",
	})
	if err != nil {
		t.Fatalf("create returned an error: %v", err)
	}
	if adapter.createCalls != 1 {
		t.Fatalf("Create calls = %d, want 1", adapter.createCalls)
	}
	want := provider.CreateInput{Title: "Add feature", Body: "Details", HeadBranch: "feature", BaseBranch: "trunk", Draft: true}
	if adapter.createInput != want {
		t.Fatalf("Create input = %#v, want %#v", adapter.createInput, want)
	}
	assertSuccessfulCommandOutput(t, stdout)
}

func TestCreateCommandDryRunDoesNotMutate(t *testing.T) {
	adapter := &commandTestProvider{}
	application, stdout, _ := commandTestApp(adapter, true)

	if err := application.create([]string{"--title", "Preview", "--head", "feature"}); err != nil {
		t.Fatalf("create returned an error: %v", err)
	}
	if adapter.createCalls != 0 {
		t.Fatalf("Create calls = %d, want 0", adapter.createCalls)
	}
	var response struct {
		DryRun bool `json:"dry_run"`
		Data   struct {
			Action string `json:"action"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !response.DryRun || response.Data.Action != "create" {
		t.Fatalf("unexpected response: %s", stdout.String())
	}
}

func TestApproveCommand(t *testing.T) {
	adapter := &commandTestProvider{}
	application, stdout, _ := commandTestApp(adapter, false)

	if err := application.approve([]string{"--body", "Looks good", "--sha", "abc123", "42"}); err != nil {
		t.Fatalf("approve returned an error: %v", err)
	}
	if adapter.approveCalls != 1 || adapter.approvePR != 42 {
		t.Fatalf("Approve calls = %d, PR = %d; want 1, 42", adapter.approveCalls, adapter.approvePR)
	}
	want := provider.ApprovalInput{Body: "Looks good", ExpectedSHA: "abc123"}
	if adapter.approveInput != want {
		t.Fatalf("Approve input = %#v, want %#v", adapter.approveInput, want)
	}
	assertSuccessfulCommandOutput(t, stdout)
}

func TestApproveCommandDryRunDoesNotMutate(t *testing.T) {
	adapter := &commandTestProvider{}
	application, stdout, _ := commandTestApp(adapter, true)

	if err := application.approve([]string{"--sha", "abc123", "42"}); err != nil {
		t.Fatalf("approve returned an error: %v", err)
	}
	if adapter.approveCalls != 0 {
		t.Fatalf("Approve calls = %d, want 0", adapter.approveCalls)
	}
	var response struct {
		DryRun bool `json:"dry_run"`
		Data   struct {
			Action string `json:"action"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !response.DryRun || response.Data.Action != "approve" {
		t.Fatalf("unexpected response: %s", stdout.String())
	}
}

func assertSuccessfulCommandOutput(t *testing.T, output *bytes.Buffer) {
	t.Helper()
	var response struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !response.OK {
		t.Fatalf("unexpected response: %s", output.String())
	}
}
