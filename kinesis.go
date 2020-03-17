package main

import (
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	uuid "github.com/nu7hatch/gouuid"
)

const (
	defaultRegion = "us-west-2"
	chunkSize     = 1048576 - 200
)

// EventChunk contains raw event data + metadata if chunking a large event
type EventChunk struct {
	ChunkNumber int    `json:"chunkNumber"`
	NumChunks   int    `json:"numberOfChunks"`
	UUID        string `json:"uuid"`
	Data        string `json:"data"`
}

type onlineHandler struct {
	client *kinesis.Kinesis
}

// Serves 2 purposes:
//   1. Allows for dev override with env var
//   2. If falling back to default, const value is not addressable
// 		but AWS SDK needs a string pointer (*string vs. string)
func getStreamName() *string {
	var name string
	if os.Getenv("STREAM_NAME") != "" {
		name = os.Getenv("STREAM_NAME")
	}
	return &name
}

func getRegion() *string {
	var region string
	if os.Getenv("AWS_REGION") != "" {
		region = os.Getenv("AWS_REGION")
	} else {
		region = defaultRegion
	}
	return &region
}

func getVerboseCredentialErrors() *bool {
	copy := verboseCredentialErrors
	return &copy
}

func getOnlineHandler() *onlineHandler {
	userSession := session.Must(session.NewSession(&aws.Config{
		CredentialsChainVerboseErrors: getVerboseCredentialErrors(),
		Region:                        getRegion(),
	}))
	var kclient *kinesis.Kinesis
	log.Printf("Creating Kinesis client")
	// Allows for dev override
	endpoint := os.Getenv("STREAM_ENDPOINT")
	if endpoint != "" {
		log.Printf("Sending unverified traffic to stream endpoint=" + endpoint)
		kclient = kinesis.New(userSession, &aws.Config{
			Endpoint:    &endpoint,
			Credentials: credentials.NewStaticCredentials("x", "x", "x"),
			Region:      getRegion(),
			HTTPClient: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true},
				}},
		})
	} else {
		log.Println("Fetching temp credentials...")
		kinesisTempCreds := stscreds.NewCredentials(userSession, flags.streamRoleArn)
		log.Println("Success!")
		kclient = kinesis.New(userSession, &aws.Config{
			CredentialsChainVerboseErrors: getVerboseCredentialErrors(),
			Credentials:                   kinesisTempCreds,
			Region:                        getRegion(),
		})
	}
	return &onlineHandler{
		client: kclient,
	}
}

func (h *onlineHandler) handleEvent(line HTTPEvent) {
	lineStr := httpEventToString(line)
	messages, err := buildMessages(lineStr)
	if err != nil {
		log.Println(err)
	}
	for _, m := range messages {
		err := sendToStream(m, h.client)
		if err != nil {
			log.Println(err)
		}
	}
	go logUsage(telemetryUsageOnline)
}

func min(i1, i2 int) int {
	if i1 < i2 {
		return i1
	}
	return i2
}

func chunkData(data string, size int) []string {
	chunks := []string{}
	for c := 0; c < len(data); c += size {
		nextChunk := data[c:min(c+size, len(data))]
		chunks = append(chunks, nextChunk)
	}
	return chunks
}

func buildMessages(line string) ([]EventChunk, error) {
	chunks := chunkData(line, chunkSize)
	numChunks := len(chunks)
	messages := []EventChunk{}
	eventUUID, err := uuid.NewV4()
	if err != nil {
		return messages, err
	}
	for chunkID, chunk := range chunks {
		nextMessage := EventChunk{
			ChunkNumber: chunkID,
			NumChunks:   numChunks,
			UUID:        eventUUID.String(),
			Data:        chunk,
		}
		messages = append(messages, nextMessage)
	}
	return messages, nil
}

func sendToStream(message EventChunk, client *kinesis.Kinesis) error {
	dataBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}
	log.Println("Sending event with UUID=" + message.UUID)
	partition := "replay-partition-key-" + time.Now().String()
	response, err := client.PutRecord(&kinesis.PutRecordInput{
		StreamName:   getStreamName(),
		Data:         dataBytes,
		PartitionKey: &partition,
	})
	if err != nil {
		return err
	}
	log.Printf("%+v\n", response)
	return nil
}

// Kinesis: No-op as this handler doesn't buffer anything
func (h *onlineHandler) flushBuffer() {}
