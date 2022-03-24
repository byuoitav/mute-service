package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
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
		deviceID   string
		hubAddress string
		apiAddress string
		dbAddress  string
	)

	pflag.StringVarP(&logLevel, "log-level", "L", "info", "Level at which the logger operates. Refer to https://godoc.org/go.uber.org/zap/zapcore#Level for options")
	pflag.StringVarP(&deviceID, "device-id", "", "", "Device id as found in couch")
	pflag.StringVarP(&hubAddress, "hub-address", "", "", "Address of the event hub")
	pflag.StringVarP(&apiAddress, "av-api", "", "", "Address of the av-api")
	pflag.StringVarP(&dbAddress, "db-address", "", "", "Address of the room database")
	pflag.Parse()

	//set up logger
	_, log := logger(logLevel)
	defer log.Sync()

	if deviceID == "" {
		log.Fatal("Device ID required. Use --device-id to provide the id of the device")
	} else if hubAddress == "" {
		log.Fatal("Event hub address required. Use --hub-address to provide the address of the event hub")
	} else if apiAddress == "" {
		log.Fatal("AV API address required. Use --av-api to provide the address of the av-api")
	}

	log.Info("Checking room configuration")
	cancel, err := cancelConditions(dbAddress, deviceID)
	if cancel {
		log.Info("cancel conditions met; sleeping...")
		for cancel {
			time.Sleep(300 * time.Second)
			if err != nil { // in the event of an error when accessing the room config, check again in 5 min
				cancel, err = cancelConditions(dbAddress, deviceID)
			}
		}
	}

	roomID, err := parseDeviceID(deviceID)
	if err != nil {
		log.Fatal(fmt.Sprintf("invalid device id: %s", deviceID), zap.Error(err))
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
	eventMessenger, nerr := messenger.BuildMessenger(hubAddress, base.Messenger, 5000)
	if nerr != nil {
		log.Fatal("failed to build event hub messenger", zap.Error(nerr))
	}

	// subscribe to and receive events from the hub
	log.Info("Listening for room events")
	eventMessenger.SubscribeToRooms(roomID)

	for {
		event := eventMessenger.ReceiveEvent()
		if checkEvent(event) {
			log.Debug(fmt.Sprintf("handling event of type: %s", event.Key))

			roomManager.HandleEvent(event)
		}
	}
}

func checkEvent(event events.Event) bool {
	return event.Key == "muted" || event.Key == "input" || event.Key == "power" || event.Value == "master volume mute on display page" || event.Value == "master volume set on display page"
}

func cancelConditions(dbAddress, deviceID string) (bool, error) {
	if checkForControlPi(deviceID) {
		status, err := checkRoomConfig(dbAddress, deviceID)
		return !status, err
	}
	return true, nil
}

func checkForControlPi(deviceID string) bool {
	found, _ := regexp.Match(`CP1`, []byte(deviceID))
	return found
}

func checkRoomConfig(dbAddress, deviceID string) (bool, error) {
	roomID, err := parseDeviceID(deviceID)
	if err != nil {
		return false, err
	}

	resp, err := http.Get("http://" + dbAddress + "/rooms/" + roomID)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	type configuration struct {
		AutoMute bool `json:"autoMute"`
	}

	type roomConfig struct {
		Config configuration `json:"configuration"`
	}

	var config roomConfig
	if err = json.Unmarshal(body, &config); err != nil {
		return false, err
	}

	return config.Config.AutoMute, nil
}

func parseDeviceID(id string) (string, error) {
	tokens := strings.Split(id, "-")
	if len(tokens) != 3 {
		return "", fmt.Errorf("invalid device id")
	}

	return tokens[0] + "-" + tokens[1], nil
}
