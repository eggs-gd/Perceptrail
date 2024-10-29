package chain

import (
	"errors"
	"sync"
)

var ErrSkippedItem = errors.New("skipped item")

type Processor interface {
	setErrorChannel(chan<- error)
	Process()
	Close()
}

type ChainProcessor interface {
	Processor
	AddStep(actor Processor)
}

type Chain struct {
	errch  chan<- error
	actors []Processor
}

func (ch *Chain) setErrorChannel(errch chan<- error) {
	ch.errch = errch
}

func (ch *Chain) AddStep(a Processor) {
	a.setErrorChannel(ch.errch)
	ch.actors = append(ch.actors, a)
}

func (ch *Chain) Process() {

	wg := &sync.WaitGroup{}

	for _, actor := range ch.actors {
		wg.Add(1)
		go func(a Processor) {
			a.Process()
			wg.Done()
		}(actor)
	}

	wg.Wait()
}

func (ch *Chain) Close() {
	for _, a := range ch.actors {
		a.Close()
	}
}

func NewChainProcessor(errch chan error) *Chain {
	ch := &Chain{}
	ch.setErrorChannel(errch)
	return ch
}
