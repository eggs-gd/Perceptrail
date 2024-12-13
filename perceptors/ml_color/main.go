package main

import (
	"github.com/dukobpa3/perceplib/api"
	"github.com/dukobpa3/perceplib/chain"
	l "github.com/dukobpa3/perceplib/logger"
)

type colorPerceptor struct{}

func (p *colorPerceptor) Name() string                       { return "ml_color" }
func (p *colorPerceptor) DataProvider() api.DataProviderType { return api.RawDataProvider }
func (p *colorPerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *colorPerceptor) NewProcessor(chin <-chan api.RawItemR, chout chan<- api.RawItemR, logger *l.Logger) chain.Processor {
	// TODO: Implement color processor
	return nil
}

//export Perceptor
var Perceptor api.Perceptor = &colorPerceptor{}

func main() {}
