package main

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

//go:embed assets/brand/leetcode_light.svg
var siteBrandLightSVG []byte

//go:embed assets/brand/leetcode_dark.svg
var siteBrandDarkSVG []byte

//go:embed assets/illustrations/null.svg
var nullArtSVG []byte

// Small original line icons, shared by navigation and toolbars.
func lineIcon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

var (
	iconSidebar     = lineIcon(`<rect x="2" y="3" width="12" height="10" rx="2.5"/><path d="M6 3v10"/>`)
	iconBack        = lineIcon(`<path d="M12.5 8h-9M7.5 4 3.5 8l4 4"/>`)
	iconForward     = lineIcon(`<path d="M3.5 8h9M8.5 4l4 4-4 4"/>`)
	iconAccount     = lineIcon(`<circle cx="8" cy="5.5" r="2.7"/><path d="M3 13.5c0-3 10-3 10 0"/>`)
	iconProblems    = lineIcon(`<path d="M5 4h8M5 8h8M5 12h8M2.5 4h.01M2.5 8h.01M2.5 12h.01"/>`)
	iconSettings    = lineIcon(`<path d="M3 5h6M12 5h1M3 11h1M7 11h6"/><circle cx="10.5" cy="5" r="1.5"/><circle cx="5.5" cy="11" r="1.5"/>`)
	iconGear        = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`))
	iconCode        = lineIcon(`<path d="M6 4 2.5 8 6 12M10 4l3.5 4L10 12"/>`)
	iconSearch      = lineIcon(`<circle cx="7" cy="7" r="4"/><path d="m10 10 3.5 3.5"/>`)
	iconFilter      = lineIcon(`<path d="M2.5 4h11M5 8h6M7 12h2"/>`)
	iconSort        = lineIcon(`<path d="M5.5 3.5v9M3 6l2.5-2.5L8 6M10.5 12.5v-9M8 10l2.5 2.5L13 10"/>`)
	iconShuffle     = lineIcon(`<path d="M2 5h2.2c1.4 0 2.3 1 3.3 3s1.9 3 3.3 3H14M11.5 8.5 14 11l-2.5 2.5M2 11h2.2c1 0 1.7-.5 2.4-1.5M14 5h-3.2c-1 0-1.7.5-2.4 1.5M11.5 2.5 14 5l-2.5 2.5"/>`)
	iconExternal    = lineIcon(`<path d="M9.5 3H13v3.5M13 3 7.5 8.5M12 9v3a1.5 1.5 0 0 1-1.5 1.5H4A1.5 1.5 0 0 1 2.5 12V5.5A1.5 1.5 0 0 1 4 4h3"/>`)
	iconCheck       = lineIcon(`<path d="m3.5 8.5 3 3 6-7"/>`)
	iconLock        = lineIcon(`<rect x="3.5" y="7" width="9" height="6.5" rx="1.5"/><path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2"/>`)
	iconCalendar    = lineIcon(`<rect x="2.5" y="3.5" width="11" height="10" rx="2"/><path d="M5.5 2v2.5M10.5 2v2.5M2.5 7h11"/>`)
	iconDescription = lineIcon(`<path d="M4 2.5h5.5L13 6v7.5a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1v-10a1 1 0 0 1 1-1Z"/><path d="M9.5 2.5V6H13M5.5 9h5M5.5 11.5h3.5"/>`)
	iconEditorial   = lineIcon(`<path d="M2.5 3.5h4a2 2 0 0 1 2 2v8a1.5 1.5 0 0 0-1.5-1.5H2.5v-8.5Z"/><path d="M13.5 3.5h-4a2 2 0 0 0-2 2v8a1.5 1.5 0 0 1 1.5-1.5h4.5v-8.5Z"/>`)
	iconSolutions   = lineIcon(`<path d="M6 2.5h4M7 2.5v3.2L3.8 11.4A1.5 1.5 0 0 0 5.1 13.5h5.8a1.5 1.5 0 0 0 1.3-2.1L9 5.7V2.5"/><path d="M5.2 10h5.6"/>`)
	iconSubmissions = lineIcon(`<path d="M3 8a5 5 0 1 0 1.5-3.5L3 6"/><path d="M3 3v3h3M8 5.5v2.7l1.8 1.2"/>`)
	iconSignOut     = lineIcon(`<path d="M6 2.5H4a1.5 1.5 0 0 0-1.5 1.5v8A1.5 1.5 0 0 0 4 13.5h2M10.5 11l3-3-3-3M13.5 8H6.5"/>`)
	iconNullArt     = ui.MustParseSVG(nullArtSVG)
	iconPlay        = lineIcon(`<path d="M5 3.5v9l7.5-4.5z"/>`)
	iconUpload      = lineIcon(`<path d="M8 12.5v-9M4.5 7 8 3.5 11.5 7"/>`)
	iconReset       = lineIcon(`<path d="M3.4 9.8A5 5 0 1 0 4.5 4.4L3 5.8M3 2.8v3h3"/>`)
	iconDuplicate   = lineIcon(`<rect x="5.5" y="5.5" width="8" height="8" rx="1.6"/><path d="M10.5 3.5V3A1.5 1.5 0 0 0 9 1.5H4A1.5 1.5 0 0 0 2.5 3v5A1.5 1.5 0 0 0 4 9.5h.5"/>`)
	iconTrash       = lineIcon(`<path d="M3 4.5h10M6 4.5V3a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v1.5M4.5 4.5l.6 8a1.5 1.5 0 0 0 1.5 1.5h2.8a1.5 1.5 0 0 0 1.5-1.5l.6-8"/>`)
	iconMaximize    = lineIcon(`<path d="M9.5 3H13v3.5M13 3 9 7M6.5 13H3V9.5M3 13l4-4"/>`)
	iconMinimize    = lineIcon(`<path d="M13 3 9.5 6.5M9.5 3.5v3h3M3 13l3.5-3.5M6.5 12.5v-3h-3"/>`)
	iconChevronUp   = lineIcon(`<path d="m4 10 4-4 4 4"/>`)
	iconChevronDown = lineIcon(`<path d="m4 6 4 4 4-4"/>`)
	iconChevronLeft = lineIcon(`<path d="m10 4-4 4 4 4"/>`)
	iconChevronRgt  = lineIcon(`<path d="m6 4 4 4-4 4"/>`)
	iconGlobe       = lineIcon(`<circle cx="8" cy="8" r="5.5"/><path d="M2.5 8h11M8 2.5c1.8 1.6 2.5 3.5 2.5 5.5S9.8 11.9 8 13.5C6.2 11.9 5.5 10 5.5 8S6.2 4.1 8 2.5Z"/>`)
	iconFormat      = lineIcon(`<path d="M2.5 3.5h11M2.5 6.5h7M2.5 9.5h9.5M2.5 12.5h5.5"/>`)
	iconClose       = lineIcon(`<path d="m4.5 4.5 7 7M11.5 4.5l-7 7"/>`)
	iconRetrieve    = lineIcon(`<path d="M5.8 2.8C4.4 2.8 3.8 3.5 3.8 4.8v1.6c0 .9-.5 1.4-1.3 1.6.8.2 1.3.7 1.3 1.6v1.6c0 1.3.6 2 2 2M10.2 2.8c1.4 0 2 .7 2 2v1.6c0 .9.5 1.4 1.3 1.6-.8.2-1.3.7-1.3 1.6v1.6c0 1.3-.6 2-2 2"/>`)
	iconEye         = lineIcon(`<path d="M1.8 8s2.3-4 6.2-4 6.2 4 6.2 4-2.3 4-6.2 4-6.2-4-6.2-4Z"/><circle cx="8" cy="8" r="1.7"/>`)
	iconUpvote      = lineIcon(`<path d="M8 3.2 3.5 9h9L8 3.2Z"/><path d="M8 9v3.8"/>`)
	iconSiteLight   = ui.MustParseSVG(siteBrandLightSVG)
	iconSiteDark    = ui.MustParseSVG(siteBrandDarkSVG)
)
