package result

import (
	"time"

	"github.com/pkg/errors"
)

func (lg *Log) RunNewGroup(name string, f func(sg *StepGroup) error) error {
	sg := lg.AddStepGroup(name)

	startedAt := time.Now()
	err := f(sg)
	sg.Duration = time.Since(startedAt)

	if err != nil {
		sg.LastStep().AddError(err.Error())
		return errors.Wrapf(err, "%s failed", name)
	}

	return nil
}

func (lg *Log) RunNewGroupVoid(name string, f func(sg *StepGroup)) {
	_ = lg.RunNewGroup(name, func(sg *StepGroup) error {
		f(sg)
		return nil
	})
}
