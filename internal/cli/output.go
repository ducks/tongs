package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ducks/tongs/internal/provider"
)

const schemaVersion = "1"

type successEnvelope struct {
	SchemaVersion string      `json:"schema_version"`
	OK            bool        `json:"ok"`
	DryRun        bool        `json:"dry_run,omitempty"`
	Data          interface{} `json:"data"`
}

type errorEnvelope struct {
	SchemaVersion string          `json:"schema_version"`
	OK            bool            `json:"ok"`
	Error         *provider.Error `json:"error"`
}

type output struct {
	stdout io.Writer
	stderr io.Writer
	pretty bool
}

func (o output) success(data interface{}, dryRun bool) error {
	return o.encode(o.stdout, successEnvelope{SchemaVersion: schemaVersion, OK: true, DryRun: dryRun, Data: data})
}

func (o output) failure(err error) {
	var structured *provider.Error
	if !errors.As(err, &structured) {
		structured = &provider.Error{Code: "internal_error", Message: err.Error()}
	}
	_ = o.encode(o.stderr, errorEnvelope{SchemaVersion: schemaVersion, OK: false, Error: structured})
}

func (o output) usage(message string) {
	o.failure(&provider.Error{Code: "usage_error", Message: message})
}

func (o output) encode(writer io.Writer, value interface{}) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	if o.pretty {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return nil
}
