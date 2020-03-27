package main

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/aws/aws-sdk-go/service/kinesis/kinesisiface"
	uuid "github.com/nu7hatch/gouuid"
)

const (
	defaultRegion        = "us-west-2"
	chunkSize            = 1048576 - 200
	kinesaliteStreamName = "replay-zero-dev"
	kinesaliteEndpoint   = "https://localhost:4567"
)

func getRegion() string {
	var region string
	if os.Getenv("AWS_REGION") != "" {
		region = os.Getenv("AWS_REGION")
	} else {
		region = defaultRegion
	}
	return region
}

type kinesisWrapper struct {
	client kinesisiface.KinesisAPI
	// allows different clients to log certain messages at different levels
	// Ex. Regular Kinesis handler for sending HTTP data is "INFO" (log.Printf)
	// Telemetry Kinesis handler is set at "DEBUG" (logDebug)
	logger func(string, ...interface{})
}

func buildClient(streamName, streamRole string, logFunc func(string, ...interface{})) *kinesisWrapper {
	if streamName == kinesaliteStreamName {
		return buildKinesaliteClient(streamName)
	}
	return buildKinesisClient(streamName, streamRole, logFunc)
}

// Uses STS to assume an IAM role for credentials to write records
// to a real Kinesis stream in AWS
func buildKinesisClient(streamName, streamRole string, logFunc func(string, ...interface{})) *kinesisWrapper {
	log.Printf("Creating AWS Kinesis client")
	userSession := session.Must(session.NewSession(&aws.Config{
		CredentialsChainVerboseErrors: aws.Bool(verboseCredentialErrors),
		Region:                        aws.String(getRegion()),
	}))

	log.Println("Fetching temp credentials...")
	kinesisTempCreds := stscreds.NewCredentials(userSession, streamRole)
	log.Println("Success!")

	kinesisHandle := kinesis.New(userSession, &aws.Config{
		CredentialsChainVerboseErrors: aws.Bool(verboseCredentialErrors),
		Credentials:                   kinesisTempCreds,
		Region:                        aws.String(getRegion()),
	})

	sseEnabled, err := streamHasSSE(streamName, kinesisHandle)
	if err != nil {
		logFunc("Could not determine if SSE is enabled for stream %s: %s", streamName, err)
	} else if !sseEnabled {
		logWarn(fmt.Sprintf("Kinesis stream %s does NOT have Server-Side Encryption (SSE) enabled", streamName))
	}
	return &kinesisWrapper{
		client: kinesisHandle,
		logger: logFunc,
	}
}

func streamHasSSE(streamName string, client kinesisiface.KinesisAPI) (bool, error) {
	streamInfo, err := client.DescribeStream(&kinesis.DescribeStreamInput{
		StreamName: aws.String(streamName),
	})
	return *streamInfo.StreamDescription.EncryptionType == kinesis.EncryptionTypeKms, err
}

// Kinesalite is a lightweight implementation of Kinesis
// useful for development scenarios.
// https://github.com/mhart/kinesalite
func buildKinesaliteClient(streamName string) *kinesisWrapper {
	log.Printf("Creating local Kinesalite client")
	log.Printf("Sending unverified traffic to stream endpoint=" + kinesaliteEndpoint)
	kinesisHandle := kinesis.New(session.Must(session.NewSession()), &aws.Config{
		Endpoint:    aws.String(kinesaliteEndpoint),
		Credentials: credentials.NewStaticCredentials("x", "x", "x"),
		Region:      aws.String(getRegion()),
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true},
			}},
	})

	return &kinesisWrapper{
		client: kinesisHandle,
		logger: log.Printf,
	}
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

func buildMessages(line string) []EventChunk {
	chunks := chunkData(line, chunkSize)
	numChunks := len(chunks)
	messages := []EventChunk{}
	// Multiple chunks need a sort of "group ID"
	eventUUID, err := uuid.NewV4()
	var correlation string
	if err != nil {
		msg := fmt.Sprintf("UUID generation failed: %s\nFalling back to SHA1 of input string for chunk correlation", err)
		logDebug(msg)
		correlation = fmt.Sprintf("%x", sha256.Sum256([]byte(line)))
	} else {
		correlation = eventUUID.String()
	}
	for chunkID, chunk := range chunks {
		nextMessage := EventChunk{
			ChunkNumber: chunkID,
			NumChunks:   numChunks,
			UUID:        correlation,
			Data:        chunk,
		}
		messages = append(messages, nextMessage)
	}
	return messages
}

func (c *kinesisWrapper) sendToStream(message interface{}, stream string) error {
	dataBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}
	partition := "replay-partition-key-" + time.Now().String()
	response, err := c.client.PutRecord(&kinesis.PutRecordInput{
		StreamName:   aws.String(stream),
		Data:         dataBytes,
		PartitionKey: &partition,
	})
	if err != nil {
		return err
	}
	c.logger("%+v\n", response)
	return nil
}
