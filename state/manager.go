package state

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/byuoitav/common/v2/events"
	"go.uber.org/zap"
)

type RoomStateManager struct {
	Log                *zap.Logger
	RoomID             string
	AvApiAddress       string
	RoomState          *AVState
	AudioPriorityCache map[string]string
}

func (rm *RoomStateManager) HandleEvent(event events.Event) {
	if event.Key == "power" {
		rm.Log.Debug("power event")
		if event.Value == "standby" && rm.checkPower() {
			rm.powerOff()
		} else if event.Value == "on" && !rm.checkPower() {
			rm.powerOn()
		}
	} else if rm.checkPower() {
		switch event.Key {
		case "muted":
			rm.Log.Debug("muted event")
			mutedStatus, err := strconv.ParseBool(event.Value)
			if err != nil {
				rm.Log.Error("muted event returned a value that is not a boolean")
				return
			}
			if disp, same := rm.compareMute(event.TargetDevice.DeviceID, mutedStatus); !same {
				rm.Log.Debug(fmt.Sprintf("%s : %v", event.TargetDevice.DeviceID, mutedStatus))
				disp.Muted = mutedStatus

				rm.ResolveRoom()
			}
		case "input":
			rm.Log.Debug("input event")
			if disp, same := rm.compareInput(event.TargetDevice.DeviceID, event.Value); !same {
				rm.Log.Debug(fmt.Sprintf("%s : %s", event.TargetDevice.DeviceID, event.Value))
				disp.Input = event.Value

				rm.ResolveRoom()
			}
		case "user-interaction":
			rm.Log.Debug("master mute pressed")
			if event.Value == "master volume mute on display page" {
				for i := range rm.RoomState.AudioDevices {
					rm.RoomState.AudioDevices[i].Muted = true
				}

				rm.Log.Debug("parsing room id")
				bldg, room, err := parseRoomID(rm.RoomID)
				if err != nil {
					rm.Log.Error("failed to parse room id", zap.Error(err))
					return
				}

				rm.Log.Debug("sending updated room state to av-api")
				if err := updateAVState("http://"+rm.AvApiAddress+"/buildings/"+bldg+"/rooms/"+room, rm.RoomState, rm.Log); err != nil {
					rm.Log.Error("failed to update room state on av-api")
					return
				}

				rm.Log.Debug(fmt.Sprint(rm.RoomState))
			} else if event.Value == "master volume set on display page" {
				rm.Log.Debug("master volume changed, resolving room muting")

				rm.Log.Debug("parsing room id")
				bldg, room, err := parseRoomID(rm.RoomID)
				if err != nil {
					rm.Log.Error("failed to parse room id", zap.Error(err))
					return
				}

				rm.Log.Debug("resending room state to av-api")
				if err := updateAVState("http://"+rm.AvApiAddress+"/buildings/"+bldg+"/rooms/"+room, rm.RoomState, rm.Log); err != nil {
					rm.Log.Error("failed to update room state on av-api")
					return
				}
			}
		}
	}
}

func (rm *RoomStateManager) checkPower() bool {
	for _, disp := range rm.RoomState.AudioDevices {
		if disp.Power == "standby" {
			return false
		}
	}
	return true
}

func (rm *RoomStateManager) powerOn() {
	rm.Log.Debug("power on")
	for i := range rm.RoomState.AudioDevices {
		rm.RoomState.AudioDevices[i].Power = "on"
	}

	rm.ResolveRoom()
}

func (rm *RoomStateManager) powerOff() {
	rm.Log.Debug("power off")
	for i := range rm.RoomState.AudioDevices {
		rm.RoomState.AudioDevices[i].Power = "standby"
		rm.RoomState.AudioDevices[i].Muted = false
	}
}

func (rm *RoomStateManager) compareInput(id, input string) (*AudioDevice, bool) {
	displayID, err := parseDisplayID(id)
	if err != nil {
		return nil, true
	}

	d := rm.findDisplay(displayID)
	if d == nil {
		// display is not an audio device
		return d, true
	}

	return d, d.Input == input
}

func (rm *RoomStateManager) compareMute(id string, muted bool) (*AudioDevice, bool) {
	displayID, err := parseDisplayID(id)
	if err != nil {
		return nil, true
	}

	d := rm.findDisplay(displayID)
	if d == nil {
		// display is not an audio device
		return d, true
	}

	return d, d.Muted == muted
}

func (rm *RoomStateManager) findDisplay(id string) *AudioDevice {
	for i, disp := range rm.RoomState.AudioDevices {
		if disp.Name == id {
			return &rm.RoomState.AudioDevices[i]
		}
	}
	return nil
}

