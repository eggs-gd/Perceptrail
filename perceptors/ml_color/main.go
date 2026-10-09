package main

import (
	chain "github.com/eggs-gd/go-chain"
	l "github.com/eggs-gd/go-zap-decor"
	"github.com/eggs-gd/perceplib/api"
)

//export Perceptor
var Perceptor api.Perceptor = &colorPerceptor{}

type colorPerceptor struct{}

func (p *colorPerceptor) Name() string { return "ml_color" }

func (p *colorPerceptor) DataProvider() api.DataProviderType { return api.RawDataProvider }

func (p *colorPerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }

func (p *colorPerceptor) Decorator(logger *l.Logger) chain.Decorator[api.RawItemR, api.RawItemR] {
	// TODO: Implement color processor
	return nil
}

func main() {}
