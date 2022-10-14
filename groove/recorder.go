package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

type ArchivedRequest struct {
	Url     string
	Method  string
	Headers map[string][]string
	Body    []byte

	// Order that the request was issued; expected to be FIFO
	// Allows requests with the same parameters to return in the correct order
	//order int
}

type ArchivedResponse struct {
	// The response mirrors the request on the same URL. Redirects are logged separately
	// and will return a "Location" redirect prompt in the headers here.
	/// response metadata
	//redirected bool
	Status  int
	Headers map[string][]string
	Body    []byte
}

const (
	RecorderModeOff   = iota
	RecorderModeRead  = iota
	RecorderModeWrite = iota
)

type RecordedRecord struct {
	Request  ArchivedRequest
	Response ArchivedResponse
	//inflightMilliseconds int
}

type Recorder struct {
	mode     int // RecorderModeRead | RecorderModeWrite
	requests []*RecordedRecord
}

func NewRecorder() *Recorder {
	return &Recorder{
		//mode:     RecorderModeOff,
		// TODO: Restore default
		mode:     RecorderModeWrite,
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
			Request: ArchivedRequest{
				// last url accessed - how do we get the first
				Url:     request.URL.String(),
				Method:  request.Method,
				Headers: request.Header,
				Body:    requestBody,
			},
			Response: ArchivedResponse{
				Status:  response.StatusCode,
				Headers: response.Header,
				Body:    responseBody,
			},
		},
	)
}

func (r *Recorder) ExportData() (response *bytes.Buffer, err error) {
	/*
	 * Formats data in a readable payload, gzipped for space savings
	 */
	log.Printf("Total requests: %d", len(r.requests))
	json, err := json.Marshal(r.requests)

	if err != nil {
		log.Println("Unable to export json payload")
		return nil, err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(json)
	gz.Close()

	return &buf, nil
}

func (r *Recorder) Print() {
	log.Printf("Total requests: %d", len(r.requests))

	for i := 0; i < len(r.requests); i++ {
		record := r.requests[i]
		log.Printf("Request archive: %s %s (response size: %d)\n", record.Request.Url, record.Request.Method, len(record.Response.Body))
	}
}