func (rm *RoomStateManager) InitializeRoomState() error {
	rm.Log.Debug("parsing room id")
	bldg, room, err := parseRoomID(rm.RoomID)
	if err != nil {
		rm.Log.Error("failed to parse room id", zap.Error(err))
		return err
	}

	rm.Log.Debug("fetching room state from av-api")
	currentState, err := requestAVState("http://"+rm.AvApiAddress+"/buildings/"+bldg+"/rooms/"+room, rm.Log)
	if err != nil {
		rm.Log.Error("failed to request room state from the av-api", zap.Error(err))
		return err
	}

	rm.RoomState = currentState
	rm.Log.Debug(fmt.Sprint(rm.RoomState))

	return nil
}

func (rm *RoomStateManager) ResolveRoom() error {
	rm.Log.Debug(fmt.Sprint(rm.RoomState))
	rm.Log.Debug("grouping displays with similar inputs")
	displayGroups := groupDisplays(rm.RoomState)
	rm.Log.Debug(fmt.Sprintf("Display groups: %v", displayGroups))

	rm.Log.Debug("muting duplicates across all display groups")
	for input, group := range displayGroups {
		if len(group) >= 2 {
			rm.muteDuplicateDisplays(input, group, rm.RoomState)
		} else if len(group) == 1 {
			rm.AudioPriorityCache[input] = group[0]
			d := rm.findDisplay(group[0])
			d.Muted = false
		}
	}
	rm.Log.Debug(fmt.Sprint(rm.RoomState))

	rm.Log.Debug("parsing room id")
	bldg, room, err := parseRoomID(rm.RoomID)
	if err != nil {
		rm.Log.Error("failed to parse room id", zap.Error(err))
		return err
	}

	rm.Log.Debug("sending updated room state to av-api")
	if err := updateAVState("http://"+rm.AvApiAddress+"/buildings/"+bldg+"/rooms/"+room, rm.RoomState, rm.Log); err != nil {
		rm.Log.Error("failed to update room state on av-api")
		return err
	}
	return nil
}

func groupDisplays(state *AVState) map[string][]string {
	inputGroups := make(map[string][]string)
	for _, disp := range state.Displays {
		for _, audioDev := range state.AudioDevices {
			if disp.Name == audioDev.Name {
				if _, ok := inputGroups[audioDev.Input]; !ok {
					inputGroups[audioDev.Input] = []string{disp.Name}
				} else {
					inputGroups[audioDev.Input] = append(inputGroups[audioDev.Input], disp.Name)
				}
			}
		}
	}
	return inputGroups
}

func (rm *RoomStateManager) muteDuplicateDisplays(input string, displays []string, state *AVState) {
	chosenDisplay := -1

	if _, ok := rm.AudioPriorityCache[input]; ok {
		for i, disp := range displays {
			if disp == rm.AudioPriorityCache[input] {
				chosenDisplay = i
			}
		}
	}

	if chosenDisplay == -1 {
		lowestDisplayNum := 100
		chosenDisplay = 0
		for i, disp := range displays {
			num, err := parseDisplayNumber(disp)
			if err == nil && num < lowestDisplayNum {
				lowestDisplayNum = num
				chosenDisplay = i
			}
		}
	}

	rm.AudioPriorityCache[input] = displays[chosenDisplay]

	for i := range state.AudioDevices {
		if state.AudioDevices[i].Name == displays[chosenDisplay] {
			state.AudioDevices[i].Muted = false
		} else if state.AudioDevices[i].Input == input {
			state.AudioDevices[i].Muted = true
		}
	}
}

func parseDisplayNumber(displayName string) (int, error) {
	re, err := regexp.Compile(`D([0-9]+)`)
	if err != nil {
		return -1, err
	}

	r := re.FindStringSubmatch(displayName)
	if len(r) < 2 {
		return -1, fmt.Errorf("failed to parse display name; not expected format: `D#`")
	}
	return strconv.Atoi(r[1])
}

func parseRoomID(id string) (string, string, error) {
	tokens := strings.Split(id, "-")
	if len(tokens) < 2 {
		return "", "", fmt.Errorf("invalid room id: %s", id)
	}
	return tokens[0], tokens[1], nil
}

func parseDisplayID(deviceID string) (string, error) {
	tokens := strings.Split(deviceID, "-")
	if len(tokens) < 3 {
		return "", fmt.Errorf("invalid device id: %s", deviceID)
	}
	return tokens[2], nil
}
