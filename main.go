package main

import (
	"fmt"

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
	)

	pflag.StringVarP(&logLevel, "log-level", "L", "info", "Level at which the logger operates. Refer to https://godoc.org/go.uber.org/zap/zapcore#Level for options")
	pflag.StringVarP(&roomID, "room-id", "", "", "Room id as found in couch")
	pflag.StringVarP(&hubAddress, "hub-address", "", "", "Address of the event hub")
	pflag.StringVarP(&apiAddress, "av-api", "", "", "Address of the av-api")
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

	roomManager := &state.RoomStateManager{
		Log:          log,
		RoomID:       roomID,
		AvApiAddress: apiAddress,
		DisplayCache: make(map[string]string),
	}

	// run on startup
	log.Info("Initializing the room on startup")
	if err := roomManager.ResolveRoom(); err != nil {
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
		log.Debug("received event", zap.Any("event", event))
		if checkEvent(event) {
			log.Debug(fmt.Sprintf("handling event of type: %s", event.Key))
			roomManager.ResolveRoom()
		}
	}
}

func checkEvent(event events.Event) bool {
	return (event.Key == "muted" && event.Value == "false") || event.Key == "input"
}
