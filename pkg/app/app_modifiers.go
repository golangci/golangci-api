package app

import (
	"github.com/golangci/golangci-api/pkg/app/providers"
)

type Modifier func(a *App)

func SetProviderFactory(pf providers.Factory) Modifier {
	return func(a *App) {
		a.providerFactory = pf
	}
}
