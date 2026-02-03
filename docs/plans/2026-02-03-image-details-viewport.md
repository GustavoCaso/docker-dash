# Image Details Viewport Feature

**Date:** 2026-02-03

## Summary

Added a side-by-side layout to the image list component that displays detailed image information (including layers) in a scrollable viewport when an image is selected.

## Changes Made

### 1. Data Model (`internal/service/types.go`)

Added `Layer` struct and `Layers` field to `Image`:

```go
type Layer struct {
    ID      string    // Layer digest/ID
    Command string    // Dockerfile instruction that created this layer
    Size    int64     // Layer size in bytes
    Created time.Time // When the layer was created
}

type Image struct {
    // ... existing fields ...
    Layers []Layer // Image layers from history
}
```

### 2. Layer Fetching (`internal/service/local.go`)

- Added `fetchLayers()` method that calls Docker's `ImageHistory` API
- Updated `List()` to populate layers for each image at load time

### 3. UI Component (`internal/ui/components/image_list.go`)

**Layout:**
- 40/60 horizontal split (list on left, details on right)
- Both panes have rounded borders
- Focused pane has pink border (color 205), unfocused has gray (color 240)

**Focus Management:**
- `Enter` - Focus the details viewport (when list is focused)
- `Esc` - Return focus to the image list (when viewport is focused)
- Only the focused pane receives keyboard input

**Details Display:**
- Image name, ID (short), size, created date
- Dangling status if applicable
- Numbered list of layers with:
  - Cleaned command (removes `/bin/sh -c` and `#(nop)` prefixes)
  - Size per layer
  - Short layer ID

## Files Modified

- `internal/service/types.go` - Added Layer struct, Layers field
- `internal/service/local.go` - Added fetchLayers(), updated List()
- `internal/ui/components/image_list.go` - Complete rewrite for side-by-side layout

## Next Steps / Future Improvements

- Consider lazy-loading layers only when an image is selected (performance optimization for many images)
- Add more image details (environment variables, exposed ports, entrypoint)
- Add ability to copy layer commands or IDs to clipboard
