package analytics

import (
	"context"

	"github.com/dukex/mixpanel"
	"github.com/savaki/amplitude-go"
	log "github.com/sirupsen/logrus"
)

type EventName string

const EventPRChecked EventName = "PR checked"
const EventRepoAnalyzed EventName = "Repo analyzed"

type Tracker interface {
	Track(ctx context.Context, event EventName)
}

type amplitudeMixpanelTracker struct{}

func (t amplitudeMixpanelTracker) Track(ctx context.Context, eventName EventName) {
	trackingProps := getTrackingProps(ctx)
	userID := trackingProps["userIDString"].(string)

	eventProps := map[string]interface{}{}
	for k, v := range trackingProps {
		if k != "userIDString" {
			eventProps[k] = v
		}
	}

	addedEventProps := ctx.Value(eventName).(map[string]interface{})
	for k, v := range addedEventProps {
		eventProps[k] = v
	}
	log.Infof("track event %s with props %+v", eventName, eventProps)

	ac := getAmplitudeClient()
	if ac != nil {
		ev := amplitude.Event{
			UserId:          userID,
			EventType:       string(eventName),
			EventProperties: eventProps,
		}
		if err := ac.Publish(ev); err != nil {
			Log(ctx).Warnf("Can't publish %+v to amplitude: %s", ev, err)
		}
	}

	mp := getMixpanelClient()
	if mp != nil {
		const ip = "0" // don't auto-detect
		ev := &mixpanel.Event{
			IP:         ip,
			Properties: eventProps,
		}
		if err := mp.Track(userID, string(eventName), ev); err != nil {
			Log(ctx).Warnf("Can't publish event %s (%+v) to mixpanel: %s", string(eventName), ev, err)
		}
	}
}
