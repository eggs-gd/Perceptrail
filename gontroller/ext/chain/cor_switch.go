package chain

type ISwitch[Ti any, To any] interface {
	// Switch takes Ti and then makes decision where it should be placed
	// mapping evaluates by order channels in parent struct and results
	// arr := Switch(in Ti)
	// &Splitter.chout[0] <- arr[0]
	// &Splitter.chout[n] <- arr[n]
	Switch(Ti) []To
}
type switchRunner[Ti any, To any] struct {
	chin      <-chan Ti
	chout     []chan<- To
	processor ISwitch[Ti, To]
}

func (s *switchRunner[Ti, To]) Process() {
	for input := range s.chin {
		results := s.processor.Switch(input)
		for i, output := range s.chout {
			output <- results[i]
		}
	}
	// for _, ch := range s.chout {
	// 	close(ch)
	// }
}

func NewSwitch[Ti any, To any](chin <-chan Ti, chout []chan<- To, processor ISwitch[Ti, To]) ControlBase {
	return &switchRunner[Ti, To]{
		chin, chout, processor,
	}
}
