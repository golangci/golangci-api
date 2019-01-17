package consumers

import (
	"context"
)

type ResultLogger func(err error)

type Consumer interface {
	ConsumeMessage(ctx context.Context, message []byte) error
	ResultLogger() ResultLogger
}
