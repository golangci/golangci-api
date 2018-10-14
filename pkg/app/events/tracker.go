package events

import (
	"context"
	"strconv"

	"github.com/dukex/mixpanel"
	"github.com/savaki/amplitude-go"
	"github.com/sirupsen/logrus"
)

func NewAuthenticatedTracker(userID int) AuthenticatedTracker {
	return AuthenticatedTracker{
		userID: userID,
	}
}

type AuthenticatedTracker struct {
	userID    int
	userProps map[string]interface{}
}

func (t AuthenticatedTracker) WithUserProps(props map[string]interface{}) AuthenticatedTracker {
	tc := t
	tc.userProps = props
	return tc
}

func (t AuthenticatedTracker) Track(ctx context.Context, eventName string, props map[string]interface{}) {
	eventProps := map[string]interface{}{}
	for k, v := range props {
		eventProps[k] = v
	}

	logrus.Infof("track event %s with props %+v", eventName, eventProps)

	userIDString := strconv.Itoa(t.userID)
	ac := GetAmplitudeClient()
	if ac != nil {
		ev := amplitude.Event{
			UserId:          userIDString,
			EventType:       eventName,
			EventProperties: eventProps,
			UserProperties:  t.userProps,
		}
		if err := ac.Publish(ev); err != nil {
			logrus.Warnf("Can't publish %+v to amplitude: %s", ev, err)
		}
	}

	mp := GetMixpanelClient()
	if mp != nil {
		const ip = "0" // don't auto-detect
		ev := &mixpanel.Event{
			IP:         ip,
			Properties: eventProps,
		}
		if err := mp.Track(userIDString, eventName, ev); err != nil {
			logrus.Warnf("Can't publish event %s (%+v) to mixpanel: %s", eventName, ev, err)
		}
	}
}
