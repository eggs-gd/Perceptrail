package validator

import (
	"gontroller/ext/chain"
	t "gontroller/pkg/_t"
	"gontroller/pkg/model"
)

type importValidator struct{}

func (cd *importValidator) Decorate(in t.RawExif) model.ItemDto {
	return model.ItemDto{}
}

func (cd *importValidator) Close() {

}

func NewValidator(count int, chin <-chan t.RawExif, chout chan<- model.ItemDto) chain.Processor {
	processor := &importValidator{}
	return chain.NewDecorator(chin, chout, processor)
}
