package main

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/go-test/deep"
)

const exampleHTTPEventJSON = `{"event_pair_id":"abc123","http_method":"GET","endpoint":"/path/to","req_headers":[{"name":"Authorization","value":"Bearer password"}],"request_body":"{}","resp_headers":[{"name":"Content-Type","value":"application/json"},{"name":"Cookie","value":"key:value"}],"response_body":"Success","http_response_code":"200"}`

var exampleHTTPEvent = HTTPEvent{
	PairID:     "abc123",
	HTTPMethod: "GET",
	Endpoint:   "/path/to",
	ReqHeaders: []Header{
		Header{Name: "Authorization", Value: "Bearer password"},
	},
	ReqBody: "{}",
	RespHeaders: []Header{
		Header{Name: "Content-Type", Value: "application/json"},
		Header{Name: "Cookie", Value: "key:value"},
	},
	RespBody:     "Success",
	ResponseCode: "200",
}

func TestParseHTTPEvent(t *testing.T) {
	data := parseHTTPEvent(exampleHTTPEventJSON)
	if !eventsAreEqual(exampleHTTPEvent, data) {
		diff := deep.Equal(exampleHTTPEvent, data)
		t.Error(diff)
	}
}

func TestHTTPEventToString(t *testing.T) {
	json := httpEventToString(exampleHTTPEvent)
	if json != exampleHTTPEventJSON {
		t.Fatalf("Serialized JSON was not equal to the expected value")
	}
}

func TestRemoveAll(t *testing.T) {
	headers := []Header{
		Header{Name: "a", Value: "b"},
		Header{Name: "c", Value: "d"},
		Header{Name: "e", Value: "f"},
	}
	expected := []Header{
		Header{Name: "a", Value: "b"},
		Header{Name: "c", Value: "d"},
	}
	newHeaders := removeAll(headers, []int{2})
	if !reflect.DeepEqual(newHeaders, expected) {
		t.Fatalf("Trimmed headers list is not the expected value")
	}
}

func TestConvertRequestResponse(t *testing.T) {
	request := http.Request{
		Method: "GET",
		URL: &url.URL{
			Path: "/path/to",
		},
		Header: http.Header{
			"Authorization": []string{"Bearer password"},
		},
	}
	response := http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"Cookie":       []string{"key:value"},
		},
	}
	httpEvent, err := convertRequestResponse(&request, &response, "{}", "Success")
	if err != nil {
		t.Fatalf("Req/resp conversion failed: %v", err)
	}
	expectedHTTPEvent := exampleHTTPEvent
	expectedHTTPEvent.PairID = httpEvent.PairID
	if !eventsAreEqual(expectedHTTPEvent, httpEvent) {
		diff := deep.Equal(expectedHTTPEvent, httpEvent)
		t.Error(diff)
	}
}

func eventsAreEqual(e1, e2 HTTPEvent) bool {
	return e1.PairID == e2.PairID &&
		e1.HTTPMethod == e2.HTTPMethod &&
		e1.Endpoint == e2.Endpoint &&
		e1.ReqBody == e2.ReqBody &&
		e1.RespBody == e2.RespBody &&
		e1.ResponseCode == e2.ResponseCode &&
		headersAreEqual(e1.ReqHeaders, e2.ReqHeaders) &&
		headersAreEqual(e1.RespHeaders, e2.RespHeaders)
}

func headersAreEqual(h1, h2 []Header) bool {
	h1Map := make(map[string]string)
	h2Map := make(map[string]string)

	for _, header := range h1 {
		h1Map[header.Name] = header.Value
	}
	for _, header := range h2 {
		h2Map[header.Name] = header.Value
	}

	for k, v := range h1Map {
		if h2Map[k] != v {
			return false
		}
	}

	return true
}
