# Mute Service

This service monitors room events from the event hub and automatically manages audio muting in AV rooms. When multiple displays in a room are showing the same input source, the service intelligently mutes duplicate audio outputs to prevent audio conflicts and feedback loops.

## How it works

The mute service:

1. Connects to the event hub and subscribes to room events for a specific device
2. Monitors for `muted`, `input`, `power`, and `user-interaction` events
3. Tracks the current state of all audio devices in the room via the AV API
4. When multiple displays share the same input source, automatically mutes all but one display
5. Uses an intelligent priority system to determine which display should remain unmuted:
    - First checks the audio priority cache for previously selected displays
    - If no cached preference exists, defaults to the lowest numbered display (e.g., D1 over D2)
6. Maintains audio priority cache to remember user preferences across input changes

The service ensures only one audio output is active per input source, preventing echo and audio conflicts in multi-display rooms. It also respects room configuration settings and can be disabled for rooms that don't require auto-muting.

## Required Flags

All of these flags are required and have no defaults:

-   `device-id` - Device ID as found in couch (e.g., "ITB-1108A-CP1")
-   `hub-address` - Address of the event hub (e.g., "localhost:7000")
-   `av-api` - Address of the av-api (e.g., "localhost:8000")
-   `db-address` - Address of the room database (e.g., "localhost:5984")
-   `db-user` - Username for the room database
-   `db-pass` - Password for the room database

## Optional Flags

-   `-p` - Port for the HTTP server (default: "8080")
-   `-l` - Log level: debug, info, warn, error (default: "info", auto-set to "debug" on Windows)

## Example Usage

```bash
./mute-service \
  -device-id "ITB-1108A-CP1" \
  -hub-address "event-hub.example.com:7000" \
  -av-api "av-api.example.com:8000" \
  -db-address "room-db.example.com:5984" \
  -db-user "admin" \
  -db-pass "password" \
  -p "8080" \
  -l "info"
```

## HTTP Endpoints

-   `GET /healthz` - Health check endpoint, returns service health status
-   `GET /status` - Service status including current log level and uptime
-   `GET /room-state` - Current room state with audio devices, display groups, muting decisions, and priority cache
-   `GET /log-level` - Get current log level
-   `GET /log-level/:level` - Set log level (debug, info, warn, error)
-   `PUT /log-level/:level` - Set log level via PUT request

## Monitored Events

The service listens for and responds to these event types from the event hub:

-   **muted** - When a display's mute status changes
-   **input** - When a display's input source changes
-   **power** - When a display's power state changes (on/standby)
-   **user-interaction** - Special events like "master volume mute on display page"

## Room Configuration

The service checks the room's configuration in the database to determine if auto-muting should be enabled. Rooms with `autoMute: false` in their configuration will cause the service to sleep and periodically re-check rather than actively processing events.

For Control Pi devices (containing "CP1" in the device ID), the service validates the room configuration before starting. Non-Control Pi devices bypass this check.

## Audio Priority Cache

The service maintains an in-memory cache that remembers which display was chosen as the primary audio output for each input source. This ensures consistent behavior when users switch between inputs - the same display will remain unmuted when returning to a previously used input source.

## Logging

The service uses structured JSON logging via Go's `log/slog` package. Log levels can be changed at runtime via the HTTP endpoints. When running on Windows, the log level automatically defaults to "debug" for development purposes.

## Room State Debugging

The `/room-state` endpoint provides detailed information about the current state of the room for debugging and monitoring:

```json
{
    "room_id": "ITB-1108",
    "audio_devices": [
        {
            "name": "D1",
            "muted": false,
            "power": "on",
            "input": "VIA1"
        },
        {
            "name": "D2",
            "muted": true,
            "power": "on",
            "input": "VIA1"
        }
    ],
    "displays": [{ "name": "D1" }, { "name": "D2" }],
    "display_groups": {
        "VIA1": ["D1", "D2"]
    },
    "audio_priority_cache": {
        "VIA1": "D1"
    },
    "power_status": true,
    "timestamp": "2024-01-15T10:30:00Z"
}
```

This endpoint is useful for understanding which displays are currently muted, how displays are grouped by input source, and what the audio priority cache contains.
