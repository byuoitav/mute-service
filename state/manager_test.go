package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveRoom(t *testing.T) {

}

func TestGroupDisplays(t *testing.T) {
	testState := &AVState{
		Displays: []Displays{
			{
				Name:    "D1",
				Power:   "on",
				Input:   "VIA1",
				Blanked: false,
			},
			{
				Name:    "D2",
				Power:   "on",
				Input:   "PC1",
				Blanked: false,
			},
			{
				Name:    "D3",
				Power:   "on",
				Input:   "PC1",
				Blanked: false,
			},
			{
				Name:    "D4",
				Power:   "on",
				Input:   "VIA1",
				Blanked: false,
			},
		},
		AudioDevices: []AudioDevices{
			{
				Name:   "D1",
				Power:  "on",
				Input:  "VIA1",
				Muted:  false,
				Volume: 30,
			},
			{
				Name:   "D2",
				Power:  "on",
				Input:  "PC1",
				Muted:  false,
				Volume: 30,
			},
			{
				Name:   "D3",
				Power:  "on",
				Input:  "PC1",
				Muted:  false,
				Volume: 30,
			},
			{
				Name:   "D4",
				Power:  "on",
				Input:  "VIA1",
				Muted:  false,
				Volume: 30,
			},
		},
	}

	inputGroups := groupDisplays(testState)
	assert.Equal(t, len(inputGroups), 2, "")
	assert.Equal(t, len(inputGroups["VIA1"]), 2, "")
	assert.Equal(t, len(inputGroups["PC1"]), 2, "")

	testState = &AVState{
		Displays: []Displays{
			{
				Name:    "D1",
				Power:   "on",
				Input:   "VIA1",
				Blanked: false,
			},
			{
				Name:    "D2",
				Power:   "on",
				Input:   "VIA2",
				Blanked: false,
			},
			{
				Name:    "D3",
				Power:   "on",
				Input:   "VIA3",
				Blanked: false,
			},
		},
		AudioDevices: []AudioDevices{
			{
				Name:   "D1",
				Power:  "on",
				Input:  "VIA1",
				Muted:  false,
				Volume: 30,
			},
			{
				Name:   "D2",
				Power:  "on",
				Input:  "VIA2",
				Muted:  false,
				Volume: 30,
			},
			{
				Name:   "D3",
				Power:  "on",
				Input:  "VIA3",
				Muted:  false,
				Volume: 30,
			},
		},
	}

	inputGroups = groupDisplays(testState)
	assert.Equal(t, len(inputGroups), 3, "")
	assert.Equal(t, len(inputGroups["VIA1"]), 1, "")
	assert.Equal(t, len(inputGroups["VIA2"]), 1, "")
	assert.Equal(t, len(inputGroups["VIA3"]), 1, "")
}

func TestMuteDuplicateDisplays(t *testing.T) {
	manager := &RoomStateManager{
		Log:          nil,
		RoomID:       "",
		AvApiAddress: "",
		DisplayCache: make(map[string]string),
	}

	// check if the default lowest number is chosen since the cache is empty
	displays := []string{
		"D4",
		"D2",
		"D3",
	}

	avState := &AVState{
		Displays: []Displays{
			{
				Name:    "D4",
				Power:   "on",
				Input:   "VIA1",
				Blanked: false,
			},
			{
				Name:    "D2",
				Power:   "on",
				Input:   "VIA1",
				Blanked: false,
			},
			{
				Name:    "D3",
				Power:   "on",
				Input:   "VIA1",
				Blanked: false,
			},
		},
		AudioDevices: []AudioDevices{
			{
				Name:   "D4",
				Power:  "on",
				Input:  "VIA1",
				Muted:  false,
				Volume: 30,
			},
			{
				Name:   "D2",
				Power:  "on",
				Input:  "VIA1",
				Muted:  false,
				Volume: 30,
			},
			{
				Name:   "D3",
				Power:  "on",
				Input:  "VIA1",
				Muted:  false,
				Volume: 30,
			},
		},
	}

	manager.muteDuplicateDisplays("VIA1", displays, avState)
	assert.Equal(t, avState.AudioDevices[0].Muted, true, "")
	assert.Equal(t, avState.AudioDevices[1].Muted, false, "")
	assert.Equal(t, avState.AudioDevices[2].Muted, true, "")

	// check to see if the cache works
	// D2 should be remembered and not muted
	displays = []string{
		"D1",
		"D2",
		"D3",
	}

	avState = &AVState{
		Displays: []Displays{
			{
				Name:    "D1",
				Power:   "on",
				Input:   "VIA1",
				Blanked: false,
			},
			{
				Name:    "D2",
				Power:   "on",
				Input:   "VIA1",
				Blanked: false,
			},
			{
				Name:    "D3",
				Power:   "on",
				Input:   "VIA1",
				Blanked: false,
			},
		},
		AudioDevices: []AudioDevices{
			{
				Name:   "D1",
				Power:  "on",
				Input:  "VIA1",
				Muted:  false,
				Volume: 30,
			},
			{
				Name:   "D2",
				Power:  "on",
				Input:  "VIA1",
				Muted:  false,
				Volume: 30,
			},
			{
				Name:   "D3",
				Power:  "on",
				Input:  "VIA1",
				Muted:  false,
				Volume: 30,
			},
		},
	}

	manager.muteDuplicateDisplays("VIA1", displays, avState)
	assert.Equal(t, avState.AudioDevices[0].Muted, true, "")
	assert.Equal(t, avState.AudioDevices[1].Muted, false, "")
	assert.Equal(t, avState.AudioDevices[2].Muted, true, "")

	d := manager.DisplayCache["VIA1"]
	assert.Equal(t, d, "D2", "")
}

func TestParseDisplayNumber(t *testing.T) {
	displays := []string{
		"D1",
		"D10",
		"Bad",
	}

	num, err := parseDisplayNumber(displays[0])
	assert.Equal(t, num, 1, "")
	assert.Nil(t, err)

	num, err = parseDisplayNumber(displays[1])
	assert.Equal(t, num, 10, "")
	assert.Nil(t, err)

	_, err = parseDisplayNumber(displays[2])
	assert.NotNil(t, err)
}

func TestParseRoomID(t *testing.T) {
	bldg, room, err := parseRoomID("ITB-1106")
	assert.Equal(t, bldg, "ITB", "")
	assert.Equal(t, room, "1106", "")
	assert.Nil(t, err)

	_, _, err = parseRoomID("BadID")
	assert.NotNil(t, err)
}
