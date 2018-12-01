package processors

import "context"

type NopProcessor struct{}

func (p NopProcessor) Process(ctx context.Context) error {
	return nil
}
