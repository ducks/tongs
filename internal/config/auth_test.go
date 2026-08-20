package config

import "testing"

func TestTokenUsesProviderSpecificEnvironment(t *testing.T) {
	t.Setenv("PR_TOKEN", "")
	t.Setenv("PR_GITHUB_TOKEN", "provider-token")
	t.Setenv("GH_TOKEN", "fallback-token")

	token, err := Token("github", "github.com")
	if err != nil {
		t.Fatalf("Token returned an error: %v", err)
	}
	if token != "provider-token" {
		t.Fatalf("Token = %q, want provider-token", token)
	}
}

func TestTokenUsesGenericEnvironmentFirst(t *testing.T) {
	t.Setenv("PR_TOKEN", "generic-token")
	t.Setenv("PR_GITHUB_TOKEN", "provider-token")

	token, err := Token("github", "github.com")
	if err != nil {
		t.Fatalf("Token returned an error: %v", err)
	}
	if token != "generic-token" {
		t.Fatalf("Token = %q, want generic-token", token)
	}
}
