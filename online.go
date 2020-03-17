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
		client:            buildKinesisClient(streamRole),
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

func (h *onlineHandler) streamEvent(event EventChunk) error {
	return sendToStream(event, h.kinesisStreamName, h.client)
}

// Kinesis: No-op as this handler doesn't buffer anything
func (h *onlineHandler) flushBuffer() {}
