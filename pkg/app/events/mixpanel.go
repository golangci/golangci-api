package events

import (
	"os"
	"sync"

	"github.com/dukex/mixpanel"
)

var mixpanelClient mixpanel.Mixpanel
var mixpanelClientOnce sync.Once

func GetMixpanelClient() mixpanel.Mixpanel {
	mixpanelClientOnce.Do(func() {
		apiKey := os.Getenv("MIXPANEL_API_KEY")
		mixpanelClient = mixpanel.New(apiKey, "")
	})

	return mixpanelClient
}
