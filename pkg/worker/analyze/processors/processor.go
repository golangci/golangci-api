package processors

import "context"

type Processor interface {
	Process(ctx context.Context) error
}
