package chain

type Processor interface {
	Process()
	Close()
}

type ChainProcessor interface {
	Processor
	AddStep(actor *Processor)
}

type Chain struct {
	actors []Processor
}

func (ch *Chain) AddStep(a Processor) {
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
