package app

import (
	"context"
	"sync"
)

type svcContext struct {
	wg       sync.WaitGroup
	services []Service
}

type Service interface {
	Start(context.Context)
}

type SvcContext interface {
	AddService(Service)
	RunApp(context.Context)
}

func NewSvcContext() *svcContext {

	return &svcContext{
		wg: sync.WaitGroup{},
	}
}

func (svc *svcContext) AddService(service Service) {
	svc.services = append(svc.services, service)
}

func (svc *svcContext) RunApp(parentCtx context.Context) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	for _, sv := range svc.services {
		svc.wg.Add(1)
		go func(s Service) {
			defer svc.wg.Done()
			s.Start(ctx)
		}(sv)
	}

	<-ctx.Done()
	svc.wg.Wait()
}
