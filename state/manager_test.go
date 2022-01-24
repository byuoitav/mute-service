package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveRoom(t *testing.T) {

}

func TestPowerOn(t *testing.T) {
	manager := &RoomStateManager{
		Log:                nil,
		RoomID:             "",
		AvApiAddress:       "",
		AudioPriorityCache: make(map[string]string),
		RoomState: &AVState{
			AudioDevices: []AudioDevice{
				{
					Name:  "D1",
					Power: "standby",
					Input: "VIA1",
					Muted: false,
				},
				{
					Name:  "D2",
					Power: "standby",
					Input: "VIA1",
					Muted: false,
				},
				{
					Name:  "D3",
					Power: "standby",
					Input: "VIA1",
					Muted: false,
				},
			},
		},
	}

	manager.powerOn()

	assert.Equal(t, true, manager.checkPower())
}

func TestPowerOff(t *testing.T) {
	manager := &RoomStateManager{
		Log:                nil,
		RoomID:             "",
		AvApiAddress:       "",
		AudioPriorityCache: make(map[string]string),
		RoomState: &AVState{
			AudioDevices: []AudioDevice{
				{
					Name:  "D1",
					Power: "on",
					Input: "VIA1",
					Muted: false,
				},
				{
					Name:  "D2",
					Power: "on",
					Input: "VIA1",
					Muted: false,
				},
				{
					Name:  "D3",
					Power: "on",
					Input: "VIA1",
					Muted: false,
				},
			},
		},
	}

	manager.powerOff()

	assert.Equal(t, false, manager.checkPower())
}

func TestCompareInput(t *testing.T) {
	manager := &RoomStateManager{
		Log:                nil,
		RoomID:             "",
		AvApiAddress:       "",
		AudioPriorityCache: make(map[string]string),
		RoomState: &AVState{
			AudioDevices: []AudioDevice{
				{
					Name:  "D1",
					Power: "on",
					Input: "VIA1",
					Muted: false,
				},
				{
					Name:  "D2",
					Power: "on",
					Input: "VIA1",
					Muted: false,
				},
				{
					Name:  "D3",
					Power: "on",
					Input: "VIA1",
					Muted: false,
				},
			},
		},
	}

	display, ok := manager.compareInput("D2", "VIA1")
	assert.Equal(t, true, ok)
	assert.Equal(t, &manager.RoomState.AudioDevices[1], display)

	display, ok = manager.compareInput("D2", "VIA7")
	assert.Equal(t, false, ok)
	assert.Equal(t, &manager.RoomState.AudioDevices[1], display)

	display, ok = manager.compareInput("D20", "VIA1")
	assert.Equal(t, true, ok)
	assert.Nil(t, display)
}

func TestCompareMute(t *testing.T) {
	manager := &RoomStateManager{
		Log:                nil,
		RoomID:             "",
		AvApiAddress:       "",
		AudioPriorityCache: make(map[string]string),
		RoomState: &AVState{
			AudioDevices: []AudioDevice{
				{
					Name:  "D1",
					Power: "on",
					Input: "VIA1",
					Muted: false,
				},
				{
					Name:  "D2",
					Power: "on",
					Input: "VIA1",
					Muted: true,
				},
				{
					Name:  "D3",
					Power: "on",
					Input: "VIA1",
					Muted: true,
				},
			},
		},
	}

	display, ok := manager.compareMute("D2", true)
	assert.Equal(t, true, ok)
	assert.Equal(t, &manager.RoomState.AudioDevices[1], display)

	display2, ok := manager.compareMute("D2", false)
	assert.Equal(t, false, ok)
	assert.Equal(t, &manager.RoomState.AudioDevices[1], display2)

	display3, ok := manager.compareMute("D20", true)
	assert.Equal(t, true, ok)
	assert.Nil(t, display3)
}

func TestFindDisplay(t *testing.T) {
	manager := &RoomStateManager{
		Log:                nil,
		RoomID:             "",
		AvApiAddress:       "",
		AudioPriorityCache: make(map[string]string),
		RoomState: &AVState{
			AudioDevices: []AudioDevice{
				{
					Name:  "D1",
					Power: "on",
					Input: "VIA1",
					Muted: false,
				},
				{
					Name:  "D2",
					Power: "on",
					Input: "VIA1",
					Muted: false,
				},
				{
					Name:  "D3",
					Power: "on",
					Input: "VIA1",
					Muted: false,
				},
			},
		},
	}

	display := manager.findDisplay("D1")
	assert.Equal(t, &manager.RoomState.AudioDevices[0], display)

	display2 := manager.findDisplay("D20")
	assert.Nil(t, display2)
}

