package provider

import "errors"

var (
	ErrUnauthorized = errors.New("no provider authorization")
	ErrNotFound     = errors.New("not found in provider")
)
