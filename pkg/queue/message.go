package queue

type Message interface {
	DeduplicationID() string
}
