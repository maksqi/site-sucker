package desktop

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/egoist/mygo"
)

// prefs are the settings of the app itself, apart from a crawl's, kept in
// the user data folder from one launch to the next.
type prefs struct {
	// Theme is the appearance: the system's, light or dark.
	Theme mygo.ThemeSource `json:"theme"`
	// Zoom is the size of the interface, one of zoomOptions' values.
	Zoom float32 `json:"zoom"`
}

// themeOption is an appearance Settings offers.
type themeOption struct {
	source mygo.ThemeSource
	label  string
}

// themeOptions are the appearances, in the order of the segmented control.
var themeOptions = []themeOption{
	{mygo.ThemeSystem, "Auto"},
	{mygo.ThemeLight, "Light"},
	{mygo.ThemeDark, "Dark"},
}

// zoomOption is a size of the interface Settings offers.
type zoomOption struct {
	zoom  float32
	label string
}

// zoomOptions are the sizes of the interface, in the order of the
// segmented control. The default is the middle one, larger than the sizes
// the widgets are drawn at.
var zoomOptions = []zoomOption{
	{1, "Small"},
	{1.15, "Medium"},
	{1.3, "Large"},
}

func defaultPrefs() prefs { return prefs{Theme: mygo.ThemeSystem, Zoom: zoomOptions[1].zoom} }

// prefsPath is where the preferences are kept.
func prefsPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "preferences.json"), nil
}

// readPrefs reads the preferences at path: the defaults when there are
// none yet, and in place of anything it does not know.
func readPrefs(path string) (prefs, error) {
	p := defaultPrefs()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return p, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &p)
	}
	switch p.Theme {
	case mygo.ThemeSystem, mygo.ThemeLight, mygo.ThemeDark:
	default:
		p.Theme = mygo.ThemeSystem
	}
	if !slices.ContainsFunc(zoomOptions, func(o zoomOption) bool { return o.zoom == p.Zoom }) {
		p.Zoom = defaultPrefs().Zoom
	}
	return p, err
}

func writePrefs(path string, p prefs) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// setTheme chooses the appearance, the window's and the app's, and keeps
// it for the next launch.
func (a *app) setTheme(source mygo.ThemeSource) {
	if a.prefs.Theme == source {
		return
	}
	a.prefs.Theme = source
	a.sys.applyTheme(source)
	a.savePrefs()
}

// setZoom chooses the size of the interface, which the next frame shows,
// and keeps it for the next launch.
func (a *app) setZoom(zoom float32) {
	if a.prefs.Zoom == zoom {
		return
	}
	a.prefs.Zoom = zoom
	a.savePrefs()
}

// savePrefs keeps the preferences: a file of a few bytes, written at once.
func (a *app) savePrefs() {
	a.fail("Could not save the settings", a.sys.savePrefs(a.prefs))
}
