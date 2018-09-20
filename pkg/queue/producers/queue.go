package producers

type Message interface {
	DeduplicationID() string
}

type Queue interface {
	Put(message Message) error
}
