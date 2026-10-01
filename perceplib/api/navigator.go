package api

import "context"

// Navigator is a perceptor that gives the gallery a way through the whole library:
// an order of every item (the endless sheet), with sections for the side panel.
// The client lays the sheet out; switching navigators keeps the photo the user is
// at in place and rearranges the rest around it.
//
//   - absolute (Relative false): the order does not depend on the anchor — date
//     (newest first), geo; the anchor is only where the client puts the view;
//   - relative: the order is built from the anchor — a trail through similar
//     photos in both directions (faces, objects).
type Navigator interface {
	Perceptor
	View() View
	// Order: every item of items, in sheet order. anchor: the item the user is at
	// ("" for none) — relative navigators start from it.
	Order(ctx context.Context, anchor string, items []ItemDataProvider) ([]Entry, error)
}

// View: what the client shows for a navigator — a button in the toolbars and a help
// text
type View struct {
	Title string
	// SVG markup, 24×24 viewBox, currentColor; the client shows it as an image (no
	// scripts run)
	Icon     string
	Help     string
	Relative bool
}

// Entry: one item of the sheet. Section is set on the first item of a section
// (the side panel's marks).
type Entry struct {
	Guid    string
	Section *Section
}

// Section: a mark on the side panel. Level 0 is the coarsest (a year, a country);
// deeper levels are shown when there is room.
type Section struct {
	Level int
	Label string
}
