package llm

import (
	"errors"
	"fmt"
)

// LLM error types for better error handling and categorization
var (
	// ErrNoAPIKey indicates the required API key is not configured
	ErrNoAPIKey = errors.New("API key not configured")

	// ErrInvalidResponse indicates the API returned an unparseable response
	ErrInvalidResponse = errors.New("invalid response from API")

	// ErrRateLimited indicates the API rate limit was exceeded
	ErrRateLimited = errors.New("rate limit exceeded")

	// ErrContextCancelled indicates the request was cancelled
	ErrContextCancelled = errors.New("request cancelled")

	// ErrRequestFailed indicates a generic request failure
	ErrRequestFailed = errors.New("request failed")
)

// APIError represents an error returned by the LLM API
type APIError struct {
	Provider   string // gemini, openai, anthropic
	StatusCode int    // HTTP status code
	Message    string // Error message from API
	Err        error  // Underlying error
}

func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s API error (HTTP %d): %s", e.Provider, e.StatusCode, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s API error: %s: %v", e.Provider, e.Message, e.Err)
	}
	return fmt.Sprintf("%s API error: %s", e.Provider, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// IsRateLimited checks if the error indicates rate limiting
func (e *APIError) IsRateLimited() bool {
	return e.StatusCode == 429 || errors.Is(e.Err, ErrRateLimited)
}

// IsAuthError checks if the error indicates authentication failure
func (e *APIError) IsAuthError() bool {
	return e.StatusCode == 401 || e.StatusCode == 403
}

// IsTransient checks if the error is temporary and the request can be retried
func (e *APIError) IsTransient() bool {
	return e.StatusCode == 429 || e.StatusCode == 500 || e.StatusCode == 502 || e.StatusCode == 503 || e.StatusCode == 504
}

// NewAPIError creates a new APIError with the given parameters
func NewAPIError(provider string, statusCode int, message string, err error) *APIError {
	return &APIError{
		Provider:   provider,
		StatusCode: statusCode,
		Message:    message,
		Err:        err,
	}
}

// ParseError represents a JSON parsing error
type ParseError struct {
	Provider string
	Stage    string // e.g., "response parsing", "plan extraction"
	Input    string // truncated input that failed to parse
	Err      error
}

func (e *ParseError) Error() string {
	if e.Input != "" && len(e.Input) > 100 {
		e.Input = e.Input[:100] + "..."
	}
	return fmt.Sprintf("%s parse error during %s: %v", e.Provider, e.Stage, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// NewParseError creates a new ParseError
func NewParseError(provider, stage, input string, err error) *ParseError {
	return &ParseError{
		Provider: provider,
		Stage:    stage,
		Input:    input,
		Err:      err,
	}
}
