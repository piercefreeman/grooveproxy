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

	// Optional tape ID to tag this request with a certain tape
	TapeID string `json:"tape_id"`

	//inflightMilliseconds int
}

type Recorder struct {
	/*
	 * Tape recorder and replayer
	 *
	 * A recorder supports multiple concurrent tape writes but only playback from one tape at a time.
	 */
	mode    int // RecorderModeRead | RecorderModeWrite
	records []*RecordedRecord

	// Record indexes that are already consumed
	consumedRecords []*RecordedRecord
}

func NewRecorder() *Recorder {
	return &Recorder{
		mode:            RecorderModeOff,
		records:         make([]*RecordedRecord, 0),
		consumedRecords: make([]*RecordedRecord, 0),
	}
}

func (r *Recorder) LogPair(request *http.Request, requestHeaders *HeaderDefinition, response *http.Response) {
	archivedRequest := requestToArchivedRequest(request)
	archivedResponse := responseToArchivedResponse(response)

	if archivedRequest != nil && archivedResponse != nil {
		r.records = append(
			r.records,
			&RecordedRecord{
				Request:  *archivedRequest,
				Response: *archivedResponse,
				TapeID:   requestHeaders.tapeID,
			},
		)
	}
}

func (r *Recorder) ExportData(tapeID string) (response *bytes.Buffer, err error) {
	/*
	 * Formats data in a readable payload, gzipped for space savings
	 * If tapeID is blank, will export all recorded items
	 * If tapeID is provided, will export items with that tape ID and those with no tape flagged. We include
	 * items with no tape flagged because some requests cannot be tagged with a tape via request interception
	 * because of browser control limitations.
	 */
	var recordsToExport []*RecordedRecord

	if len(tapeID) == 0 {
		recordsToExport = r.records
	} else {
		recordsToExport = filterSlice(r.records, func(record *RecordedRecord) bool {
			return record.TapeID == tapeID || record.TapeID == ""
		})
	}

	log.Printf("Total requests: %d", len(recordsToExport))
	json, err := json.Marshal(recordsToExport)

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

func (r *Recorder) ClearTapeID(tapeID string) {
	/*
	 * Remove all records with the given tape ID
	 */
	r.records = filterSlice(r.records, func(record *RecordedRecord) bool {
		return record.TapeID != tapeID
	})
	r.consumedRecords = filterSlice(r.consumedRecords, func(record *RecordedRecord) bool {
		return record.TapeID != tapeID
	})
}

func (r *Recorder) FindMatchingResponse(request *http.Request, requestHeaders *HeaderDefinition) *http.Response {
	/*
	 * Given a new request, determine if we have a match in the tape to handle it
	 */
	log.Printf("Record size: %d\n", len(r.records))
	for _, record := range r.records {
		// If we are looking for a tape, limit ourselves to just that tape
		// Otherwise we are free to use any matching item if it's not linked to a tape
		if record.TapeID != requestHeaders.tapeID {
			continue
		}

		if record.Request.Url == request.URL.String() {
			// Only allow each request to be played back one time
			if contains(r.consumedRecords, record) {
				log.Printf("Already seen record, continuing: %s\n", request.URL.String())
				continue
			}

			// Don't allow this same record to be played back again
			r.consumedRecords = append(r.consumedRecords, record)

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

			recordResult := recorder.FindMatchingResponse(r, ctx.UserData.(*HeaderDefinition))

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

				recorder.LogPair(request, ctx.UserData.(*HeaderDefinition), response)
				log.Println("Added record log.")
			}

			return response
		},
	)
}
