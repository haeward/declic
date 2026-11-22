//go:build darwin

package main

import _ "embed"

// The ticket thumbnails come from the macOS Default and Dark renditions of
// icon/AppIcon.icon. The four Dock overrides are exports of icon/dockicons/*.svg.
var (
	//go:embed assets/dockicons/ticket.png
	ticketPreview []byte
	//go:embed assets/dockicons/ticket_dark.png
	ticketDarkPreview []byte
	//go:embed assets/dockicons/match.png
	matchIcon []byte
	//go:embed assets/dockicons/pancake.png
	pancakeIcon []byte
	//go:embed assets/dockicons/pruners.png
	prunersIcon []byte
	//go:embed assets/dockicons/nesting.png
	nestingIcon []byte
)
