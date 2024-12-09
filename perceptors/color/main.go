package main

import (
	"perceptrail/perceptors"
)

type ColorPlugin struct{}

func (c *ColorPlugin) Name() string                                  { return "ColorPlugin" }
func (c *ColorPlugin) Initialize(ctx perceptors.InitContext) error   { return nil }
func (c *ColorPlugin) UIEndpoints() ([]perceptors.UIEndpoint, error) { return nil, nil }
func (c *ColorPlugin) Scan(items []perceptors.Item) error            { return nil }
func (c *ColorPlugin) Sort(items []perceptors.Item, params map[string]interface{}) ([]perceptors.SortIndex, error) {
	return nil, nil
}
func (c *ColorPlugin) Filter(items []perceptors.Item, params map[string]interface{}) ([]perceptors.Item, error) {
	return nil, nil
}

var Perceptor perceptors.Perceptor = &ColorPlugin{}
