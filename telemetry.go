package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/helper"
)

var (
	gpgPublicKey = ""
)

// events
const (
	telemetryUsageOpen    = iota
	telemetryUsageOnline  = iota
	telemetryUsageOffline = iota

	telemetryEndpoint = "REPLAY_ZERO_TELEMETRY_ENDPOINT"
)

// logInfo contains data to send to the HTTP endpoint.
type logInfo struct {
	Username  string
	Mode      string
	Message   string
	Timestamp int64
}

// getTelemetryEndpoint returns the value of the environment
// variable 'REPLAY_ZERO_TELEMETRY_ENDPOINT'.
func getTelemetryEndpoint() string {
	return os.Getenv(telemetryEndpoint)
}

// logUsage sends usage information - when enabled - to
// an HTTP endpoint. Whether or not this func sends data
// is dependent on the `telemetryEndpoint` global.
func logUsage(event int) {
	if getTelemetryEndpoint() == "" {
		// If telemetry is not enabled, then there's aboslutely nothing
		// that needs to happen here. Immediately return.
		return
	} else if gpgPublicKey == "" {
		contents, err := fetchGpgPublicKey()
		if err != nil {
			logDebug(err.Error())
			return
		}
		gpgPublicKey = contents
	}
	// convert the telemetry events from ints to strings
	eventMessage := ""
	mode := ""
	switch event {
	case telemetryUsageOpen:
		mode = "open"
		eventMessage = "started the app"
	case telemetryUsageOnline:
		mode = "online"
		eventMessage = "recorded data in online mode"
	case telemetryUsageOffline:
		mode = "offline"
		eventMessage = "recorded data in offline mode"
	}
	// construct and send the payload
	info := &logInfo{
		Username:  getCurrentUser(),
		Mode:      mode,
		Message:   eventMessage,
		Timestamp: time.Now().Unix(),
	}
	err := sendLogInfoToRemote(info)
	if err != nil {
		logDebug(fmt.Sprintf("Could not reach the HTTP endpoint: %v\n", err))
	}
}

// getCurrentUser returns the user's login username.
func getCurrentUser() string {
	user, err := user.Current()
	if err != nil {
		logDebug(fmt.Sprintf("Could not get the current user's name: %v\n", err))
		return "(unknown)"
	}
	return user.Username
}

// sendLogInfoToRemote sends a payload of information to the remote
// telemetry HTTP endpoint.
// This func does not check to see if telemetry is enabled or not -
// call `logUsage` for that and to build the payload.
func sendLogInfoToRemote(info *logInfo) error {
	client := &http.Client{}
	data, err := json.Marshal(info)

	logDebug(fmt.Sprintf("Sending data to the telemetry endpoint: %s", data))
	if err != nil {
		return err
	}
	encrypted, err := gpgEncrypt(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", getTelemetryEndpoint(), bytes.NewBuffer(encrypted))
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logDebug(fmt.Sprintf("Telemetry endpoint response code: %s, body: %s", resp.Status, responseBody))
	return resp.Body.Close()
}

func fetchGpgPublicKey() (string, error) {
	logDebug("Fetching encryption key...")
	resp, err := http.Get(getTelemetryEndpoint())
	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	keyContents := string(raw)

	_, err = crypto.NewKeyFromArmored(keyContents)
	if err != nil {
		return "", errors.New("Key material is not armored")
	}

	return keyContents, nil
}

func gpgEncrypt(data []byte) ([]byte, error) {
	stringData := string(data)
	encrypted, err := helper.EncryptMessageArmored(gpgPublicKey, stringData)
	if err != nil {
		return []byte{}, err
	}
	return []byte(encrypted), nil
}
