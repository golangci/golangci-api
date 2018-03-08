package analytics

import (
	"context"
	"strconv"
	"time"

	"github.com/dukex/mixpanel"
	"github.com/golangci/golangci-api/app/models"
	"github.com/savaki/amplitude-go"
)

type Tracker interface {
	UserRegistered(user *models.User)
}

type amplitudeMixpanelTracker struct{}

func (t amplitudeMixpanelTracker) UserRegistered(user *models.User) {
	userIDString := strconv.Itoa(int(user.ID))
	eventProps := map[string]interface{}{
		"provider": "github",
	}
	userProps := map[string]interface{}{
		"registeredAt": time.Now(),
	}
	const eventName = "registered"

	ac := getAmplitudeClient()
	if ac != nil {
		ac.Publish(amplitude.Event{
			UserId:          userIDString,
			EventType:       eventName,
			EventProperties: eventProps,
			UserProperties:  userProps,
		})
	}

	mp := getMixpanelClient()
	if mp != nil {
		const ip = "0" // don't auto-detect
		mp.Track(userIDString, eventName, &mixpanel.Event{
			IP:         ip,
			Properties: eventProps,
		})
		mp.Update(userIDString, &mixpanel.Update{
			IP:         ip,
			Operation:  "$set_once", // Works just like "$set", except it will not overwrite existing property values. This is useful for properties like "First login date".
			Properties: userProps,
		})
	}
}

func GetTracker(_ context.Context) Tracker {
	return amplitudeMixpanelTracker{}
}
