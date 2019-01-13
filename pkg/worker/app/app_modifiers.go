package app

import (
	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"
)

type Modifier func(a *App)

func SetPullProcessorFactory(pf processors.PullProcessorFactory) Modifier {
	return func(a *App) {
		a.ppf = pf
	}
}
