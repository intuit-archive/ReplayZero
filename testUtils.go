package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/aws/aws-sdk-go/service/kinesis/kinesisiface"
)

// - - - - - - - - - - - - -
//        DATA MOCKS
// - - - - - - - - - - - - -

// Credit: https://stackoverflow.com/a/22892986
var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randomStringWithLength(size int) string {
	b := make([]rune, size)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func generateSampleEvent() HTTPEvent {
	return HTTPEvent{
		PairID:     "c1487b92-01a0-4b08-b66d-52c597e88e67",
		HTTPMethod: "POST",
		Endpoint:   "/test/api",
		ReqHeaders: []Header{
			Header{"User-Agent", "curl/7.54.0"},
			Header{"Accept", "*/*"},
			Header{"Content-Length", "22"},
		},
		ReqBody: "this is a test payload",
		RespHeaders: []Header{
			Header{"X-Real-Server", "test.server"},
			Header{"Content-Length", "22"},
			Header{"Date", "Date: Tue, 18 Feb 2020 20:42:12 GMT"},
		},
		RespBody:     "Test payload back atcha",
		ResponseCode: "200",
	}
}

var sampleEvent = generateSampleEvent()

// - - - - - - - - - - - - -
//        AWS MOCKS
// - - - - - - - - - - - - -

type mockKinesisClient struct {
	timesCalled int
	kinesisiface.KinesisAPI
}

func (m *mockKinesisClient) DescribeStream(inp *kinesis.DescribeStreamInput) (*kinesis.DescribeStreamOutput, error) {
	m.timesCalled++
	if *inp.StreamName == "simulate_empty_response" {
		return &kinesis.DescribeStreamOutput{}, nil
	} else {
		return &kinesis.DescribeStreamOutput{
			StreamDescription: &kinesis.StreamDescription{
				StreamName:     aws.String("test-stream"),
				EncryptionType: aws.String(kinesis.EncryptionTypeKms),
			},
		}, nil
	}
}

func (m *mockKinesisClient) PutRecord(inp *kinesis.PutRecordInput) (*kinesis.PutRecordOutput, error) {
	m.timesCalled++
	// Used to deterministically simulate an error in the AWS SDK
	// See `TestSendToStreamKinesisError` for usage
	if *inp.StreamName == "simulate_error" {
		return &kinesis.PutRecordOutput{}, fmt.Errorf("simulated service error")
	}
	return &kinesis.PutRecordOutput{
		SequenceNumber: aws.String("a"),
		ShardId:        aws.String("b"),
	}, nil
}

// - - - - - - - - - - - - -
//        I/O MOCKS
// - - - - - - - - - - - - -

// Writes all data to devNull & succeeds
// https://golang.org/pkg/io/ioutil/#pkg-variables
func emptyWriter(h *offlineHandler) io.Writer {
	return ioutil.Discard
}

// Accepts a log message and does nothing with it
func nopLog(msg string, v ...interface{}) {}
