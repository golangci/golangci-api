package events

import (
	"os"
	"sync"

	"github.com/savaki/amplitude-go"
)

var amplitudeClient *amplitude.Client
var amplitudeClientOnce sync.Once

func GetAmplitudeClient() *amplitude.Client {
	amplitudeClientOnce.Do(func() {
		apiKey := os.Getenv("AMPLITUDE_API_KEY")
		amplitudeClient = amplitude.New(apiKey)
	})

	return amplitudeClient
}
