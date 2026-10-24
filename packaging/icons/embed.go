// Package icons embeds Snag's app icon; every place that shows the icon uses these files.
package icons

import _ "embed"

// App is the window and desktop icon (256px).
//
//go:embed snag-256.png
var App []byte

// Tray is the status-area icon (64px).
//
//go:embed snag-64.png
var Tray []byte

// TrayTemplate is the macOS menu-bar icon: a monochrome image the system tints.
//
//go:embed tray-template.png
var TrayTemplate []byte

// SVG is the scalable original.
//
//go:embed snag.svg
var SVG []byte
