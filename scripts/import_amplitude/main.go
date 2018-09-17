package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/shared"
	"github.com/golangci/golib/server/database"
	"github.com/jinzhu/gorm"
	uuid "github.com/satori/go.uuid"
)

func main() {
	inputFile := flag.String("input-file", "amp.json", "Amplitude json file path")
	flag.Parse()

	shared.Init()

	if err := importAmplitude(*inputFile); err != nil {
		log.Fatal(err)
	}
}

func importAmplitude(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("can't open file %s: %s", filePath, err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		eventLine := s.Text()
		if err = importEvent(eventLine); err != nil {
			return fmt.Errorf("can't import event %s: %s", eventLine, err)
		}
	}

	return nil
}

type EventTime time.Time

func (et *EventTime) UnmarshalJSON(data []byte) error {
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}

	layouts := []string{"2006-01-02 15:04:05.000000", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err != nil {
			continue
		}

		*et = EventTime(t)
		return nil
	}

	return fmt.Errorf("can't parse time %s", s)
}

type Event struct {
	EventProperties struct {
		RepoName    string
		TotalIssues int
		Status      string
		PRNumber    int
	} `json:"event_properties"`
	EventTime EventTime `json:"event_time"`
}

func importEvent(eventJSON string) error {
	var event Event
	if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
		return fmt.Errorf("can't unmarshal event json: %s", err)
	}

	if event.EventProperties.RepoName == "" { // not interesting event type
		return nil
	}

	et := time.Time(event.EventTime)
	if et.After(time.Date(2018, 04, 13, 0, 0, 0, 0, time.UTC)) {
		// after that time we already save analyzes into pg
		return nil
	}

	return storeEvent(&event)
}

func storeEvent(event *Event) error {
	var gr models.Repo
	err := models.NewRepoQuerySet(database.GetDB().Unscoped()).
		NameEq(event.EventProperties.RepoName).
		One(&gr)
	if err != nil {
		return fmt.Errorf("can't find repo %s: %s", event.EventProperties.RepoName, err)
	}

	et := time.Time(event.EventTime)
	status := "processed/success"
	if event.EventProperties.Status != "ok" {
		status = "processed/failure"
	}
	ga := models.PullRequestAnalysis{
		Model: gorm.Model{
			CreatedAt: et,
			UpdatedAt: et,
		},
		RepoID:             gr.ID,
		PullRequestNumber:  event.EventProperties.PRNumber,
		GithubDeliveryGUID: fmt.Sprintf("fake_%s", uuid.NewV4().String()),

		Status:              status,
		ReportedIssuesCount: event.EventProperties.TotalIssues,
		// TODO: ResultJSON
	}

	if err = ga.Create(database.GetDB()); err != nil {
		return fmt.Errorf("can't create pull request analysis: %s", err)
	}

	log.Print(ga)
	return nil
}
