package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

type ArchivedRequest struct {
	url     string
	method  string
	headers map[string][]string
	body    []byte

	// Order that the request was issued; expected to be FIFO
	// Allows requests with the same parameters to return in the correct order
	//order int
}

type ArchivedResponse struct {
	// The response mirrors the request on the same URL. Redirects are logged separately
	// and will return a "Location" redirect prompt in the headers here.
	/// response metadata
	//redirected bool
	status  int
	headers map[string][]string
	body    []byte
}

const (
	RecorderModeOff   = iota
	RecorderModeRead  = iota
	RecorderModeWrite = iota
)

type RecordedRecord struct {
	request  ArchivedRequest
	response ArchivedResponse
	//inflightMilliseconds int
}

type Recorder struct {
	mode     int // RecorderModeRead | RecorderModeWrite
	requests []*RecordedRecord
}

func NewRecorder() *Recorder {
	return &Recorder{
		mode:     RecorderModeOff,
		requests: make([]*RecordedRecord, 0),
	}
}

func (r *Recorder) LogPair(request *http.Request, response *http.Response) {
	requestBody, errRequest := io.ReadAll(request.Body)
	responseBody, errResponse := io.ReadAll(response.Body)

	if errRequest != nil || errResponse != nil {
		log.Println("Unable to read body stream.")
		return
	}

	// Allow other clients to consume these bodies again
	request.Body = ioutil.NopCloser(bytes.NewReader(requestBody))
	response.Body = ioutil.NopCloser(bytes.NewReader(responseBody))

	r.requests = append(
		r.requests,
		&RecordedRecord{
			request: ArchivedRequest{
				// last url accessed - how do we get the first
				url:     request.URL.String(),
				method:  request.Method,
				headers: request.Header,
				body:    requestBody,
			},
			response: ArchivedResponse{
				status:  response.StatusCode,
				headers: response.Header,
				body:    responseBody,
			},
		},
	)
}

func (r *Recorder) Print() {
	log.Printf("Total requests: %d", len(r.requests))

	for i := 0; i < len(r.requests); i++ {
		record := r.requests[i]
		log.Printf("Request archive: %s %s (response size: %d)\n", record.request.url, record.request.method, len(record.response.body))
	}
}
