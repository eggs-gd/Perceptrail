package chain

type ControlBase interface {
	Process()
}

type IChain interface {
	ControlBase
	AddStep(actor *ControlBase)
	Stop()
}

type Chain struct {
	actors []ControlBase
}

func (ch *Chain) AddStep(a ControlBase) {
	ch.actors = append(ch.actors, a)
}

func (ch *Chain) Process() {
	for _, actor := range ch.actors {
		go actor.Process()
	}
}

func (ch *Chain) Stop() {

}
