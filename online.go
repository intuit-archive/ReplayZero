package main

import (
	"log"

	"github.com/aws/aws-sdk-go/service/kinesis/kinesisiface"
)

// EventChunk contains raw event data + metadata if chunking a large event
type EventChunk struct {
	ChunkNumber int    `json:"chunkNumber"`
	NumChunks   int    `json:"numberOfChunks"`
	UUID        string `json:"uuid"`
	Data        string `json:"data"`
}

type onlineHandler struct {
	client kinesisiface.KinesisAPI
}

func getOnlineHandler() *onlineHandler {
	return &onlineHandler{
		client: buildKinesisClient(flags.streamRoleArn),
	}
}

func (h *onlineHandler) handleEvent(line HTTPEvent) {
	lineStr := httpEventToString(line)
	messages, err := buildMessages(lineStr)
	if err != nil {
		log.Println(err)
	}
	for _, m := range messages {
		log.Println("Sending event with UUID=" + m.UUID)
		err := sendToStream(m, flags.streamName, h.client)
		if err != nil {
			log.Println(err)
		}
	}
	go logUsage(telemetryUsageOnline)
}

// Kinesis: No-op as this handler doesn't buffer anything
func (h *onlineHandler) flushBuffer() {}
