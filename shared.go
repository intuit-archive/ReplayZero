package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	uuid "github.com/nu7hatch/gouuid"
)

// Header is the JSON representation of a header.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HTTPEvent is the JSON representation of an HTTP event.
type HTTPEvent struct {
	PairID       string   `json:"event_pair_id"`
	HTTPMethod   string   `json:"http_method"`
	Endpoint     string   `json:"endpoint"`
	ReqHeaders   []Header `json:"req_headers"`
	ReqBody      string   `json:"request_body"`
	RespHeaders  []Header `json:"resp_headers"`
	RespBody     string   `json:"response_body"`
	ResponseCode string   `json:"http_response_code"`
}

type eventHandler interface {
	handleEvent(HTTPEvent)
	flushBuffer()
}

func parseHTTPEvent(raw string) HTTPEvent {
	event := HTTPEvent{}
	err := json.Unmarshal([]byte(raw), &event)
	check(err)
	return event
}

func httpEventToString(data HTTPEvent) string {
	s, err := json.Marshal(data)
	check(err)
	return string(s)
}

// https://stackoverflow.com/a/37335777
// Process the largest indices marked for removal first.
// Processing smaller indices could make larger indices
// invalid during later processing
func removeAll(lst []Header, iv []int) []Header {
	sort.Sort(sort.Reverse(sort.IntSlice(iv)))
	for w, i := range iv {
		lst[i] = lst[len(lst)-1-w]
		lst = lst[:len(lst)-1-w]
	}
	return lst
}

// convertRequestResponse converts an HTTP `Request` and `Response` to the
// `HTTPEvent` struct for use in generating test strings.
func convertRequestResponse(request *http.Request, response *http.Response, reqBody, respBody string) (HTTPEvent, error) {
	uuid, err := uuid.NewV4()
	if err != nil {
		return HTTPEvent{}, err
	}
	var requestHeaders []Header
	for key, value := range request.Header {
		requestHeaders = append(requestHeaders, Header{key, strings.Join(value, ",")})
	}
	var responseHeaders []Header
	for key, value := range response.Header {
		responseHeaders = append(responseHeaders, Header{key, strings.Join(value, ",")})
	}
	return HTTPEvent{
		PairID:       uuid.String(),
		HTTPMethod:   request.Method,
		Endpoint:     request.URL.Path,
		ReqHeaders:   requestHeaders,
		ReqBody:      reqBody,
		RespHeaders:  responseHeaders,
		RespBody:     respBody,
		ResponseCode: strconv.Itoa(response.StatusCode),
	}, nil
}
