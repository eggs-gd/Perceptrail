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
	AddStep(actor *Processor)
}

type Chain struct {
	wg     *sync.WaitGroup
	errch  chan error
	actors []Processor
}

func (ch *Chain) AddStep(a Processor) {
	a.setErrorChannel(ch.errch)
	ch.actors = append(ch.actors, a)
}

func (ch *Chain) Process() {
	for _, actor := range ch.actors {
		go actor.Process()
	}
}

func (ch *Chain) Close() {
	for _, a := range ch.actors {
		a.Close()
	}
}

func NewChainProcessor(errch chan error, wg *sync.WaitGroup) *Chain {
	return &Chain{
		wg:    wg,
		errch: errch,
	}
}
