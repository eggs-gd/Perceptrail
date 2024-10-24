package validator

import (
	"gontroller/ext/chain"
	"gontroller/pkg/exif"
	"gontroller/pkg/model"
)

type importValidator struct{}

func (cd *importValidator) Decorate(in exif.RawExif) model.ItemDto {
	return model.ItemDto{}
}

func NewValidator(count int, chin <-chan exif.RawExif, chout chan<- model.ItemDto) chain.ControlBase {
	processor := &importValidator{}
	return chain.NewDecorator(chin, chout, processor)
}
