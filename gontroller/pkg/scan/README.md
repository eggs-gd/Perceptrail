# Scan Package

The `scan` package implements file system scanning and metadata extraction pipeline.

## Overview

This package provides functionality for:
- Walking through file system
- Extracting metadata from files
- Processing files through plugins
- Updating database with extracted information

## Core Components

### FSWalker
Entry point that walks through file system:
```go
type FSWalker interface {
    Walk(path string) error
    Stop()
}
```

### Entry Processor
Processes initial file entries:
```go
type EntryProcessor interface {
    Process(path string) error
}
```

### EXIF Pipeline

#### ExifExtractor
Extracts EXIF data from files:
- Reads file metadata
- Parses EXIF information
- Prepares data for plugins

#### ExifPluginProcessor
Manages EXIF-based plugins pipeline:
```
[RawItem] -> Opener -> [Plugin1] -> [Plugin2] -> ... -> [PluginN] -> Closer -> [ItemDto]
```

Components:
- **Opener**: Converts RawItem to RawItemRW interface
- **Plugins**: Process EXIF data (date, geo, size, etc.)
- **Closer**: Updates database with processed data

## Data Flow

1. FSWalker finds files
2. EntryProcessor creates initial RawItem
3. ExifExtractor reads EXIF data
4. ExifPluginProcessor runs plugins chain:
   - Opener prepares data
   - Each plugin processes specific metadata
   - Closer saves results

## Plugin System

Supports different types of plugins:
- EXIF data processors
- Raw data processors
- Metadata processors

Plugin requirements:
```go
type Perceptor interface {
    Name() string
    DataProvider() DataProviderType
    ProcessingMode() ProcessingMode
}
```

## Usage Example

```go
// Create scanner
scanner := scan.NewScanner(logger)

// Configure paths
scanner.AddPath("/photos")
scanner.AddPath("/documents")

// Start scanning
go scanner.Start()

// Handle results
for result := range scanner.Results() {
    // Process scan results
}
```

## Error Handling

Errors are reported through dedicated error channels:
- Scan errors
- Processing errors
- Plugin errors

## Best Practices

1. Scanner Configuration
   - Set appropriate buffer sizes
   - Configure file filters
   - Set scan depth

2. Plugin Management
   - Load plugins at startup
   - Verify plugin compatibility
   - Handle plugin errors

3. Resource Management
   - Close files properly
   - Clean up temporary data
   - Handle goroutine lifecycle

4. Performance
   - Use buffered channels
   - Implement concurrent processing
   - Control memory usage 