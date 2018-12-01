package analytics

import (
	"os"
	"sync"

	"github.com/golangci/golangci-api/pkg/worker/lib/runmode"
	"github.com/savaki/amplitude-go"
)

var amplitudeClient *amplitude.Client
var amplitudeClientOnce sync.Once

func getAmplitudeClient() *amplitude.Client {
	amplitudeClientOnce.Do(func() {
		if runmode.IsProduction() {
			apiKey := os.Getenv("AMPLITUDE_API_KEY")
			amplitudeClient = amplitude.New(apiKey)
		}
	})

	return amplitudeClient
}
