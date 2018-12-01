package analytics

import (
	"os"
	"sync"

	"github.com/dukex/mixpanel"
	"github.com/golangci/golangci-api/pkg/worker/lib/runmode"
)

var mixpanelClient mixpanel.Mixpanel
var mixpanelClientOnce sync.Once

func getMixpanelClient() mixpanel.Mixpanel {
	mixpanelClientOnce.Do(func() {
		if runmode.IsProduction() {
			apiKey := os.Getenv("MIXPANEL_API_KEY")
			mixpanelClient = mixpanel.New(apiKey, "")
		}
	})

	return mixpanelClient
}
