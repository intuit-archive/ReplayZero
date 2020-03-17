package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"

	flag "github.com/spf13/pflag"

	"github.com/intuit/replay-zero/templates"
	"github.com/ztrue/shutdown"
)

const (
	verboseCredentialErrors = true
)

// Version is injected at build time via the '-X' linker flag
// https://golang.org/cmd/link/#hdr-Command_Line
var Version string

var flags struct {
	version           bool
	listenPort        int
	defaultTargetPort int
	batchSize         int
	output            string
	debug             bool
	streamRoleArn     string
	streamName        string
}

var client = &http.Client{}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func logErr(err error) {
	if err != nil {
		log.Println(err)
	}
}

func logDebug(msg string) {
	if flags.debug {
		log.Printf("[DEBUG] %s\n", msg)
	}
}

func readFlags() {
	// Overrides the default message of "pflag: help requested" in the case of -h / --help
	flag.ErrHelp = errors.New("")
	flag.BoolVarP(&flags.version, "version", "V", false, "Print version info and exit")
	flag.IntVarP(&flags.listenPort, "listen-port", "l", 9000, "The port the Replay Zero proxy will listen on")
	flag.IntVarP(&flags.defaultTargetPort, "target-port", "t", 8080, "The port the Replay Zero proxy will forward to on localhost")
	flag.BoolVar(&flags.debug, "debug", false, "Set logging to also print debug messages")
	flag.IntVarP(&flags.batchSize, "batch-size", "b", 1, "Buffer events before writing out to a file")
	flag.StringVarP(&flags.output, "output", "o", "karate", "Either [karate] or [gatling]")
	flag.StringVarP(&flags.streamName, "streamName", "s", "", "AWS Kinesis Stream name (streaming mode only)")
	flag.StringVarP(&flags.streamRoleArn, "streamRoleArn", "r", "", "AWS Kinesis Stream ARN (streaming mode only)")
	flag.Parse()

	if flags.version {
		fmt.Printf("replay-zero version %s\n", Version)
		os.Exit(0)
	}

	if flags.batchSize == 0 {
		log.Println("Batch size cannot be zero! Using batch=1")
		flags.batchSize = 1
	} else if flags.batchSize > 1 {
		log.Printf("Using fixed batch size of %d\n", flags.batchSize)
	}
}

func getFormat(output string) outputFormat {
	switch output {
	case "karate":
		return outputFormat{
			template:  templates.KarateBase,
			extension: "feature",
		}
	case "gatling":
		return outputFormat{
			template:  templates.GatlingBase,
			extension: "scala",
		}
	default:
		log.Printf("Unknown output '%s', defaulting to karate", output)
		return outputFormat{
			template:  templates.KarateBase,
			extension: "feature",
		}
	}
}

func buildNewTargetURL(req *http.Request) string {
	scheme := req.URL.Scheme
	if scheme == "" {
		scheme = "http"
	}

	host := req.URL.Host
	if host == "" {
		host = fmt.Sprintf("localhost:%d", flags.defaultTargetPort)
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, req.URL.Path)
}

// A higher-order function that accepts an HTTPEvent handler and
// returns a network interceptor that passes parses + builds an HTTPEvent
// and passes that on to the parametrized handler for further processing.
func createServerHandler(h eventHandler) func(http.ResponseWriter, *http.Request) {
	return func(wr http.ResponseWriter, originalRequest *http.Request) {
		// 1. Construct proxy request
		newURL := buildNewTargetURL(originalRequest)
		defer originalRequest.Body.Close()
		originalBody, err := ioutil.ReadAll(originalRequest.Body)
		if err != nil {
			log.Printf("[ERROR] Could not read original request body: %v\n", err)
			return
		}
		originalBodyString := string(originalBody)
		request, err := http.NewRequest(originalRequest.Method, newURL, strings.NewReader(originalBodyString))
		if err != nil {
			log.Printf("[ERROR] Could not build the outgoing request: %v\n", err)
			return
		}
		request.Header = originalRequest.Header

		// 2. Execute proxy request
		response, err := client.Do(request)
		if err != nil {
			log.Printf("[ERROR] Could not process HTTP request to target: %v\n", err)
			return
		}

		// 3. Copy data for proxy response
		for k, v := range response.Header {
			wr.Header().Set(k, strings.Join(v, ","))
		}
		wr.WriteHeader(response.StatusCode)
		defer response.Body.Close()
		originalRespBody, err := ioutil.ReadAll(response.Body)
		io.Copy(wr, strings.NewReader(string(originalRespBody)))
		originalRespBodyString := string(originalRespBody)
		// 4. Parse request + response data and pass on to event handler
		event, err := convertRequestResponse(request, response, originalBodyString, originalRespBodyString)
		if err != nil {
			log.Printf("[ERROR] Could not convert request and response: %v\n", err)
			return
		}

		log.Printf("Saw event:\n%s %s %s", event.PairID, event.HTTPMethod, event.Endpoint)
		h.handleEvent(event)
	}
}

func main() {
	go logUsage(telemetryUsageOpen)
	readFlags()

	var h eventHandler
	if len(flags.streamName) > 0 {
		if len(flags.streamRoleArn) == 0 {
			log.Println("AWS Kinesis Stream ARN and name required for streaming mode")
			os.Exit(1)
		}
		log.Println("Running ONLINE, sending recorded events to Kinesis")
		h = getOnlineHandler(flags.streamName, flags.streamRoleArn)
	} else {
		log.Printf("Running OFFLINE, writing out events to %s files\n", flags.output)
		h = getOfflineHandler(flags.output)
	}

	shutdown.Add(func() {
		log.Println("Cleaning up...")
		h.flushBuffer()
	})

	http.HandleFunc("/", createServerHandler(h))
	listenAddr := fmt.Sprintf("localhost:%d", flags.listenPort)
	log.Println("Proxy listening on " + listenAddr)
	go func() {
		log.Fatal(http.ListenAndServe(listenAddr, nil))
	}()

	shutdown.Listen(syscall.SIGINT)
}
