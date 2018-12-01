package apperrors

import "net/http"

type NopTracker struct{}

func NewNopTracker() *NopTracker {
	return &NopTracker{}
}

func (t NopTracker) Track(level Level, errorText string, ctx map[string]interface{}) {
}

func (t NopTracker) WithHTTPRequest(r *http.Request) Tracker {
	return t
}
