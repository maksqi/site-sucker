package desktop

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// settingsPage holds the settings of the app itself: its appearance, and
// the profiles that carry a crawl's settings in and out.
func (a *app) settingsPage(c *ui.Context) {
	a.page(c, func() {
		a.group(c, "Appearance", "", func() {
			a.row(c, "Theme", "Auto follows the system's light or dark appearance", false, func() {
				chosen := max(0, slices.IndexFunc(themeOptions, func(o themeOption) bool { return o.source == a.prefs.Theme }))
				labels := make([]string, len(themeOptions))
				for i, o := range themeOptions {
					labels[i] = o.label
				}
				seg := a.segmented(c, "theme", "Theme", &chosen, labels, nil)
				if seg.Changed() {
					a.setTheme(themeOptions[chosen].source)
				}
			})
			a.row(c, "Interface size", "How large text, buttons and controls are", false, func() {
				chosen := max(0, slices.IndexFunc(zoomOptions, func(o zoomOption) bool { return o.zoom == a.prefs.Zoom }))
				labels := make([]string, len(zoomOptions))
				for i, o := range zoomOptions {
					labels[i] = o.label
				}
				if a.segmented(c, "zoom", "Interface size", &chosen, labels, nil).Changed() {
					a.setZoom(zoomOptions[chosen].zoom)
				}
			})
		})
		a.group(c, "Profiles", "A profile is a JSON file of every setting of a crawl. "+passwordWarning, func() {
			a.row(c, "Import a profile", "Replaces the current settings ("+modKey+"O)", false, func() {
				if a.button(c, buttonSecondary, nil, "Import…", false).Clicked() {
					a.importProfile()
				}
			})
			a.row(c, "Export the settings", "Saves them as a profile ("+modKey+"S)", false, func() {
				if a.button(c, buttonSecondary, nil, "Export…", false).Clicked() {
					a.exportProfile()
				}
			})
		})
	})
}
