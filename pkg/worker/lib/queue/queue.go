package queue

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/RichardKnop/machinery/v1"
	"github.com/RichardKnop/machinery/v1/config"
	"github.com/sirupsen/logrus"
)

var server *machinery.Server
var initOnce sync.Once

func initServer() {
	redisURL := fmt.Sprintf("%s/1", os.Getenv("REDIS_URL")) // use separate DB #1 for queue
	logrus.Infof("REDIS_URL=%q", redisURL)

	cnf := &config.Config{
		Broker:          redisURL,
		DefaultQueue:    "machinery_tasks",
		ResultBackend:   redisURL,
		ResultsExpireIn: int((7 * 24 * time.Hour).Seconds()), // store results for 1 week
	}

	var err error
	server, err = machinery.NewServer(cnf)
	if err != nil {
		log.Fatalf("Can't init machinery queue server: %s", err)
	}
}

func Init() {
	initOnce.Do(initServer)
}

func GetServer() *machinery.Server {
	return server
}
