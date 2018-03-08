package analytics

import (
	"os"
	"sync"

	"github.com/savaki/amplitude-go"
)

var amplitudeClient *amplitude.Client
var amplitudeClientOnce sync.Once

func getAmplitudeClient() *amplitude.Client {
	amplitudeClientOnce.Do(func() {
		if os.Getenv("GO_ENV") == "prod" {
			apiKey := os.Getenv("AMPLITUDE_API_KEY")
			amplitudeClient = amplitude.New(apiKey)
		}
	})

	return amplitudeClient
}
