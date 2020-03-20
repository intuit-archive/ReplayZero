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
	kinesisStreamName string
	client            kinesisiface.KinesisAPI
}

func getOnlineHandler(streamName, streamRole string) *onlineHandler {
	return &onlineHandler{
		kinesisStreamName: streamName,
		client:            buildClient(streamName, streamRole),
	}
}

func (h *onlineHandler) handleEvent(line HTTPEvent) {
	go telemetry.logUsage(telemetryUsageOnline)
	lineStr := httpEventToString(line)
	messages := buildMessages(lineStr)
	for _, m := range messages {
		log.Println("Sending event with UUID=" + m.UUID)
		err := sendToStream(m, flags.streamName, h.client)
		if err != nil {
			log.Println(err)
		}
	}
}

// Kinesis: No-op as this handler doesn't buffer anything
func (h *onlineHandler) flushBuffer() {}
