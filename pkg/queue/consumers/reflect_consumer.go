package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"gopkg.in/redsync.v1"

	"github.com/golangci/golangci-api/pkg/queue"
	"github.com/pkg/errors"
)

type ReflectConsumer struct {
	handler         interface{}
	timeout         time.Duration
	distlockFactory *redsync.Redsync
}

func NewReflectConsumer(handler interface{}, timeout time.Duration, df *redsync.Redsync) (*ReflectConsumer, error) {
	handlerType := reflect.TypeOf(handler)
	if handlerType.Kind() != reflect.Func {
		return nil, fmt.Errorf("handler kind %s is not %s", handlerType.Kind(), reflect.Func)
	}

	if handlerType.NumIn() != 2 {
		return nil, fmt.Errorf("args count %d must be two", handlerType.NumIn())
	}

	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	firstArgType := handlerType.In(0)
	if !firstArgType.Implements(contextType) {
		return nil, fmt.Errorf("handler's first arg is not Context, it's %s", firstArgType.Kind())
	}

	secondArgType := handlerType.In(1)
	if secondArgType.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("handler's second arg is not pointer, it's %s", secondArgType.Kind())
	}
	secondArgPointedType := secondArgType.Elem()
	if secondArgPointedType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("handler's second arg's pointer points no to struct but to %s", secondArgPointedType.Kind())
	}
	messageType := reflect.TypeOf((*queue.Message)(nil)).Elem()
	if !secondArgType.Implements(messageType) {
		return nil, fmt.Errorf("handler's second arg doesn't implement queue.Message interface")
	}

	if handlerType.NumOut() != 1 {
		return nil, fmt.Errorf("invalid output values count %d != 1", handlerType.NumOut())
	}
	retType := handlerType.Out(0)
	err := errors.New("")
	errorType := reflect.TypeOf(&err).Elem()
	if !retType.Implements(errorType) {
		return nil, fmt.Errorf("return type is not error, it's %s", retType.Kind())
	}

	return &ReflectConsumer{
		handler:         handler,
		timeout:         timeout,
		distlockFactory: df,
	}, nil
}

func (c ReflectConsumer) ConsumeMessage(ctx context.Context, message []byte) error {
	handlerType := reflect.TypeOf(c.handler)
	secondArgPointedType := handlerType.In(1).Elem()
	callArgValue := reflect.New(secondArgPointedType)
	callArg := callArgValue.Interface()

	if err := json.Unmarshal(message, callArg); err != nil {
		return errors.Wrap(errors.Wrap(ErrBadMessage, err.Error()), "json unmarshal failed")
	}

	handler := reflect.ValueOf(c.handler)
	return c.runHandler(ctx, handler, callArgValue)
}

func (c ReflectConsumer) runHandler(ctx context.Context, handler, callArgValue reflect.Value) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	callArgMessage := callArgValue.Interface().(queue.Message)
	lockID := fmt.Sprintf("locks/consumers/%s", callArgMessage.DeduplicationID())
	distLock := c.distlockFactory.NewMutex(lockID, redsync.SetExpiry(c.timeout))
	if err := distLock.Lock(); err != nil {
		return errors.Wrapf(err, "failed to acquire distributed lock %s", lockID)
	}
	defer distLock.Unlock()

	retValues := handler.Call([]reflect.Value{reflect.ValueOf(ctx), callArgValue})
	retVal := retValues[0]

	if retVal.Interface() != nil {
		return retVal.Interface().(error)
	}

	return nil
}
