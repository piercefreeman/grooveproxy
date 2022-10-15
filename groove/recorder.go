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

func (r *Recorder) LoadData(fileHandler io.Reader) (err error) {
	gzreader, err := gzip.NewReader(fileHandler)
	if err != nil {
		return err
	}

	output, err := ioutil.ReadAll(gzreader)
	if err != nil {
		return err
	}

	json.Unmarshal(output, &r.requests)

	return nil
}

func (r *Recorder) FindMatchingResponse(request *http.Request) *http.Response {
	/*
	 * Given a new request, determine if we have a match in the tape to handle it
	 */
	// TODO: Implement order of operations for same output
	log.Printf("Record size: %d\n", len(r.requests))
	for _, record := range r.requests {
		if record.Request.Url == request.URL.String() {
			resp := &http.Response{}
			resp.Request = request
			resp.TransferEncoding = request.TransferEncoding
			resp.Header = make(http.Header)
			for key, valueList := range record.Response.Headers {
				for _, value := range valueList {
					resp.Header.Add(key, value)
				}
			}
			resp.StatusCode = record.Response.Status
			resp.Status = http.StatusText(record.Response.Status)
			resp.ContentLength = int64(len(record.Response.Body))
			resp.Body = ioutil.NopCloser(bytes.NewReader(record.Response.Body))
			return resp
		}
	}

	return nil
}

func (r *Recorder) Print() {
	log.Printf("Total requests: %d", len(r.requests))

	for _, record := range r.requests {
		log.Printf("Request archive: %s %s (response size: %d)\n", record.Request.Url, record.Request.Method, len(record.Response.Body))
	}
}
