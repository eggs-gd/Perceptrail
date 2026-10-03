package main

import (
	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"
	l "github.com/eggs-gd/perceplib/logger"
)

type colorPerceptor struct{}

func (p *colorPerceptor) Name() string                       { return "ml_color" }
func (p *colorPerceptor) DataProvider() api.DataProviderType { return api.RawDataProvider }
func (p *colorPerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *colorPerceptor) Decorator(logger *l.Logger) chain.Decorator[api.RawItemR, api.RawItemR] {
	// TODO: Implement color processor
	return nil
}

//export Perceptor
var Perceptor api.Perceptor = &colorPerceptor{}

func main() {}
