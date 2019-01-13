package result

import (
	"fmt"
	"log"
	"strings"
	"time"

	goenvconfig "github.com/golangci/golangci-api/pkg/goenvbuild/config"
)

type Step struct {
	Description string
	OutputLines []string `json:",omitempty"`
	Error       string   `json:",omitempty"`

	logger *log.Logger
}

func (s *Step) AddOutputLine(format string, args ...interface{}) {
	line := fmt.Sprintf(format, args...)

	if s.logger != nil {
		s.logger.Printf("    %s\n", line)
	}

	s.OutputLines = append(s.OutputLines, line)
}

func (s *Step) AddError(err string) {
	if s.logger != nil {
		s.logger.Printf("    Error: %s\n", err)
	}

	s.Error = err
}

func (s *Step) AddOutput(output string) {
	if output == "" {
		return
	}

	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			continue // don't add trailing new line
		}
		s.AddOutputLine(line)
	}
}

type StepGroup struct {
	Name     string
	Steps    []*Step
	Duration time.Duration

	logger *log.Logger
}

func (sg *StepGroup) LastStep() *Step {
	return sg.Steps[len(sg.Steps)-1]
}

func (sg *StepGroup) AddStep(desc string) *Step {
	if sg.logger != nil {
		sg.logger.Printf("  [STEP] %s\n", desc)
	}

	step := &Step{
		Description: desc,
		logger:      sg.logger,
	}
	sg.Steps = append(sg.Steps, step)
	return step
}

func (sg *StepGroup) AddStepCmd(cmd string, args ...string) *Step {
	return sg.AddStep(fmt.Sprintf("$ %s %s", cmd, strings.Join(args, " ")))
}

func (sg *StepGroup) Finish() {
	if sg.logger != nil {
		sg.logger.Println()
	}
}

type Log struct {
	Groups []*StepGroup

	logger *log.Logger
}

func NewLog(logger *log.Logger) *Log {
	return &Log{
		logger: logger,
	}
}

func (lg *Log) AddStepGroup(name string) *StepGroup {
	sg := &StepGroup{
		Name:   name,
		logger: lg.logger,
	}
	lg.Groups = append(lg.Groups, sg)

	if lg.logger != nil {
		lg.logger.Printf("[GROUP] %s\n", name)
	}

	return sg
}

func (lg *Log) LastStepGroup() *StepGroup {
	return lg.Groups[len(lg.Groups)-1]
}

type Result struct {
	ServiceConfig goenvconfig.Service

	WorkDir             string
	Environment         map[string]string
	GolangciLintVersion string

	Log   *Log
	Error string `json:",omitempty"`
}

func (res Result) Finish() {
	logger := res.Log.logger
	if logger == nil {
		return
	}

	if res.Error == "" {
		logger.Println("Success!")
	} else {
		logger.Println()
		logger.Println(res.Error)
	}
}
