package main

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"testing"

	"github.com/kylelemons/godebug/diff"
	"github.com/intuit/replay-zero/templates"
)

// - - - - - - - - - - - - -
//        UTILITIES
// - - - - - - - - - - - - -

var sampleEvent = HTTPEvent{
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

// Writes all data to devNull & succeeds
// https://golang.org/pkg/io/ioutil/#pkg-variables
func emptyWriter(h *offlineHandler) io.Writer {
	return ioutil.Discard
}

// - - - - - - - - - - - - -
//          TESTS
// - - - - - - - - - - - - -

func TestOfflineHandleEventNoFlush(t *testing.T) {
	handler := offlineHandler{
		defaultBatchSize: 2,
	}
	handler.handleEvent(exampleHTTPEvent)
	if len(handler.buffer) != 1 {
		t.Errorf("Events in buffer should be 1, got %d", len(handler.buffer))
	}
	if handler.numWrites != 0 {
		t.Errorf("Number of writes should be 0, got %d", handler.numWrites)
	}
}

func TestOfflineHandleEventFlushBadTemplate(t *testing.T) {
	var buff bytes.Buffer
	handler := offlineHandler{
		format: outputFormat{
			template: `{{{`,
		},
		currentBatchSize: 1,
		writerFactory: func(h *offlineHandler) io.Writer {
			return bufio.NewWriter(&buff)
		},
	}
	handler.handleEvent(exampleHTTPEvent)
	if len(handler.buffer) != 0 {
		t.Errorf("Events in buffer should be 0, got %d", len(handler.buffer))
	}
	if handler.numWrites != 0 {
		t.Errorf("Number of writes should be 0 (should have an error), got %d", handler.numWrites)
	}
	length := len(buff.String())
	if length != 0 {
		t.Errorf("Buffer should be empty, saw length of %d", length)

	}
}

func TestOfflineReadReplayHeaders(t *testing.T) {
	handler := offlineHandler{}
	newHeaders := handler.readReplayHeaders([]Header{
		Header{Name: "Replay_batch", Value: "2"},
	})
	if handler.currentBatchSize != 2 {
		t.Fatalf("Current batch size should be 2, got %d", handler.currentBatchSize)
	}
	if len(newHeaders) != 0 {
		t.Fatalf("New headers size should be 0, got %d", len(newHeaders))
	}
}

func TestReadBadBatchSize(t *testing.T) {
	defaultSize := 1
	originalCurrent := 2
	handler := offlineHandler{
		defaultBatchSize: defaultSize,
		currentBatchSize: originalCurrent,
	}

	// Tests non-numeric batch size which changes nothing.
	// Header should still not be present in resulting array
	out := handler.readReplayHeaders([]Header{Header{Name: "Replay_batch", Value: "aaa"}})
	if len(out) != 0 {
		t.Errorf("Array should have length 0 but was %d", len(out))
	}
	if handler.currentBatchSize != originalCurrent {
		t.Errorf("Curent batch size should be %d but was %d", originalCurrent, handler.currentBatchSize)
	}

	// Tests zero batch size which should fall back to default batch size
	out = handler.readReplayHeaders([]Header{Header{Name: "Replay_batch", Value: "0"}})
	if len(out) != 0 {
		t.Errorf("Array should have length 0 but was %d", len(out))
	}
	if handler.currentBatchSize != defaultSize {
		t.Errorf("Curent batch size should be %d but was %d", defaultSize, handler.currentBatchSize)
	}
}

// Table-driven test for validating all templates
func TestVerifyTemplates(t *testing.T) {
	testFuncMap := templates.DefaultFuncMap()
	testFuncMap["now"] = func() string {
		return "18 Feb 20 12:22 PST"
	}

	var templateTests = []struct {
		name     string
		template string
		expected string
	}{
		{"karate", templates.KarateBase, testKarateExpected},
		{"gatling", templates.GatlingBase, testGatlingExpected},
	}

	for _, tt := range templateTests {
		t.Run(tt.name, func(t *testing.T) {
			var buff bytes.Buffer
			buffWriter := bufio.NewWriter(&buff)
			handler := &offlineHandler{
				format: outputFormat{
					template: tt.template,
				},
				// Making sure to test that multiple scenarios don't bunch up against each other
				buffer: []HTTPEvent{sampleEvent, sampleEvent},
				writerFactory: func(h *offlineHandler) io.Writer {
					return buffWriter
				},
				templateFuncMap: testFuncMap,
			}
			err := handler.runTemplate()
			if err != nil {
				t.Fatal(err)
			}
			if err := buffWriter.Flush(); err != nil {
				t.Error(err)
			}

			actual := buff.String()
			difference := diff.Diff(tt.expected, actual)
			if len(difference) != 0 {
				t.Error("Got unexpected output from template!")
				t.Error(difference)
			}
		})
	}
}

func TestRunTemplateError(t *testing.T) {
	handler := &offlineHandler{
		format: outputFormat{
			template: `{{{`,
		},
		writerFactory: emptyWriter,
	}

	err := handler.runTemplate()
	if err == nil {
		t.Error("Expected error but got nil")
	}
}

func TestResetHandler(t *testing.T) {
	handler := offlineHandler{
		defaultBatchSize: 1,
		writerFactory:    emptyWriter,
	}
	handler.buffer = append(handler.buffer, HTTPEvent{})
	handler.currentBatchSize = 2

	if handler.writerFactory == nil {
		t.Fatal("Handler's event writer must be non-empty for this test")
	}
	handler.flushBuffer()
	if len(handler.buffer) != 0 {
		log.Fatal("Buffer should be empty")
	}
	if len(handler.buffer) != 0 {
		log.Fatalf("Events in buffer should be 0, got %d", len(handler.buffer))
	}
	if handler.currentBatchSize != 1 {
		log.Fatalf("Current batch size should be 1, not %d", handler.currentBatchSize)
	}
}