func TestGroupDisplays(t *testing.T) {
	testState := &AVState{
		Displays: []Display{
			{
				Name:  "D1",
				Power: "on",
				Input: "VIA1",
			},
			{
				Name:  "D2",
				Power: "on",
				Input: "PC1",
			},
			{
				Name:  "D3",
				Power: "on",
				Input: "PC1",
			},
			{
				Name:  "D4",
				Power: "on",
				Input: "VIA1",
			},
		},
		AudioDevices: []AudioDevice{
			{
				Name:  "D1",
				Power: "on",
				Input: "VIA1",
				Muted: false,
			},
			{
				Name:  "D2",
				Power: "on",
				Input: "PC1",
				Muted: false,
			},
			{
				Name:  "D3",
				Power: "on",
				Input: "PC1",
				Muted: false,
			},
			{
				Name:  "D4",
				Power: "on",
				Input: "VIA1",
				Muted: false,
			},
		},
	}

	inputGroups := groupDisplays(testState)
	assert.Equal(t, len(inputGroups), 2, "")
	assert.Equal(t, len(inputGroups["VIA1"]), 2, "")
	assert.Equal(t, len(inputGroups["PC1"]), 2, "")

	testState = &AVState{
		Displays: []Display{
			{
				Name:  "D1",
				Power: "on",
				Input: "VIA1",
			},
			{
				Name:  "D2",
				Power: "on",
				Input: "VIA2",
			},
			{
				Name:  "D3",
				Power: "on",
				Input: "VIA3",
			},
		},
		AudioDevices: []AudioDevice{
			{
				Name:  "D1",
				Power: "on",
				Input: "VIA1",
				Muted: false,
			},
			{
				Name:  "D2",
				Power: "on",
				Input: "VIA2",
				Muted: false,
			},
			{
				Name:  "D3",
				Power: "on",
				Input: "VIA3",
				Muted: false,
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
		Log:                nil,
		RoomID:             "",
		AvApiAddress:       "",
		AudioPriorityCache: make(map[string]string),
	}

	// check if the default lowest number is chosen since the cache is empty
	displays := []string{
		"D4",
		"D2",
		"D3",
	}

	avState := &AVState{
		Displays: []Display{
			{
				Name:  "D4",
				Power: "on",
				Input: "VIA1",
			},
			{
				Name:  "D2",
				Power: "on",
				Input: "VIA1",
			},
			{
				Name:  "D3",
				Power: "on",
				Input: "VIA1",
			},
		},
		AudioDevices: []AudioDevice{
			{
				Name:  "D4",
				Power: "on",
				Input: "VIA1",
				Muted: false,
			},
			{
				Name:  "D2",
				Power: "on",
				Input: "VIA1",
				Muted: false,
			},
			{
				Name:  "D3",
				Power: "on",
				Input: "VIA1",
				Muted: false,
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
		Displays: []Display{
			{
				Name:  "D1",
				Power: "on",
				Input: "VIA1",
			},
			{
				Name:  "D2",
				Power: "on",
				Input: "VIA1",
			},
			{
				Name:  "D3",
				Power: "on",
				Input: "VIA1",
			},
		},
		AudioDevices: []AudioDevice{
			{
				Name:  "D1",
				Power: "on",
				Input: "VIA1",
				Muted: false,
			},
			{
				Name:  "D2",
				Power: "on",
				Input: "VIA1",
				Muted: false,
			},
			{
				Name:  "D3",
				Power: "on",
				Input: "VIA1",
				Muted: false,
			},
		},
	}

	manager.muteDuplicateDisplays("VIA1", displays, avState)
	assert.Equal(t, avState.AudioDevices[0].Muted, true, "")
	assert.Equal(t, avState.AudioDevices[1].Muted, false, "")
	assert.Equal(t, avState.AudioDevices[2].Muted, true, "")

	d := manager.AudioPriorityCache["VIA1"]
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
