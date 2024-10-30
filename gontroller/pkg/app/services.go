package app

import (
	"context"
	"sync"
)

type svcContext struct {
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	services []Service
}

type Service interface {
	Start(context.Context)
	Stop()
}

type SvcContext interface {
	AddService(Service)

	StartApp()
	StopApp()
}

func NewSvcContext(ctx context.Context) *svcContext {
	cntx, cancel := context.WithCancel(ctx)

	return &svcContext{
		ctx:    cntx,
		cancel: cancel,
		wg:     sync.WaitGroup{},
	}
}

func (svc *svcContext) AddService(service Service) {
	svc.services = append(svc.services, service)
}

func (svc *svcContext) StartApp() {
	for {
		select {
		case <-svc.ctx.Done():
			for _, service := range svc.services {
				service.Stop()
			}
			svc.wg.Wait()
			return

		default:
			for _, sv := range svc.services {
				svc.wg.Add(1)
				go func(s Service) {
					defer svc.wg.Done()
					s.Start(svc.ctx)
				}(sv)
			}
		}
	}
}

func (svc *svcContext) StopApp() {
	svc.cancel()
}
