package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

type ArchivedRequest struct {
	Url     string              `json:"url"`
	Method  string              `json:"method"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`

	// Order that the request was issued; expected to be FIFO
	// Allows requests with the same parameters to return in the correct order
	//order int
}

type ArchivedResponse struct {
	// The response mirrors the request on the same URL. Redirects are logged separately
	// and will return a "Location" redirect prompt in the headers here.
	/// response metadata
	//redirected bool
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`
}

func requestToArchivedRequest(request *http.Request) *ArchivedRequest {
	requestBody, err := io.ReadAll(request.Body)

	if err != nil {
		log.Println("Unable to read request body stream.")
		return nil
	}

	// Allow other clients to consume these bodies again
	request.Body = ioutil.NopCloser(bytes.NewReader(requestBody))

	return &ArchivedRequest{
		// last url accessed - how do we get the first
		Url:     request.URL.String(),
		Method:  request.Method,
		Headers: request.Header,
		Body:    requestBody,
	}
}

func responseToArchivedResponse(response *http.Response) *ArchivedResponse {
	responseBody, err := io.ReadAll(response.Body)

	if err != nil {
		log.Println("Unable to read response body stream.")
		return nil
	}

	// Allow other clients to consume these bodies again
	response.Body = ioutil.NopCloser(bytes.NewReader(responseBody))

	return &ArchivedResponse{
		Status:  response.StatusCode,
		Headers: response.Header,
		Body:    responseBody,
	}
}

func archivedResponseToResponse(request *http.Request, archivedResponse *ArchivedResponse) *http.Response {
	// Format the archived response as a full http response
	resp := &http.Response{}
	resp.Request = request
	resp.TransferEncoding = request.TransferEncoding
	resp.Header = make(http.Header)
	for key, valueList := range archivedResponse.Headers {
		for _, value := range valueList {
			resp.Header.Add(key, value)
		}
	}
	resp.StatusCode = archivedResponse.Status
	resp.Status = http.StatusText(archivedResponse.Status)
	resp.ContentLength = int64(len(archivedResponse.Body))
	resp.Body = ioutil.NopCloser(bytes.NewReader(archivedResponse.Body))
	return resp
}
