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

// Serves 2 purposes:
//   1. Allows for dev override with env var
//   2. If falling back to default, const value is not addressable
// 		but AWS SDK needs a string pointer (*string vs. string)
func getStreamName() string {
	var name string
	if os.Getenv("STREAM_NAME") != "" {
		name = os.Getenv("STREAM_NAME")
	}
	return name
}

func getRegion() string {
	var region string
	if os.Getenv("AWS_REGION") != "" {
		region = os.Getenv("AWS_REGION")
	} else {
		region = defaultRegion
	}
	return region
}

func buildKinesisClient() *kinesis.Kinesis {
	userSession := session.Must(session.NewSession(&aws.Config{
		CredentialsChainVerboseErrors: aws.Bool(verboseCredentialErrors),
		Region:                        aws.String(getRegion()),
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
			Region:      aws.String(getRegion()),
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
			CredentialsChainVerboseErrors: aws.Bool(verboseCredentialErrors),
			Credentials:                   kinesisTempCreds,
			Region:                        aws.String(getRegion()),
		})
	}
	return kclient
}

func chunkData(data string, size int) []string {
	chunks := []string{}
	for c := 0; c < len(data); c += size {
		nextChunk := data[c:min(c+size, len(data))]
		chunks = append(chunks, nextChunk)
	}
	return chunks
}

func min(i1, i2 int) int {
	if i1 < i2 {
		return i1
	}
	return i2
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

func sendToStream(message interface{}, client *kinesis.Kinesis) error {
	dataBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}
	partition := "replay-partition-key-" + time.Now().String()
	response, err := client.PutRecord(&kinesis.PutRecordInput{
		StreamName:   aws.String(getStreamName()),
		Data:         dataBytes,
		PartitionKey: &partition,
	})
	if err != nil {
		return err
	}
	log.Printf("%+v\n", response)
	return nil
}
