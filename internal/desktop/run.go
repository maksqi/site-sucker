// Package desktop is the Site Sucker desktop app: a window of native UI,
// drawn by mygo in the look of a SwiftUI app on macOS, over the engine that
// runs the crawls.
package desktop

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const aboutText = "A website downloader in Go: HTTP/SOCKS5 proxies, JavaScript rendering in Chromium, and a native mygo interface."

var jsonFilters = []mygo.FileFilter{{Name: "JSON profile", Extensions: []string{"json"}}}

// Run starts the app and returns when it quits.
func Run() {
	app := mygo.App
	app.SetName("Site Sucker")
	a := newApp(system{})
	// The window opens before the user can ask for a dialog.
	a.sys = desktopSystem(func() *mygo.Window { return a.win })

	app.WhenReady(func() {
		a.win = openWindow(a)
		a.engine.SetWindow(a.win)
		app.SetMenu(buildMenu(a))
	})
	// Closing the window quits, as in every version of the app, after
	// stopping the crawl and the interactive browser.
	app.OnWindowAllClosed(func() {
		a.engine.Shutdown()
		app.Quit()
	})
	app.OnBeforeQuit(func(*mygo.QuitEvent) { a.engine.Shutdown() })
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func openWindow(a *app) *mygo.Window {
	opts := mygo.WindowOptions{
		Title:          "Site Sucker",
		Width:          1180,
		Height:         800,
		MinWidth:       920,
		MinHeight:      620,
		StateKey:       "main",
		TitleBarStyle:  mygo.TitleBarHidden,
		TitleBarHeight: barHeight,
		Content:        ui.View(a.view),
	}
	if runtime.GOOS == "darwin" {
		// The traffic lights sit in the top of the floating sidebar, level
		// with the toolbar's middle.
		opts.TrafficLightPosition = &mygo.Point{X: 21, Y: 19}
	}
	return mygo.NewWindow(opts)
}

// desktopSystem asks the system's dialogs and file manager, the dialogs as
// sheets of the window.
func desktopSystem(win func() *mygo.Window) system {
	open := func(opts mygo.OpenDialogOptions) (string, error) {
		opts.Parent = win()
		paths, err := mygo.Dialog.Open(opts)
		if err != nil || len(paths) == 0 {
			return "", err
		}
		return paths[0], nil
	}
	return system{
		chooseFolder: func(title, current string) (string, error) {
			return open(mygo.OpenDialogOptions{
				Title:             title,
				ButtonLabel:       "Choose",
				DefaultPath:       strings.TrimSpace(current),
				Directory:         true,
				CreateDirectories: true,
			})
		},
		chooseBrowser: func() (string, error) {
			opts := mygo.OpenDialogOptions{Title: "Chrome or Chromium", ButtonLabel: "Choose"}
			if runtime.GOOS == "darwin" {
				opts.DefaultPath = "/Applications"
			}
			return open(opts)
		},
		openProfile: func() (string, error) {
			return open(mygo.OpenDialogOptions{Title: "Import Profile", ButtonLabel: "Import", Filters: jsonFilters})
		},
		saveProfile: func() (string, error) {
			return mygo.Dialog.Save(mygo.SaveDialogOptions{
				Parent:            win(),
				Title:             "Export Profile",
				ButtonLabel:       "Save",
				DefaultPath:       "site-sucker-profile.json",
				Filters:           jsonFilters,
				CreateDirectories: true,
			})
		},
		openPath: mygo.Shell.OpenPath,
	}
}

func buildMenu(a *app) *mygo.Menu {
	// Clicks run on the main thread, as the view does; the window draws
	// what they changed.
	do := func(action func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) {
			action()
			if a.win != nil {
				a.win.Invalidate()
			}
		}
	}
	darwin := runtime.GOOS == "darwin"

	var items []*mygo.MenuItem
	if darwin {
		items = append(items, &mygo.MenuItem{Label: "Site Sucker", Submenu: []*mygo.MenuItem{
			{Label: "About Site Sucker", Click: func(*mygo.MenuItem, *mygo.Window) { showAbout() }},
			mygo.Separator(),
			{Label: "Settings…", Accelerator: "CmdOrCtrl+,", Click: do(func() { a.section = secLimits })},
			mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}})
	}

	file := []*mygo.MenuItem{
		{Label: "Import Profile…", Accelerator: "CmdOrCtrl+O", Click: do(a.importProfile)},
		{Label: "Export Profile…", Accelerator: "CmdOrCtrl+S", Click: do(a.exportProfile)},
		mygo.Separator(),
		{Label: "Open Download Folder", Accelerator: "CmdOrCtrl+Shift+O", Click: do(a.openFolder)},
		mygo.Separator(),
		{Role: mygo.RoleClose},
	}
	if !darwin {
		file = append(file, &mygo.MenuItem{Role: mygo.RoleQuit})
	}

	// The sections, as the sidebar lists them, by Cmd and their place.
	view := []*mygo.MenuItem{{Label: "Download", Accelerator: "CmdOrCtrl+1", Click: do(func() { a.section = secDashboard })}}
	for i, it := range navItems {
		view = append(view, &mygo.MenuItem{Label: it.label, Accelerator: fmt.Sprintf("CmdOrCtrl+%d", i+2), Click: do(func() { a.section = it.id })})
	}
	view = append(view, mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleToggleFullScreen})
	if mygo.IsDev() {
		view = append(view, mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleToggleDevTools})
	}

	items = append(items,
		&mygo.MenuItem{Label: "File", Submenu: file},
		&mygo.MenuItem{Label: "Edit", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleUndo},
			{Role: mygo.RoleRedo},
			mygo.Separator(),
			{Role: mygo.RoleCut},
			{Role: mygo.RoleCopy},
			{Role: mygo.RolePaste},
			{Role: mygo.RoleSelectAll},
		}},
		&mygo.MenuItem{Label: "Download", Submenu: []*mygo.MenuItem{
			{Label: "Start", Accelerator: "CmdOrCtrl+Enter", Click: do(a.start)},
			{Label: "Pause / Resume", Accelerator: "CmdOrCtrl+P", Click: do(a.togglePause)},
			{Label: "Stop", Accelerator: "CmdOrCtrl+.", Click: do(a.stop)},
		}},
		&mygo.MenuItem{Label: "View", Submenu: view},
		&mygo.MenuItem{Label: "Window", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleMinimize},
			{Role: mygo.RoleZoom},
		}},
	)
	if !darwin {
		items = append(items, &mygo.MenuItem{Label: "Help", Submenu: []*mygo.MenuItem{
			{Label: "About Site Sucker", Click: func(*mygo.MenuItem, *mygo.Window) { go showAbout() }},
		}})
	}
	return mygo.NewMenu(items)
}

func showAbout() {
	if runtime.GOOS == "darwin" {
		mygo.App.ShowAboutPanel(mygo.AboutPanelOptions{
			ApplicationName:    "Site Sucker",
			ApplicationVersion: mygo.App.Version(),
			Credits:            aboutText,
		})
		return
	}
	_, _ = mygo.Dialog.Message(mygo.MessageOptions{
		Type: mygo.MessageInfo, Title: "Site Sucker", Message: "Site Sucker " + mygo.App.Version(), Detail: aboutText,
	})
}
