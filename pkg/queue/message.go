package queue

type Message interface {
	LockID() string
}
