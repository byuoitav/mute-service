package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"
)

type AVState struct {
	Displays     []Display     `json:"displays"`
	AudioDevices []AudioDevice `json:"audioDevices"`
}

type AudioDevice struct {
	AudioBase
	Power string `json:"power"`
	Input string `json:"input"`
}

type Display struct {
	Name string `json:"name"`
}

// prevent posting power and input when muting displays
type AudioBase struct {
	Name  string `json:"name"`
	Muted bool   `json:"muted"`
}

func (ad AudioDevice) MarshalJSON() ([]byte, error) {
	return json.Marshal(ad.AudioBase)
}

func requestAVState(url string, log *zap.Logger) (*AVState, error) {
	log.Debug("sending request to av-api for room status")
	resp, err := http.Get(url)
	if err != nil {
		log.Error("failed to get room status", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	log.Debug("reading response body")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("failed to read room status response body", zap.Error(err))
		return nil, err
	}

	var roomState AVState

	log.Debug("unmarshaling json body")
	if err = json.Unmarshal(body, &roomState); err != nil {
		log.Error("failed to unmarshal room state from room status response body", zap.Error(err))
		return nil, err
	}

	if roomState.AudioDevices == nil {
		log.Error("no audio devices found in the room")
		return nil, errors.New("no audio devices found in the room")
	}

	//trim audio devices that aren't a display
	for i := 0; i < len(roomState.AudioDevices); i++ {
		if _, err := parseDisplayNumber(roomState.AudioDevices[i].Name); err != nil {
			roomState.AudioDevices = append(roomState.AudioDevices[:i], roomState.AudioDevices[i+1:]...)
			i--
		}
	}

	return &roomState, nil
}

func updateAVState(url string, state *AVState, log *zap.Logger) error {
	body, _ := json.Marshal(state)

	client := &http.Client{}

	log.Debug("sending request to av-api to update room state")
	request, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(body))
	if err != nil {
		log.Error("failed to create http request to update room state")
		return err
	}

	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := client.Do(request)
	if err != nil {
		log.Error("failed to send av-api request to update room state")
		return err
	}
	defer resp.Body.Close()

	log.Debug("checking response status")
	if resp.StatusCode != http.StatusOK {
		log.Error("av-api request failed, recived a non 200 status code")
		return fmt.Errorf("av-api request failed, recived a non 200 status code")
	}

	log.Debug("successfully sent state update request")
	return nil
}
