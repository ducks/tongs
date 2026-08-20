package repository

import (
	"reflect"
	"testing"

	"github.com/ducks/tongs/internal/provider"
)

func TestParseRemote(t *testing.T) {
	tests := []struct {
		name   string
		remote string
		want   provider.Repository
	}{
		{
			name:   "GitHub SSH",
			remote: "git@github.com:discourse/discourse.git",
			want:   provider.Repository{Provider: "github", Host: "github.com", Owner: "discourse", Name: "discourse"},
		},
		{
			name:   "GitLab subgroup",
			remote: "https://gitlab.com/acme/platform/service.git",
			want:   provider.Repository{Provider: "gitlab", Host: "gitlab.com", Owner: "acme/platform", Name: "service"},
		},
		{
			name:   "Codeberg",
			remote: "ssh://git@codeberg.org/ducks/tongs.git",
			want:   provider.Repository{Provider: "gitea", Host: "codeberg.org", Owner: "ducks", Name: "tongs"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseRemote(test.remote)
			if err != nil {
				t.Fatalf("parseRemote returned an error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parseRemote = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestParsePullRequestURL(t *testing.T) {
	tests := []struct {
		name             string
		url              string
		providerOverride string
		wantRepo         provider.Repository
		wantNumber       int
	}{
		{
			name:       "GitHub",
			url:        "https://github.com/discourse/discourse/pull/123/files",
			wantRepo:   provider.Repository{Provider: "github", Host: "github.com", Owner: "discourse", Name: "discourse"},
			wantNumber: 123,
		},
		{
			name:       "GitLab subgroup",
			url:        "https://gitlab.com/acme/platform/service/-/merge_requests/42",
			wantRepo:   provider.Repository{Provider: "gitlab", Host: "gitlab.com", Owner: "acme/platform", Name: "service"},
			wantNumber: 42,
		},
		{
			name:             "GitHub Enterprise override",
			url:              "https://github.example.com/acme/service/pull/7",
			providerOverride: "github",
			wantRepo:         provider.Repository{Provider: "github", Host: "github.example.com", Owner: "acme", Name: "service"},
			wantNumber:       7,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo, number, err := parsePullRequestURL(test.url, test.providerOverride)
			if err != nil {
				t.Fatalf("parsePullRequestURL returned an error: %v", err)
			}
			if !reflect.DeepEqual(repo, test.wantRepo) || number != test.wantNumber {
				t.Fatalf("parsePullRequestURL = %#v, %d; want %#v, %d", repo, number, test.wantRepo, test.wantNumber)
			}
		})
	}
}

func TestUnknownHostRequiresProvider(t *testing.T) {
	_, err := Resolve(Options{Repo: "git.example.com/acme/service"}, "1")
	if err == nil {
		t.Fatal("Resolve unexpectedly succeeded")
	}
}
