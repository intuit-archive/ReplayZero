package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/intuit/replay-zero/templates"
)

const (
	outDir = "."
)

type writerFactory func(*offlineHandler) io.Writer

type outputFormat struct {
	template  string
	extension string
}

type offlineHandler struct {
	format           outputFormat
	buffer           []HTTPEvent
	defaultBatchSize int
	currentBatchSize int
	numWrites        int
	writerFactory    writerFactory
	templateFuncMap  template.FuncMap
}

func getOfflineHandler(output string) eventHandler {
	return &offlineHandler{
		format:           getFormat(output),
		defaultBatchSize: flags.batchSize,
		currentBatchSize: flags.batchSize,
		writerFactory:    getFileWriter,
		templateFuncMap:  templates.DefaultFuncMap(),
	}
}

func (h *offlineHandler) handleEvent(logEvent HTTPEvent) {
	go telemetry.logUsage(telemetryUsageOffline)
	logEvent.ReqHeaders = h.readReplayHeaders(logEvent.ReqHeaders)
	h.buffer = append(h.buffer, logEvent)

	if len(h.buffer) == h.currentBatchSize {
		h.flushBuffer()
	}
}

func (h *offlineHandler) readReplayHeaders(headers []Header) []Header {
	toRemove := []int{}
	for i, header := range headers {
		headerParts := strings.Split(header.Name, "Replay_")
		if len(headerParts) != 2 {
			continue
		}
		// all special "replay_" headers should be removed
		// before the data is persisted in any way
		toRemove = append(toRemove, i)
		switch headerParts[1] {
		case "batch":
			dynamicBatchSize, err := strconv.Atoi(header.Value)
			if err != nil {
				log.Println("Unable to parse dynamic batch size: " + header.Value)
				continue
			} else if dynamicBatchSize == 0 {
				h.flushBuffer()
				log.Printf("Batch size cannot be zero! Using default batch=%d\n", h.defaultBatchSize)
			} else {
				h.flushBuffer()
				log.Printf("Detected dynamic batch size with size %d\n", dynamicBatchSize)
				h.currentBatchSize = dynamicBatchSize
			}
		}
	}

	return removeAll(headers, toRemove)
}

func (h *offlineHandler) getNextFileName() string {
	return fmt.Sprintf("replay_scenarios_%d.%s", h.numWrites, h.format.extension)
}

func getFileWriter(h *offlineHandler) io.Writer {
	if h.format.extension == "" {
		log.Println("[ERROR] File extension is empty, not writing file")
		return nil
	}
	fileName := h.getNextFileName()
	out := filepath.Join(outDir, fileName)
	f, err := os.Create(out)
	if err != nil {
		logErr(err)
		return nil
	}
	return f
}

func (h *offlineHandler) runTemplate() error {
	t, err := template.New("").Funcs(h.templateFuncMap).Parse(h.format.template)
	if err != nil {
		return err
	}

	return t.Execute(h.writerFactory(h), h.buffer)
}

// KarateGen: Write out buffered events in the case of a user-enacted exit (i.e. ctrl+c)
func (h *offlineHandler) flushBuffer() {
	numEvents := len(h.buffer)
	if numEvents > 0 {
		log.Println("Flushing buffer...")
		err := h.runTemplate()
		if err == nil {
			suffix := ""
			if numEvents > 1 {
				suffix = "s"
			}
			log.Printf("Wrote %d scenario%s to file %s\n", numEvents, suffix, h.getNextFileName())
			h.numWrites++
		} else {
			logErr(err)
		}
	}
	h.resetBuffer()
}

func (h *offlineHandler) resetBuffer() {
	// https://stackoverflow.com/a/16973160
	h.buffer = nil
	h.currentBatchSize = h.defaultBatchSize
}
