package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionReturnsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"--version"}, strings.NewReader(""), &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !response.OK || response.Data.Version != version {
		t.Fatalf("unexpected response: %s", stdout.String())
	}
}

func TestHelpIsSuccessfulAndStructured(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"schema_version":"1"`) || !strings.Contains(stdout.String(), `"usage"`) {
		t.Fatalf("unexpected response: %s", stdout.String())
	}
}

func TestUsageErrorsAreStructured(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"wat"}, strings.NewReader(""), &stdout, &stderr)
	if exit != 2 || stdout.Len() != 0 {
		t.Fatalf("exit = %d, stdout = %s", exit, stdout.String())
	}
	if !strings.Contains(stderr.String(), `"code":"usage_error"`) {
		t.Fatalf("unexpected error: %s", stderr.String())
	}
}

func TestReadValueFromStdin(t *testing.T) {
	value, err := readValue("", "-", strings.NewReader("body from stdin\n"))
	if err != nil {
		t.Fatalf("readValue returned an error: %v", err)
	}
	if value != "body from stdin" {
		t.Fatalf("value = %q", value)
	}
}
