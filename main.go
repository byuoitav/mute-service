package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/byuoitav/mute-service/state"

	"github.com/byuoitav/central-event-system/hub/base"
	"github.com/byuoitav/central-event-system/messenger"
	"github.com/byuoitav/common/v2/events"
	"github.com/gin-gonic/gin"
)

var logger *slog.Logger
var roomManager *state.RoomStateManager

func main() {
	var err error

	deviceID := flag.String("device-id", "", "Device id as found in couch")
	hubAddress := flag.String("hub-address", "", "Address of the event hub")
	apiAddress := flag.String("av-api", "", "Address of the av-api")
	dbAddress := flag.String("db-address", "", "Address of the room database")
	dbUser := flag.String("db-user", "", "Username of the room database")
	dbPass := flag.String("db-pass", "", "Password of the room database")
	port := flag.String("p", "8080", "Port for the HTTP server")
	logLevelFlag := flag.String("l", "info", "slog log level")
	flag.Parse()

	//setup logger
	var logLevel = new(slog.LevelVar)

	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	logLevel.Set(slog.LevelInfo)
	if runtime.GOOS == "windows" {
		*logLevelFlag = "debug"
		logger.Info("running from Windows, logging set to debug")
	}

	err = setLogLevel(*logLevelFlag, logLevel)
	if err != nil {
		logger.Error("can not set log level", "error", err)
	}

	// Check for all missing required flags (flags without defaults)
	var missingFlags []string
	if *deviceID == "" {
		missingFlags = append(missingFlags, "device-id")
	}
	if *hubAddress == "" {
		missingFlags = append(missingFlags, "hub-address")
	}
	if *apiAddress == "" {
		missingFlags = append(missingFlags, "av-api")
	}
	if *dbAddress == "" {
		missingFlags = append(missingFlags, "db-address")
	}
	if *dbUser == "" {
		missingFlags = append(missingFlags, "db-user")
	}
	if *dbPass == "" {
		missingFlags = append(missingFlags, "db-pass")
	}

	if len(missingFlags) > 0 {
		logger.Error("Missing required flags - these flags are required and have no defaults",
			slog.Any("missing-flags", missingFlags),
			slog.String("help", "Use flag --help to see all available options"))
		os.Exit(1)
	}

	logger.Info("Starting mute service",
		slog.String("device-id", *deviceID),
		slog.String("hub-address", *hubAddress),
		slog.String("api-address", *apiAddress),
		slog.String("db-address", *dbAddress),
		slog.String("port", *port),
		slog.String("log-level", logLevel.Level().String()))

	// Start HTTP server in goroutine
	go startHTTPServer(*port, logLevel)

	logger.Info("Checking room configuration")
	cancel, err := cancelConditions(*dbAddress, *dbUser, *dbPass, *deviceID)
	if cancel {
		logger.Info("cancel conditions met; sleeping for 30 seconds...", slog.Any("error", err))
		for cancel {
			time.Sleep(30 * time.Second)
			if err != nil { // in the event of an error when accessing the room config, check again in 30 seconds
				logger.Warn("retrying room configuration check due to previous error", slog.Any("error", err))
				cancel, err = cancelConditions(*dbAddress, *dbUser, *dbPass, *deviceID)
			}
		}
	}

	roomID, err := parseDeviceID(*deviceID)
	if err != nil {
		logger.Error("invalid device id", slog.String("device-id", *deviceID), slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("Parsed room ID", slog.String("room-id", roomID))

	roomManager = &state.RoomStateManager{
		Log:                logger,
		RoomID:             roomID,
		AvApiAddress:       *apiAddress,
		RoomState:          nil,
		AudioPriorityCache: make(map[string]string),
	}

	// initialize room state on start up
	logger.Info("Initializing the room on startup")
	if err := roomManager.InitializeRoomState(); err != nil {
		logger.Error("failed to initialize room", slog.Any("error", err))
		os.Exit(1)
	}

	// connect to the event hub
	logger.Info("Starting event hub messenger", slog.String("hub-address", *hubAddress))
	eventMessenger, nerr := messenger.BuildMessenger(*hubAddress, base.Messenger, 5000)
	if nerr != nil {
		logger.Error("failed to build event hub messenger", slog.Any("error", nerr))
		os.Exit(1)
	}

	// subscribe to and receive events from the hub
	logger.Info("Listening for room events", slog.String("room-id", roomID))
	eventMessenger.SubscribeToRooms(roomID)

	for {
		event := eventMessenger.ReceiveEvent()
		if checkEvent(event) {
			logger.Debug("handling event",
				slog.String("event-type", event.Key),
				slog.String("event-value", event.Value),
				slog.String("device", event.TargetDevice.DeviceID))

			roomManager.HandleEvent(event)
		}
	}
}

func setLogLevel(level string, logLevel *slog.LevelVar) error {
	level = strings.ToLower(level)
	if level == "debug" {
		logLevel.Set(slog.LevelDebug)
	} else if level == "info" {
		logLevel.Set(slog.LevelInfo)
	} else if level == "warn" {
		logLevel.Set(slog.LevelWarn)
	} else if level == "error" {
		logLevel.Set(slog.LevelError)
	} else {
		return fmt.Errorf("the debug level must be one of (debug, info, warn, error) received %s", level)
	}
	return nil
}

func startHTTPServer(port string, logLevel *slog.LevelVar) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Health check endpoint
	r.GET("/healthz", func(c *gin.Context) {
		logger.Info("health check requested",
			slog.String("endpoint", "/healthz"),
			slog.String("method", "GET"),
			slog.String("client-ip", c.ClientIP()))
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Status endpoint
	r.GET("/status", func(c *gin.Context) {
		logger.Info("status requested",
			slog.String("endpoint", "/status"),
			slog.String("method", "GET"),
			slog.String("client-ip", c.ClientIP()))
		c.JSON(http.StatusOK, gin.H{
			"status":    "running",
			"log-level": logLevel.Level().String(),
			"uptime":    time.Now().Format(time.RFC3339),
		})
	})

	// Room state endpoint - shows current muting decisions and audio device states
	r.GET("/room-state", func(c *gin.Context) {
		logger.Info("room state requested",
			slog.String("endpoint", "/room-state"),
			slog.String("method", "GET"),
			slog.String("client-ip", c.ClientIP()))

		// Access the global room manager (we'll need to pass this in)
		if roomManager == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "room manager not initialized",
			})
			return
		}

		// Group displays to show current input groupings
		displayGroups := state.GroupDisplays(roomManager.RoomState)

		// Create response with all pertinent info
		response := gin.H{
			"room_id":              roomManager.RoomID,
			"audio_devices":        roomManager.RoomState.AudioDevices,
			"displays":             roomManager.RoomState.Displays,
			"display_groups":       displayGroups,
			"audio_priority_cache": roomManager.AudioPriorityCache,
			"power_status":         roomManager.CheckPower(),
			"timestamp":            time.Now().Format(time.RFC3339),
		}

		c.IndentedJSON(http.StatusOK, response)
	})

	// Get current log level
	r.GET("/log-level", func(c *gin.Context) {
		logger.Info("log level requested",
			slog.String("endpoint", "/log-level"),
			slog.String("method", "GET"),
			slog.String("client-ip", c.ClientIP()))
		c.JSON(http.StatusOK, gin.H{"log-level": logLevel.Level().String()})
	})

	// Set log level (GET)
	r.GET("/log-level/:level", func(c *gin.Context) {
		level := c.Param("level")
		logger.Info("log level change requested",
			slog.String("endpoint", "/log-level/:level"),
			slog.String("method", "GET"),
			slog.String("requested-level", level),
			slog.String("client-ip", c.ClientIP()))

		err := setLogLevel(level, logLevel)
		if err != nil {
			logger.Error("can not set log level", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		logger.Info("log level successfully changed",
			slog.String("new-level", logLevel.Level().String()))
		c.JSON(http.StatusOK, gin.H{
			"message":   "log level updated",
			"log-level": logLevel.Level().String(),
		})
	})

	// Set log level (PUT)
	r.PUT("/log-level/:level", func(c *gin.Context) {
		level := c.Param("level")
		logger.Info("log level change requested",
			slog.String("endpoint", "/log-level/:level"),
			slog.String("method", "PUT"),
			slog.String("requested-level", level),
			slog.String("client-ip", c.ClientIP()))

		err := setLogLevel(level, logLevel)
		if err != nil {
			logger.Error("can not set log level", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		logger.Info("log level successfully changed",
			slog.String("new-level", logLevel.Level().String()))
		c.JSON(http.StatusOK, gin.H{
			"message":   "log level updated",
			"log-level": logLevel.Level().String(),
		})
	})

	logger.Info("Starting HTTP server", slog.String("port", port))
	if err := r.Run(":" + port); err != nil {
		logger.Error("failed to start HTTP server", slog.Any("error", err))
		os.Exit(1)
	}
}

func checkEvent(event events.Event) bool {
	return event.Key == "muted" || event.Key == "input" || event.Key == "power" || event.Value == "master volume mute on display page" || event.Value == "master volume set on display page"
}

func cancelConditions(dbAddress, dbUser, dbPass, deviceID string) (bool, error) {
	if checkForControlPi(deviceID) {
		status, err := checkRoomConfig(dbAddress, dbUser, dbPass, deviceID)
		if err != nil {
			logger.Error("failed to check room configuration",
				slog.String("device-id", deviceID),
				slog.String("db-address", dbAddress),
				slog.Any("error", err))
		}
		return !status, err
	}
	return true, nil
}

func checkForControlPi(deviceID string) bool {
	found, _ := regexp.Match(`CP1`, []byte(deviceID))
	return found
}

func checkRoomConfig(dbAddress, dbUser, dbPass, deviceID string) (bool, error) {
	roomID, err := parseDeviceID(deviceID)
	if err != nil {
		logger.Error("failed to parse device ID", slog.String("device-id", deviceID), slog.Any("error", err))
		return false, err
	}

	url := "http://" + dbAddress + "/rooms/" + roomID
	logger.Debug("checking room configuration", slog.String("url", url))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("failed to create HTTP request", slog.String("url", url), slog.Any("error", err))
		return false, err
	}

	if dbUser != "" && dbPass != "" {
		req.SetBasicAuth(dbUser, dbPass)
		logger.Debug("using basic authentication for database request")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("failed to make HTTP request to database",
			slog.String("url", url),
			slog.Any("error", err))
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		logger.Error("authentication failed for database request",
			slog.String("url", url),
			slog.Int("status-code", resp.StatusCode))
		return false, fmt.Errorf("authentication failed: %d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("unexpected status code from database",
			slog.String("url", url),
			slog.Int("status-code", resp.StatusCode))
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("failed to read response body", slog.Any("error", err))
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
		logger.Error("failed to unmarshal room configuration",
			slog.String("body", string(body)),
			slog.Any("error", err))
		return false, err
	}

	logger.Info("room configuration retrieved successfully",
		slog.String("room-id", roomID),
		slog.Bool("auto-mute", config.Config.AutoMute))

	return config.Config.AutoMute, nil
}

func parseDeviceID(id string) (string, error) {
	tokens := strings.Split(id, "-")
	if len(tokens) != 3 {
		return "", fmt.Errorf("invalid device id")
	}

	return tokens[0] + "-" + tokens[1], nil
}
