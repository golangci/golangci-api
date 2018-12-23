package policy

import "github.com/golangci/golangci-api/internal/api/apierrors"

var ErrNotOrgAdmin = apierrors.NewNotAcceptableError("NOT_ORG_ADMIN")
var ErrNoActiveSubscription = apierrors.NewNotAcceptableError("NOT_ACTIVE_SUBSCRIPTION")
var ErrNoSeatInSubscription = apierrors.NewNotAcceptableError("NOT_SEAT_IN_SUBSCRIPTION")
