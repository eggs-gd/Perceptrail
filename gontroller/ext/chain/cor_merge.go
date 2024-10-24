package chain

// Merge takes many inputs and put them to one channel with[out] conversion
type Merge[Ti any, To any] func([]Ti) To

type IMerge[Ti any, To any] interface {
	Merge([]Ti) To
}
type mergeRunner[Ti any, To any] struct {
	chin      []<-chan Ti
	chout     chan<- To
	processor IMerge[Ti, To]
}

func (m *mergeRunner[Ti, To]) Process() {
	for {
		var inputs []Ti
		for _, ch := range m.chin {
			input, ok := <-ch
			if ok {
				inputs = append(inputs, input)
			} else {
				return
			}
		}
		result := m.processor.Merge(inputs)
		m.chout <- result
	}
}

func NewMerger[Ti any, To any](chin []<-chan Ti, chout chan<- To, processor IMerge[Ti, To]) ControlBase {
	return &mergeRunner[Ti, To]{
		chin, chout, processor,
	}
}
