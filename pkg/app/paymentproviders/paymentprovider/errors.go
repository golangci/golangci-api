package paymentprovider

import "errors"

var (
	ErrNotFound         = errors.New("not found in provider")
	ErrInvalidCardToken = errors.New("invalid card token")
)
