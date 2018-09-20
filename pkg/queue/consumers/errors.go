package consumers

import "errors"

var ErrRetryLater = errors.New("retry later")
var ErrBadMessage = errors.New("bad message")
