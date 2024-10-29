package app

import "sync"

type svcContext struct {
	wg       sync.WaitGroup
	services []Service
}

type Service interface {
	Start()
	Stop()
}

type SvcContext interface {
	AddService(service Service)

	StartApp()
	StopApp()
}

func NewSvcContext() *svcContext {
	return &svcContext{
		wg: sync.WaitGroup{},
	}
}

func (svc *svcContext) AddService(service Service) {
	svc.services = append(svc.services, service)
}

func (svc *svcContext) StartApp() {
	for _, sv := range svc.services {
		svc.wg.Add(1)
		go func(s Service) {
			defer svc.wg.Done()
			s.Start()
		}(sv)
	}
}

func (svc *svcContext) StopApp() {
	for _, service := range svc.services {
		service.Stop()
	}
	svc.wg.Wait()
}
