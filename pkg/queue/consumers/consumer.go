package consumers

import (
	"context"
)

type Consumer interface {
	ConsumeMessage(ctx context.Context, message []byte) error
}
