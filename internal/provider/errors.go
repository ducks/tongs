package provider

import "fmt"

type Error struct {
	Code       string      `json:"code"`
	Message    string      `json:"message"`
	StatusCode int         `json:"status_code,omitempty"`
	Details    interface{} `json:"details,omitempty"`
}

func (e *Error) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s (HTTP %d)", e.Message, e.StatusCode)
	}
	return e.Message
}
