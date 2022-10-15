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

const (
	RecorderModeOff   = iota
	RecorderModeRead  = iota
	RecorderModeWrite = iota
)

type RecordedRecord struct {
	Request  ArchivedRequest  `json:"request"`
	Response ArchivedResponse `json:"response"`
	//inflightMilliseconds int
}

type Recorder struct {
	mode    int // RecorderModeRead | RecorderModeWrite
	records []*RecordedRecord

	// Indexes of requests that are already consumed
	consumedRecords []int
}

func NewRecorder() *Recorder {
	return &Recorder{
		mode:            RecorderModeOff,
		records:         make([]*RecordedRecord, 0),
		consumedRecords: make([]int, 0),
	}
}

func (r *Recorder) LogPair(request *http.Request, response *http.Response) {
	archivedRequest := requestToArchivedRequest(request)
	archivedResponse := responseToArchivedResponse(response)

	r.records = append(
		r.records,
		&RecordedRecord{
			Request:  *archivedRequest,
			Response: *archivedResponse,
		},
	)
}

func (r *Recorder) ExportData() (response *bytes.Buffer, err error) {
	/*
	 * Formats data in a readable payload, gzipped for space savings
	 */
	log.Printf("Total requests: %d", len(r.records))
	json, err := json.Marshal(r.records)

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

	// Wipe old data
	r.Clear()

	// Load fresh records into the structs
	json.Unmarshal(output, &r.records)

	return nil
}

func (r *Recorder) Clear() {
	r.records = nil
	r.consumedRecords = nil
}

func (r *Recorder) FindMatchingResponse(request *http.Request) *http.Response {
	/*
	 * Given a new request, determine if we have a match in the tape to handle it
	 */
	log.Printf("Record size: %d\n", len(r.records))
	for recordIndex, record := range r.records {
		if record.Request.Url == request.URL.String() {
			// Only allow each request to be played back one time
			if containsInt(r.consumedRecords, recordIndex) {
				log.Printf("Already seen record, continuing: %s\n", request.URL.String())
				continue
			}

			// Don't allow this same record to be played back again
			r.consumedRecords = append(r.consumedRecords, recordIndex)

			// Format the archived response as a full http response
			resp := archivedResponseToResponse(request, &record.Response)
			return resp
		}
	}

	return nil
}

func (r *Recorder) Print() {
	log.Printf("Total requests: %d", len(r.records))

	for _, record := range r.records {
		log.Printf("Request archive: %s %s (response size: %d)\n", record.Request.Url, record.Request.Method, len(record.Response.Body))
	}
}
