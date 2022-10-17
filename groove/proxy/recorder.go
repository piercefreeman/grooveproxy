package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	goproxy "github.com/piercefreeman/goproxy"
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

func setupRecorderMiddleware(proxy *goproxy.ProxyHttpServer, recorder *Recorder) {
	proxy.OnRequest().DoFunc(
		/*
		 * Recorder
		 */
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			// Only handle responses during write mode
			log.Printf("Recorder get mode... %d\n", recorder.mode)
			if recorder.mode != RecorderModeRead {
				return r, nil
			}

			recordResult := recorder.FindMatchingResponse(r)

			if recordResult != nil {
				log.Printf("Record found: %s\n", r.URL.String())
				return r, recordResult
			} else {
				log.Printf("No matching record found: %s\n", r.URL.String())
				// Implementation specific - for now fail result if we can't find a
				// playback entry in the tape
				return r, goproxy.NewResponse(
					r,
					goproxy.ContentTypeText,
					http.StatusInternalServerError,
					"Proxy blocked request",
				)
			}

			// Passthrough
			//return r, nil
		})

	proxy.OnResponse().DoFunc(
		/*
		 * Recorder
		 */
		func(response *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			// Only handle responses during write mode
			if recorder.mode != RecorderModeWrite {
				return response
			}

			log.Println("Record request/response...")
			requestHistory, responseHistory := getRedirectHistory(response)

			// When replaying this we want to replay it in order to capture all the
			// event history and test redirect handlers
			for i := 0; i < len(requestHistory); i++ {
				request := requestHistory[i]
				response := responseHistory[i]

				recorder.LogPair(request, response)
				log.Println("Added record log.")
			}

			return response
		},
	)
}
