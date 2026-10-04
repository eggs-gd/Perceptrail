// Package app: what the server is as a whole — its version and its services run
// together. A leaf: it imports no module of ours.
package app

import (
	"context"
	"sync"
)

// Service: a part of the server that runs until ctx ends (the import, HTTP, the
// libraries' background work)
type Service interface {
	Start(ctx context.Context)
}

// Services: the server's services, started together, stopped together
type Services struct {
	services []Service
}

func NewServices() *Services { return &Services{} }

func (s *Services) Add(service Service) {
	s.services = append(s.services, service)
}

// Run starts every service and returns once ctx ended and every one has stopped
func (s *Services) Run(ctx context.Context) {
	var running sync.WaitGroup
	for _, service := range s.services {
		running.Go(func() { service.Start(ctx) })
	}
	running.Wait()
}
