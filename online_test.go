package main

import (
	"testing"
)

// - - - - - - - - - - - - -
//        UTILITIES
// - - - - - - - - - - - - -

func init() {
	telemetry = &noopTelemetryAgent{}
}

// - - - - - - - - - - - - -
//          TESTS
// - - - - - - - - - - - - -

func TestHandleEvent(t *testing.T) {
	mockKinesis := &mockKinesisClient{}
	testHandler := &onlineHandler{
		kinesisStreamName: "test",
		client:            mockKinesis,
	}

	onlineSampleEvent := generateSampleEvent()
	expectedKinesisCalls := 2
	onlineSampleEvent.ReqBody = randomStringWithLength(1.5 * chunkSize)

	testHandler.handleEvent(onlineSampleEvent)
	if mockKinesis.timesCalled != expectedKinesisCalls {
		t.Errorf("Expected to call Kinesis %d times, but saw %d", expectedKinesisCalls, mockKinesis.timesCalled)
	}
}
