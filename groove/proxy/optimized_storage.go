// https://github.com/elazarl/goproxy/blob/ac903b0f516b4b07599ab573d837cbaccb26feba/examples/goproxy-certstorage/optimized_storage.go
// Has per-host locks to avoid situation, when multiple concurrent requests to a host without ready to use certificate will
// generate the same certificate multiple times.
package main

import (
	"crypto/tls"
	"fmt"
	"sync"
)

type OptimizedCertStore struct {
	certs map[string]*tls.Certificate
	locks map[string]*sync.Mutex
	sync.Mutex
}

func NewOptimizedCertStore() *OptimizedCertStore {
	return &OptimizedCertStore{
		certs: map[string]*tls.Certificate{},
		locks: map[string]*sync.Mutex{},
	}
}

func (s *OptimizedCertStore) Fetch(host string, genCert func() (*tls.Certificate, error)) (*tls.Certificate, error) {
	fmt.Printf("Fetching certificate for %s\n", host)

	hostLock := s.hostLock(host)
	hostLock.Lock()
	defer hostLock.Unlock()

	cert, ok := s.certs[host]
	var err error
	if !ok {
		fmt.Printf("cache miss: %s\n", host)

		cert, err = genCert()
		if err != nil {
			return nil, err
		}
		s.certs[host] = cert
	} else {
		fmt.Printf("cache hit: %s\n", host)
	}
	return cert, nil
}

func (s *OptimizedCertStore) hostLock(host string) *sync.Mutex {
	// Only one host lock should be generated at one time
	s.Lock()
	defer s.Unlock()

	lock, ok := s.locks[host]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[host] = lock
	}
	return lock
}
