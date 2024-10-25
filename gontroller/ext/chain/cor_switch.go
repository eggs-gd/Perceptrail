package chain

type Switcher[Ti any, To any] interface {
	// Switch takes Ti and then makes decision where it should be placed
	// mapping evaluates by order channels in parent struct and results
	// arr := Switch(in Ti)
	// &Splitter.chout[0] <- arr[0]
	// &Splitter.chout[n] <- arr[n]
	Switch(Ti) (map[int]To, error)
	Close()
}
type switchRunner[Ti any, To any] struct {
	cherr     chan<- error
	chin      <-chan Ti
	chout     []chan<- To
	processor Switcher[Ti, To]
}

func (s *switchRunner[Ti, To]) setErrorChannel(cherr chan<- error) {
	s.cherr = cherr
}

func (s *switchRunner[Ti, To]) Process() {
	for input := range s.chin {
		res, err := s.processor.Switch(input)
		if err != nil {
			s.cherr <- err
		} else {
			for i, o := range res {
				if i < len(s.chout) {
					s.chout[i] <- o
				}
			}
		}
	}
}

func (s *switchRunner[Ti, To]) Close() {
	s.processor.Close()
}

func NewSwitch[Ti any, To any](chin <-chan Ti, chout []chan<- To, processor Switcher[Ti, To]) Processor {
	return &switchRunner[Ti, To]{
		chin:      chin,
		chout:     chout,
		processor: processor,
	}
}
