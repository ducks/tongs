package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func Token(providerName, host string) (string, error) {
	names := []string{"PR_TOKEN"}
	switch providerName {
	case "github":
		names = append(names, "PR_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN")
	case "gitlab":
		names = append(names, "PR_GITLAB_TOKEN", "GITLAB_TOKEN")
	case "gitea":
		names = append(names, "PR_GITEA_TOKEN", "GITEA_TOKEN")
	default:
		return "", fmt.Errorf("unknown provider %q", providerName)
	}

	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value, nil
		}
	}
	if token := gitCredential(host); token != "" {
		return token, nil
	}
	if providerName == "github" {
		if token := githubCLIToken(host); token != "" {
			return token, nil
		}
	}
	return "", fmt.Errorf("no %s token found in %v or the git credential helper for %s", providerName, names, host)
}

func gitCredential(host string) string {
	command := exec.Command("git", "credential", "fill")
	command.Stdin = strings.NewReader("protocol=https\nhost=" + host + "\n\n")
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return ""
	}
	for _, line := range strings.Split(output.String(), "\n") {
		if value, found := strings.CutPrefix(line, "password="); found {
			return value
		}
	}
	return ""
}

func githubCLIToken(host string) string {
	command := exec.Command("gh", "auth", "token", "--hostname", host)
	command.Stderr = io.Discard
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
