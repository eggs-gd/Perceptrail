package chain

type EntryPoint[Ti any, To any] interface {
	Start(chin chan<- Ti)
	Decorate(in Ti) (To, error)
	Close()
}

type entryRunner[Ti any, To any] struct {
	cherr     chan<- error
	chin      chan Ti
	chout     chan<- To
	processor EntryPoint[Ti, To]
}

func (d *entryRunner[Ti, To]) setErrorChannel(cherr chan<- error) {
	d.cherr = cherr
}

func (d *entryRunner[Ti, To]) Process() {
	go d.processor.Start(d.chin)
	defer d.processor.Close()

	for input := range d.chin {
		res, err := d.processor.Decorate(input)
		if err != nil {
			d.cherr <- err
		} else {
			d.chout <- res
		}
	}
}

func (d *entryRunner[Ti, To]) Close() {
	d.processor.Close()
}

func NewEntryPoint[Ti any, To any](chout chan<- To, processor EntryPoint[Ti, To]) Processor {

	return &entryRunner[Ti, To]{
		chin:      make(chan Ti),
		chout:     chout,
		processor: processor,
	}
}
