package bunq

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// APIError is a non-2xx response from the bunq API.
type APIError struct {
	StatusCode int
	Messages   []string
	ResponseID string
}

func (e *APIError) Error() string {
	msg := strings.Join(e.Messages, "; ")
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	if e.ResponseID != "" {
		return fmt.Sprintf("bunq api: %d: %s (response id %s)", e.StatusCode, msg, e.ResponseID)
	}
	return fmt.Sprintf("bunq api: %d: %s", e.StatusCode, msg)
}

func newAPIError(resp *http.Response, raw []byte) *APIError {
	e := &APIError{StatusCode: resp.StatusCode, ResponseID: resp.Header.Get(headerResponseID)}
	var body struct {
		Error []struct {
			Description string `json:"error_description"`
		} `json:"Error"`
	}
	if json.Unmarshal(raw, &body) == nil {
		for _, item := range body.Error {
			e.Messages = append(e.Messages, item.Description)
		}
	}
	return e
}

var rateMessage = regexp.MustCompile(`maximum of (\d+) calls per (\d+) seconds?`)

const (
	defaultRateWindow = 3 * time.Second
	maxBackoff        = 60 * time.Second
)

// backoff returns the wait before retry number attempt (0-based) after a 429.
// It derives the base from the limit bunq reports, as the docs advise.
func backoff(e *APIError, attempt int) time.Duration {
	base := defaultRateWindow
	if m := rateMessage.FindStringSubmatch(strings.Join(e.Messages, " ")); m != nil {
		if calls, err := strconv.Atoi(m[1]); err == nil && calls > 0 {
			if secs, err := strconv.Atoi(m[2]); err == nil && secs > 0 {
				base = time.Duration(secs) * time.Second / time.Duration(calls)
			}
		}
	}
	d := base << attempt
	if d > maxBackoff || d <= 0 {
		return maxBackoff
	}
	return d
}
