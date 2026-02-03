# Image List Help Bar Design

## Overview

Add an inline help bar at the bottom of the list panel showing available actions for the selected image. Initially supports toggling layers visibility.

## Behavior

- **Default state:** Detail panel shows image info without layers section
- **Press `l`:** Toggles layers section visibility in detail panel
- **Help bar:** Shows `l: layers` at bottom of list panel

## Implementation

### Changes to `image_list.go`

1. Add `showLayers bool` field to `ImageList` struct
2. Handle `l` keypress in `Update()` to toggle `showLayers`
3. Modify `updateDetails()` to conditionally render layers section based on `showLayers`
4. Modify `View()` to render help bar below the list

### Layout

```
+------------------+------------------------+
|  Image List      |  Image Details         |
|  - image1        |  Name: image1:latest   |
|  - image2        |  ID: abc123            |
|  > image3        |  Size: 150 MB          |
|                  |  Created: 2026-01-15   |
|                  |                        |
|                  |  [Layers section       |
|                  |   shown when toggled]  |
|                  |                        |
|  l: layers       |                        |
+------------------+------------------------+
```

### Styling

Use `theme.HelpStyle` for the help bar text to maintain consistency with existing UI patterns.

## Future Extensions

This pattern allows adding more actions:
- `d: delete` - Delete selected image
- `p: pull` - Pull/update image
- `i: inspect` - Show full inspect JSON
