package state

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

type RoomStateManager struct {
	Log          *zap.Logger
	RoomID       string
	AvApiAddress string
	DisplayCache map[string]string
}

func (rm *RoomStateManager) ResolveRoom() error {
	rm.Log.Debug("parsing room id")
	bldg, room, err := parseRoomID(rm.RoomID)
	if err != nil {
		rm.Log.Error("failed to parse room id", zap.Error(err))
		return err
	}

	rm.Log.Debug("fetching room state from av-api")
	roomState, err := requestAVState("http://"+rm.AvApiAddress+"/buildings/"+bldg+"/rooms/"+room, rm.Log)
	if err != nil {
		rm.Log.Error("failed to request room state from the av-api", zap.Error(err))
		return err
	}

	rm.Log.Debug("grouping displays with similar inputs")
	displayGroups := groupDisplays(roomState)

	rm.Log.Debug("muting duplicates across all display groups")
	for input, group := range displayGroups {
		if len(group) >= 2 {
			rm.muteDuplicateDisplays(input, group, roomState)
		} else if len(group) == 1 {
			rm.DisplayCache[input] = group[0]
		}
	}

	rm.Log.Debug("sending updated room state to av-api")
	if err := updateAVState("http://"+rm.AvApiAddress+"/buildings/"+bldg+"/rooms/"+room, roomState, rm.Log); err != nil {
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
				if _, ok := inputGroups[disp.Input]; !ok {
					inputGroups[disp.Input] = []string{disp.Name}
				} else {
					inputGroups[disp.Input] = append(inputGroups[disp.Input], disp.Name)
				}
			}
		}
	}
	return inputGroups
}

func (rm *RoomStateManager) muteDuplicateDisplays(input string, displays []string, state *AVState) {
	chosenDisplay := -1

	if _, ok := rm.DisplayCache[input]; ok {
		for i, disp := range displays {
			if disp == rm.DisplayCache[input] {
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

	rm.DisplayCache[input] = displays[chosenDisplay]

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
