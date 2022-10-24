package main

import (
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"regexp"

	"github.com/google/uuid"
)

type RequestRequiresDefinition struct {
	/*
	 * OR listing of what the request requires to be valid
	 * If RequestRequires is not nil, then the request must match at least one of the following
	 */
	urlRegex      *regexp.Regexp
	resourceTypes []string
}

func NewRequestRequiresDefinition(urlRegex string, resourceTypes []string) (*RequestRequiresDefinition, error) {
	expression, err := regexp.Compile(urlRegex)

	if err != nil {
		return nil, err
	}

	return &RequestRequiresDefinition{
		urlRegex:      expression,
		resourceTypes: resourceTypes,
	}, nil
}

func (definition *RequestRequiresDefinition) IsRequestValid(context *DialerContext) bool {
	/*
	 * Check if the request meets the requirements of this definition
	 */
	request := context.Request
	requestType := context.requestType

	if definition.urlRegex != nil {
		if definition.urlRegex.MatchString(request.URL.String()) {
			log.Printf("Request URL matches regex %s", request.URL.String())
			return true
		}
	}

	if len(definition.resourceTypes) > 0 {
		for _, resourceType := range definition.resourceTypes {
			if requestType == resourceType {
				log.Printf("Resource type matches %s", requestType)
				return true
			}
		}
	}

	log.Println("No request match found")
	return false
}

type ProxyDefinition struct {
	url string

	// Use blank values for no authentication
	username string
	password string
}

type DialerDefinition struct {
	/*
	 * A single distance of a end-host dialer. This dial is responsible for opening
	 * up the actual http.Connection to the open internet.
	 */
	identifier string

	// The highest priority dialer is the one that will be used first
	priority int

	// If provided will try to connect through a 3rd party proxy
	// If nil will use passthrough support from the machine that hosts groove
	proxy *ProxyDefinition

	// Filter of when this dialer should be used
	// If provided will ensure that at least one condition is met
	// If not provided, will always be a candidate
	requestRequires *RequestRequiresDefinition

	Dial func(network, addr string) (net.Conn, error)
}

func NewDialerDefinition(
	priority int,
	proxy *ProxyDefinition,
	requestRequires *RequestRequiresDefinition,
) *DialerDefinition {
	// Allocate the dialer up-front so this is cached in the definition for later use
	var dialer func(network, addr string) (net.Conn, error)

	if proxy != nil {
		if len(proxy.username) > 0 && len(proxy.password) > 0 {
			log.Println("Creating authenticated end proxy dialer...")

			connectReqHandler := func(req *http.Request) {
				log.Printf("Will set basic auth on proxy connect %s %s\n", proxy.username, proxy.password)
				SetBasicAuth(proxy.username, proxy.password, req)
			}
			// This is an unideal dependency to have since the dialer doesn't really relate to the proxy
			// other than forwarding some dial through the proxy's built-in dialer
			dialer = NewConnectDialToProxyWithHandler(proxy.url, connectReqHandler)
		} else {
			log.Println("Creating unauthenticated end proxy dialer...")
			dialer = NewConnectDialToProxyWithHandler(proxy.url, nil)
		}

	} else {
		dialer = net.Dial
	}

	return &DialerDefinition{
		identifier:      uuid.New().String(),
		priority:        priority,
		proxy:           proxy,
		requestRequires: requestRequires,
		Dial:            dialer,
	}
}

func (definition *DialerDefinition) GetHost(request *http.Request) string {
	// Returns the effective "host" for a given request
	// This is just intended to be used as a key for caching attributes about the
	// given connection stream that's opened once we dial
	if definition.proxy != nil {
		url, _ := url.Parse(definition.proxy.url)
		return url.Host
	}

	return request.URL.Host
}

type DialerContext struct {
	/*
	 * Context for an individual dial lifecycle, allows us to keep track of some state
	 * when trying multiple dials in a row
	 */

	// If provided, will filter for dialers that are compatible with this request
	// If blank, will assume that
	Request *http.Request

	// Retrieved request type from the http request, since this can (and will) be manipulated
	// before we send to remote
	requestType string

	// List of DialDefinitions that we have already tried
	attemptedDialIdentifiers []string

	// Remaining dials that are available
	remainingTries int
}

type DialerSession struct {
	/*
	 * Primary object and storage structure for dial generation. Only one of these should
	 * be instantiated per groove instance.
	 */
	DialerDefinitions []*DialerDefinition

	// Attempts allowed in each context to dial a successful connection to the open internet
	// If zero, will try all available dials
	TotalTries int

	// NOTE: In the future this might store some mapping of
	// (dialer definition, host) -> success probabilities
}

func NewDialerSession() *DialerSession {
	return &DialerSession{
		DialerDefinitions: make([]*DialerDefinition, 0),
		TotalTries:        0,
	}
}

func (session *DialerSession) NewDialerContext(request *http.Request) *DialerContext {
	totalTries := session.TotalTries
	if totalTries == 0 {
		totalTries = len(session.DialerDefinitions)
	}

	requestType := request.Header.Get(ProxyResourceType)

	return &DialerContext{
		Request:        request,
		requestType:    requestType,
		remainingTries: totalTries,
	}
}

func (session *DialerSession) candidateDialers(context *DialerContext) []*DialerDefinition {
	/*
	 * Returns a list of dialers that can be used to fulfill the request
	 * param: request - nil if we don't know the request yet, true if we just need to open
	 * 	a connection over the wire for a non-http protocol like for websockets
	 */
	candidateDialers := session.DialerDefinitions

	// If request is provided, attempt to filter for the possible dialers
	if context.Request != nil {
		candidateDialers = filterSlice(
			candidateDialers,
			func(dialer *DialerDefinition) bool {
				return dialer.requestRequires == nil || (dialer.requestRequires != nil && dialer.requestRequires.IsRequestValid(context))
			},
		)
	}

	// Otherwise filter out dials that have been already tried
	candidateDialers = filterSlice(
		candidateDialers,
		func(dialer *DialerDefinition) bool {
			return !contains(context.attemptedDialIdentifiers, dialer.identifier)
		},
	)

	return candidateDialers
}

func (session *DialerSession) NextDialer(context *DialerContext) *DialerDefinition {
	/*
	 * Primary method exposed from the session and the only one that clients should have to utilize
	 * Returns the next allowed dialer to use for the given context, nil if not supported
	 */
	if context.remainingTries <= 0 {
		return nil
	}

	candidateDialers := session.candidateDialers(context)

	// Choose the one with maximum priority
	maxPriority := 0
	for _, dialer := range candidateDialers {
		if dialer.priority > maxPriority {
			maxPriority = dialer.priority
		}
	}

	// Choose one within this larger group
	maxPriorityDialers := filterSlice(
		candidateDialers,
		func(dialer *DialerDefinition) bool {
			return dialer.priority == maxPriority
		},
	)

	if len(maxPriorityDialers) == 0 {
		return nil
	}

	dialer := maxPriorityDialers[rand.Intn(len(maxPriorityDialers))]

	// Decrement the remaining tries
	context.remainingTries -= 1
	context.attemptedDialIdentifiers = append(context.attemptedDialIdentifiers, dialer.identifier)

	return dialer
}
