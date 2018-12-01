package processors

import (
	"strconv"
	"time"
)

type JSONDuration time.Duration

func (d JSONDuration) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(int(time.Duration(d) / time.Millisecond))), nil
}

func (d JSONDuration) String() string {
	return time.Duration(d).String()
}

type Timing struct {
	Name     string
	Duration JSONDuration `json:"DurationMs"`
}

type Warning struct {
	Tag  string
	Text string
}

type resultCollector struct {
	timings  []Timing
	warnings []Warning
}

func (r *resultCollector) trackTiming(name string, f func()) {
	startedAt := time.Now()
	f()
	r.timings = append(r.timings, Timing{
		Name:     name,
		Duration: JSONDuration(time.Since(startedAt)),
	})
}

func (r *resultCollector) addTimingFrom(name string, from time.Time) {
	r.timings = append(r.timings, Timing{
		Name:     name,
		Duration: JSONDuration(time.Since(from)),
	})
}

func (r *resultCollector) publicWarn(tag string, text string) {
	r.warnings = append(r.warnings, Warning{
		Tag:  tag,
		Text: text,
	})
}

type workerRes struct {
	Timings  []Timing  `json:",omitempty"`
	Warnings []Warning `json:",omitempty"`
	Error    string    `json:",omitempty"`
}

type resultJSON struct {
	Version         int
	GolangciLintRes interface{}
	WorkerRes       workerRes
}

func fromDBTime(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.Local)
}
