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

	"github.com/markbates/pkger"
	flag "github.com/spf13/pflag"

	"github.com/ztrue/shutdown"
)

const (
	verboseCredentialErrors = true
)

var (
	// Version is injected at build time via the '-X' linker flag
	// https://golang.org/cmd/link/#hdr-Command_Line
	Version string

	flags struct {
		version           bool
		listenPort        int
		defaultTargetPort int
		batchSize         int
		template          string
		extension         string
		debug             bool
		streamRoleArn     string
		streamName        string
	}

	client    = &http.Client{}
	telemetry telemetryAgent
)

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

func logWarn(msg string, v ...interface{}) {
	if len(v) > 0 {
		println(msg)
		log.Printf("[WARN] "+msg+"\n", v...)
	} else {
		log.Printf("[WARN] " + msg + "\n")
	}
}

func logDebug(msg string, v ...interface{}) {
	if flags.debug {
		if len(v) > 0 {
			log.Printf("[DEBUG] "+msg+"\n", v...)
		} else {
			log.Printf("[DEBUG] " + msg + "\n")
		}
	}
}

func readFlags() {
	// Overrides the default message of "pflag: help requested" in the case of -h / --help
	flag.ErrHelp = errors.New("")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of replay-zero:\n")
		flag.PrintDefaults()
	}
	flag.BoolVarP(&flags.version, "version", "V", false, "Print version info and exit")
	flag.IntVarP(&flags.listenPort, "listen-port", "l", 9000, "The port the Replay Zero proxy will listen on")
	flag.IntVarP(&flags.defaultTargetPort, "target-port", "p", 8080, "The port the Replay Zero proxy will forward to on localhost")
	flag.BoolVar(&flags.debug, "debug", false, "Set logging to also print debug messages")
	flag.IntVarP(&flags.batchSize, "batch-size", "b", 1, "Buffer events before writing out to a file")
	flag.StringVarP(&flags.template, "template", "t", "karate", "Either [karate] or [gatling] or [path/to/custom/template]")
	flag.StringVarP(&flags.extension, "extension", "e", "", "For custom output template")
	flag.StringVarP(&flags.streamName, "stream-name", "s", "", "AWS Kinesis Stream name (streaming mode only)")
	flag.StringVarP(&flags.streamRoleArn, "stream-role-arn", "r", "", "AWS Kinesis Stream ARN (streaming mode only)")
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

	// check if template exist at the provided path
	// TODO: add validation for correctness of template
	if !(flags.template == "karate" || flags.template == "gatling") {
		_, err := ioutil.ReadFile(flags.template)
		if err != nil {
			log.Printf("Failed to load template")
			panic(err)
		}
		if flags.extension == "" {
			log.Fatal("For custom template, output extension is expected to be passed using --extension or -e flag.")
			os.Exit(0)
		}
	}
}

// get embeded template file content
func getPkgTemplate(filePath string) string {
	f, err := pkger.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		panic(err)
	}
	b := make([]byte, info.Size())
	content, err := f.Read(b)
	if err != nil {
		panic(err)
	}
	return string(b[:content])
}

func getFormat(template string, extension string) outputFormat {
	switch template {
	case "karate":
		return outputFormat{
			template:  getPkgTemplate("/templates/karate_default.template"),
			extension: "feature",
		}
	case "gatling":
		return outputFormat{
			template:  getPkgTemplate("/templates/gatling_default.template"),
			extension: "scala",
		}
	default:
		dat, err := ioutil.ReadFile(template)
		if err != nil {
			panic(err)
		}
		return outputFormat{
			template:  string(dat),
			extension: extension,
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
		if err != nil {
			log.Printf("[ERROR] Could not read response body: %v\n", err)
			return
		}
		_, err = io.Copy(wr, strings.NewReader(string(originalRespBody)))
		if err != nil {
			log.Printf("[ERROR] Could not create copy of response body: %v\n", err)
			return
		}
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
	readFlags()
	telemetry = getTelemetryAgent()
	go telemetry.logUsage(telemetryUsageOpen)

	var h eventHandler
	if len(flags.streamName) > 0 {
		if len(flags.streamRoleArn) == 0 {
			log.Println("AWS Kinesis Stream ARN and name required for streaming mode")
			os.Exit(1)
		}
		log.Println("Running ONLINE, sending recorded events to Kinesis")
		h = getOnlineHandler(flags.streamName, flags.streamRoleArn)
	} else {
		log.Printf("Running OFFLINE, writing out events to %s files\n", flags.template)
		h = getOfflineHandler(flags.template, flags.extension)
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
