package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
)

type ReflectConsumer struct {
	handler interface{}
}

func NewReflectConsumer(handler interface{}) (*ReflectConsumer, error) {
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
		handler: handler,
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
	retValues := handler.Call([]reflect.Value{reflect.ValueOf(ctx), callArgValue})
	retVal := retValues[0]
	var err error
	if retVal.Interface() != nil {
		err = retVal.Interface().(error)
		return errors.Wrap(ErrRetryLater, err.Error())
	}

	return nil
}
