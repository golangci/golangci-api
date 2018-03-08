package analytics

import (
	"os"
	"sync"

	"github.com/dukex/mixpanel"
)

var mixpanelClient mixpanel.Mixpanel
var mixpanelClientOnce sync.Once

func getMixpanelClient() mixpanel.Mixpanel {
	mixpanelClientOnce.Do(func() {
		if os.Getenv("GO_ENV") == "prod" {
			apiKey := os.Getenv("MIXPANEL_API_KEY")
			mixpanelClient = mixpanel.New(apiKey, "")
		}
	})

	return mixpanelClient
}
