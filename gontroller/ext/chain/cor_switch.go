package chain

type Switcher[Ti any, To any] interface {
	// Switch takes Ti and then makes decision where it should be placed
	// mapping evaluates by order channels in parent struct and results
	// arr := Switch(in Ti)
	// &Splitter.chout[0] <- arr[0]
	// &Splitter.chout[n] <- arr[n]
	Switch(Ti) map[int]To
	Close()
}
type switchRunner[Ti any, To any] struct {
	chin      <-chan Ti
	chout     []chan<- To
	processor Switcher[Ti, To]
}

func (s *switchRunner[Ti, To]) Process() {
	for input := range s.chin {
		results := s.processor.Switch(input)

		for i, o := range results {
			if i < len(s.chout) {
				s.chout[i] <- o
			}
		}
	}
}

func (s *switchRunner[Ti, To]) Close() {
	s.processor.Close()
}

func NewSwitch[Ti any, To any](chin <-chan Ti, chout []chan<- To, processor Switcher[Ti, To]) Processor {
	return &switchRunner[Ti, To]{
		chin, chout, processor,
	}
}
