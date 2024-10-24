package chain

// Merge takes many inputs and put them to one channel with[out] conversion
type Merge[Ti any, To any] func([]Ti) To

type Merger[Ti any, To any] interface {
	Merge([]Ti) To
	Close()
}
type mergeRunner[Ti any, To any] struct {
	chin      []<-chan Ti
	chout     chan<- To
	processor Merger[Ti, To]
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

func (m *mergeRunner[Ti, To]) Close() {
	m.processor.Close()
}

func NewMerger[Ti any, To any](chin []<-chan Ti, chout chan<- To, processor Merger[Ti, To]) Processor {
	return &mergeRunner[Ti, To]{
		chin, chout, processor,
	}
}
