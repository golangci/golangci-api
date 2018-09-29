package hooks

import "github.com/golangci/golangci-api/pkg/providers/provider"

type afterProviderCreateFunc func(p provider.Provider) error

type Injector struct {
	afterProviderCreate []afterProviderCreateFunc
}

func (hi *Injector) AddAfterProviderCreate(hook afterProviderCreateFunc) {
	hi.afterProviderCreate = append(hi.afterProviderCreate, hook)
}

func (hi Injector) RunAfterProviderCreate(p provider.Provider) error {
	for _, hook := range hi.afterProviderCreate {
		if err := hook(p); err != nil {
			return err
		}
	}
	return nil
}
