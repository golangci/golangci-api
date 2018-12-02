package models

import (
	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in payment_gateway_event.go

// gen:qs
type PaymentGatewayEvent struct {
	gorm.Model

	Provider   string
	ProviderID string

	UserID *uint

	Type string
	Data []byte
}
