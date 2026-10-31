// Package places: what the geo perceptor keeps per photo. Its own package (not
// main), so another perceptor can read it (journeys: by time and place).
package places

import "github.com/eggs-gd/perceplib/api"

// Location: where a photo was taken, degrees (south and west negative)
type Location struct {
	Lat float64
	Lon float64
}

// Places: the geo perceptor's store (data_dir/perceptors/geo.db)
var Places = api.NewStore[Location]("geo", 1)
