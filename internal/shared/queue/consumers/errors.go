package consumers

import "errors"

var ErrRetryLater = errors.New("retry later")
var ErrPermanent = errors.New("permanent error")
var ErrBadMessage = errors.New("bad message")
