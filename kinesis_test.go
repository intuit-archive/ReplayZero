package main

import (
	"log"
	"os"
	"reflect"
	"testing"
)

func TestChunk(t *testing.T) {
	testText := "apple"
	expectedChunks := []string{"ap", "pl", "e"}

	actualChunks := chunkData(testText, 2)

	if len(actualChunks) != len(expectedChunks) {
		t.Errorf("Expected %d chunks, but got %d", len(expectedChunks), len(actualChunks))
	}

	for ind := range actualChunks {
		if actualChunks[ind] != expectedChunks[ind] {
			t.Errorf("index=%d:\nExpected chunk=%s, but got chunk=%s", ind, expectedChunks[ind], actualChunks[ind])
		}
	}
}

func TestGetRegionOverride(t *testing.T) {
	err := os.Setenv("AWS_REGION", "")
	if err != nil {
		t.Fatalf("Could not set environment variable for test: %v\n", err)
	}
	defaultName := getRegion()
	expectedDefault := "us-west-2"
	if defaultName != expectedDefault {
		t.Errorf("Expected default region=%s, actual=%s", expectedDefault, defaultName)
	}

	err = os.Setenv("AWS_REGION", "foobar")
	if err != nil {
		t.Errorf("Could not set env var: %v", err)
	}
	name := getRegion()
	if name != "foobar" {
		t.Errorf("Stream name was %s", name)
	}
}

func TestStreamHasSSEError(t *testing.T) {
	mockKinesis := &mockKinesisClient{}

	hasSSE, err := streamHasSSE("simulate_empty_response", mockKinesis)
	if err == nil {
		t.Errorf("Expected error, but got <nil>")
	}
	if hasSSE {
		t.Errorf("Expected 'hasSSE' == FALSE, but was TRUE")
	}

	hasSSE, err = streamHasSSE("success", mockKinesis)
	if err != nil {
		t.Errorf("Expected no error, but got %s", err)
	}
	if !hasSSE {
		t.Errorf("Expected 'hasSSE' == TRUE, but was FALSE")
	}
}

func TestBuildMessages(t *testing.T) {
	expected := EventChunk{
		ChunkNumber: 0,
		NumChunks:   1,
		UUID:        "",
		Data:        "foobar",
	}
	messages := buildMessages("foobar")
	if len(messages) != 1 {
		log.Fatalf("Expected 1 message, got %d", len(messages))
	}
	first := messages[0]
	expected.UUID = first.UUID
	if !reflect.DeepEqual(first, expected) {
		log.Fatalf("Kinesis message is not the expected, got %v", first)
	}
}

func TestSendToStreamMarshalError(t *testing.T) {
	mockKinesis := &mockKinesisClient{}
	wrapper := &kinesisWrapper{client: mockKinesis}

	// json.Marshal can't Marshal certain types, like channels
	err := wrapper.sendToStream(make(chan int), "test")
	if mockKinesis.timesCalled > 0 {
		t.Error("Expected mock Kinesis client not to be called, but it was")
	}
	if err == nil {
		t.Error("Expected an error, but got <nil>")
	}
}

func TestSendToStreamKinesisError(t *testing.T) {
	mockKinesis := &mockKinesisClient{}
	wrapper := &kinesisWrapper{client: mockKinesis}

	err := wrapper.sendToStream(`{"data": "test"}`, "simulate_error")
	if mockKinesis.timesCalled == 0 {
		t.Error("Expected mock Kinesis client to be called, but it was NOT")
	}
	if err == nil {
		t.Errorf("Expected error, but got <nil>")
	}
}

func TestSendToStreamKinesisSuccess(t *testing.T) {
	mockKinesis := &mockKinesisClient{}
	wrapper := &kinesisWrapper{
		client: mockKinesis,
		logger: nopLog,
	}

	err := wrapper.sendToStream(`{"data": "test"}`, "test")
	if mockKinesis.timesCalled == 0 {
		t.Error("Expected mock Kinesis client to be called, but it was NOT")
	}
	if err != nil {
		t.Errorf("Expected no error, but got %s", err)
	}
}

func TestFlushKinesisBuffer(t *testing.T) {
	wrapper := onlineHandler{}
	wrapper.flushBuffer()
}
