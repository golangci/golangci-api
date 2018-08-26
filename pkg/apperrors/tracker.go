package apperrors

import "net/http"

type Level string

const (
	LevelError Level = "ERROR"
	LevelWarn  Level = "WARN"
)

type Tracker interface {
	Track(level Level, errorText string, ctx map[string]interface{})
	WithHTTPRequest(r *http.Request) Tracker
}
