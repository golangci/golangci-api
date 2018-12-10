package paymentprovider

type EventProcessor interface {
	Process(payload string, eventUUID string) error
}
