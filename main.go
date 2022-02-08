package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/byuoitav/mute-service/state"

	"github.com/byuoitav/central-event-system/hub/base"
	"github.com/byuoitav/central-event-system/messenger"
	"github.com/byuoitav/common/v2/events"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func main() {
	var (
		logLevel   string
		roomID     string
		hubAddress string
		apiAddress string
		dbAddress  string
	)

	pflag.StringVarP(&logLevel, "log-level", "L", "info", "Level at which the logger operates. Refer to https://godoc.org/go.uber.org/zap/zapcore#Level for options")
	pflag.StringVarP(&roomID, "room-id", "", "", "Room id as found in couch")
	pflag.StringVarP(&hubAddress, "hub-address", "", "", "Address of the event hub")
	pflag.StringVarP(&apiAddress, "av-api", "", "", "Address of the av-api")
	pflag.StringVarP(&dbAddress, "db-address", "", "", "Address of the room database")
	pflag.Parse()

	//set up logger
	_, log := logger(logLevel)
	defer log.Sync()

	if roomID == "" {
		log.Fatal("Room ID required. Use --room-id to provide the id of the room")
	} else if hubAddress == "" {
		log.Fatal("Event hub address required. Use --hub-address to provide the address of the event hub")
	} else if apiAddress == "" {
		log.Fatal("AV API address required. Use --av-api to provide the address of the av-api")
	}

	log.Info("Checking room configuration")
	if cancelConditions(dbAddress, roomID) {
		log.Info("cancel conditions met; sleeping...")
		for {
			time.Sleep(600 * time.Second)
		}
	}

	roomManager := &state.RoomStateManager{
		Log:                log,
		RoomID:             roomID,
		AvApiAddress:       apiAddress,
		RoomState:          nil,
		AudioPriorityCache: make(map[string]string),
	}

	// initialize room state on start up
	log.Info("Initializing the room on startup")
	if err := roomManager.InitializeRoomState(); err != nil {
		log.Fatal("failed to initialize room", zap.Error(err))
	}

	// connect to the event hub
	log.Info("Starting event hub messenger")
	messenger, err := messenger.BuildMessenger(hubAddress, base.Messenger, 5000)
	if err != nil {
		log.Fatal("failed to build event hub messenger", zap.Error(err))
	}

	// subscribe to and receive events from the hub
	log.Info("Listening for room events")
	messenger.SubscribeToRooms(roomID)

	for {
		event := messenger.ReceiveEvent()
		if checkEvent(event) {
			log.Debug(fmt.Sprintf("handling event of type: %s", event.Key))

			roomManager.HandleEvent(event)
		}
	}
}

func checkEvent(event events.Event) bool {
	return event.Key == "muted" || event.Key == "input" || event.Key == "power" || event.Value == "master volume mute on display page"
}

func cancelConditions(dbAddress, roomID string) bool {
	if checkHostname() {
		return !checkRoomConfig(dbAddress, roomID)
	}
	return true
}

func checkHostname() bool {
	hostname, err := os.ReadFile("/etc/hostname")
	if err != nil {
		return false
	}

	found, _ := regexp.Match(`CP1`, hostname)
	return found
}

func checkRoomConfig(dbAddress, roomID string) bool {
	//get room config
	resp, err := http.Get("http://" + dbAddress + "/rooms/" + roomID)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	type configuration struct {
		AutoMute bool `json:"autoMute"`
	}

	type roomConfig struct {
		Config configuration `json:"configuration"`
	}

	var config roomConfig
	if err = json.Unmarshal(body, &config); err != nil {
		return false
	}

	return config.Config.AutoMute
}
