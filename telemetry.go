package main

import (
	"fmt"
	"os"
	"os/user"
	"time"
)

// events
const (
	telemetryUsageOpen    = iota
	telemetryUsageOnline  = iota
	telemetryUsageOffline = iota

	telemetryStreamName = "REPLAY_ZERO_TELEMETRY_STREAM"
	telemetryStreamRole = "REPLAY_ZERO_TELEMETRY_ROLE"
)

type telemetryAgent interface {
	logUsage(event int)
}

// No-op agent in case the proper variables are not set
type nopTelemetryAgent struct{}

// Agent to send telemetry to a Kinesis stream
type kinesisTelemetryAgent struct {
	stream string
	client *kinesisWrapper
}

// logInfo contains data to send to the HTTP endpoint.
type logInfo struct {
	Username  string
	Mode      string
	Message   string
	Timestamp int64
}

// Factory-type builder for telemetry agent, in case there's want / need
// to add different telemetry sinks in the future
func getTelemetryAgent() telemetryAgent {
	streamName := os.Getenv(telemetryStreamName)
	streamRole := os.Getenv(telemetryStreamRole)

	if streamName == "" {
		logDebug("Missing telemetry stream name, returning no-op agent (will not send telemetry)")
		return &nopTelemetryAgent{}
	}
	logDebug("Building Kinesis agent for sending telemetry")
	return &kinesisTelemetryAgent{
		stream: streamName,
		client: buildClient(streamName, streamRole, logDebug),
	}
}

func (agent *nopTelemetryAgent) logUsage(event int) {
	logDebug("Telemetry: NO-OP")
}

// logUsage sends usage information - when enabled - to
// an HTTP endpoint. Whether or not this func sends data
// is dependent on the `telemetryEndpoint` global.
func (agent *kinesisTelemetryAgent) logUsage(event int) {
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
	err := agent.streamTelemetry(info)
	if err != nil {
		logDebug(fmt.Sprintf("Could not send telemetry: %v\n", err))
	} else {
		logDebug("Telemetry log success")
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

// Send a telemetry message to the stream specified by `REPLAY_ZERO_TELEMETRY_STREAM`
// and authorized by the IAM role `REPLAY_ZERO_TELEMETRY_ROLE`
func (agent *kinesisTelemetryAgent) streamTelemetry(info *logInfo) error {
	return agent.client.sendToStream(info, agent.stream)
}
