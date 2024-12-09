package perceptors

import (
	l "perceptrail/logger"
)

type Bucket struct {
}

// Item represents a basic media item
type Item struct {
	ID       string                 // GUID of the item
	Path     string                 // File path or URL
	Metadata map[string]interface{} // Base metadata (e.g., date, size)
}

// UIEndpoint describes UI capabilities and actions
type UIEndpoint struct {
	Path        string        // Endpoint path, e.g., "/color/similarity"
	Method      string        // HTTP method: GET, POST
	Description string        // Description for UI display
	Parameters  []UIParameter // Parameters for the endpoint
}

// UIParameter defines expected inputs for UIEndpoint
type UIParameter struct {
	Name     string      // Parameter name, e.g., "baseColor"
	Type     string      // Type: string, number, boolean
	Required bool        // Is this parameter mandatory
	Default  interface{} // Default value if optional
}

// InitContext provides helper functions for setting up resources
type InitContext interface {
	CreateTable(name string, schema interface{}) error // Create a table in main DB
	CreateBucket(name string) (Bucket, error)          // Create a bucket in a separate DB or storage
	Logger() l.Logger                                  // Logging interface
}

type SortIndex struct {
	ItemID string  // GUID of the item
	Score  float64 // Score for sorting
}

// Perceptor interface defines the core methods for plugins
type Perceptor interface {
	Name() string                                                          // Unique plugin name
	Initialize(ctx InitContext) error                                      // Called on load, setup tables or buckets
	UIEndpoints() ([]UIEndpoint, error)                                    // Provides endpoints for client UI
	Scan(items []Item) error                                               // Processes existing items for metadata
	Sort(items []Item, params map[string]interface{}) ([]SortIndex, error) // Sorting implementation
	Filter(items []Item, params map[string]interface{}) ([]Item, error)    // Filtering implementation
}
