package importer

import (
	"gontroller/ext/chain"
	t "gontroller/pkg/_t"
	"gontroller/pkg/model"
)

type importValidator struct {
	model *model.Proxy
}

func (cd *importValidator) Decorate(in t.RawExif) (model.ItemDto, error) {
	var res = cd.model.ValidateFile(in)
	return model.ItemDto{}, nil
}

func (cd *importValidator) Close() {

}

func NewGatekeeper(count int, chin <-chan t.RawExif, chout chan<- model.ItemDto) chain.Processor {
	var model *model.Proxy = model.NewProxy()
	processor := &importValidator{
		model: model,
	}
	return chain.NewDecorator(chin, chout, processor)
}
