package api

// Navigation is what every perceptor gives the gallery: a way through the whole
// library — an order of every item (the endless sheet), with sections for the side
// panel. The client lays the sheet out; switching perceptors keeps the photo the
// user is at in place and rearranges the rest around it.
//
//   - absolute (View.Relative false): the order does not depend on the anchor — the
//     date (newest first), the size; the anchor is only where the client puts the
//     view;
//   - relative: the order is built from the anchor — a trail through similar photos
//     in both directions (faces, objects).

// View: what the client shows for a perceptor — a button in the toolbars and a
// help text
type View struct {
	// Slug: the view's name in URLs (/v/date) — public, unique among the
	// perceptors; not the plugin's name (an inside detail)
	Slug  string
	Title string
	// SVG markup, 24×24 viewBox, currentColor; the client shows it as a mask (no
	// scripts run)
	Icon     string
	Help     string
	Relative bool
}

// Fact: one line of the viewer's info panel — "Taken: 14 Sep 2025, 08:00 +03:00"
type Fact struct {
	Label string
	Value string
}

// Entry: one item of the sheet. Sections: the sections this item starts, coarsest
// first — a path ("2026", "January") or a single tag; none inside a section. The
// side panel's marks.
type Entry struct {
	GUID     GUID
	Sections []Section
}

// Section: a mark on the side panel. Level 0 is the coarsest (a year, a country);
// deeper levels are shown when there is room.
type Section struct {
	Level int
	Label string
}
