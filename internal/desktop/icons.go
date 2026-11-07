package desktop

import (
	"bytes"
	"embed"

	"github.com/egoist/mygo/ui"
)

// Icons from Lucide (icons/LICENSE), drawn in the color of the text, with
// a lighter stroke than theirs, nearer to SF Symbols' regular weight.
//
//go:embed icons/*.svg
var iconFiles embed.FS

func iconData(name string) []byte {
	data, err := iconFiles.ReadFile("icons/" + name + ".svg")
	if err != nil {
		panic(err)
	}
	return bytes.Replace(data, []byte(`stroke-width="2"`), []byte(`stroke-width="1.75"`), 1)
}

func icon(name string) *ui.SVG { return ui.MustParseSVG(iconData(name)) }

// filledIcon is an icon filled as well as stroked, as the play and stop
// glyphs of the buttons.
func filledIcon(name string) *ui.SVG {
	return ui.MustParseSVG(bytes.Replace(iconData(name), []byte(`fill="none"`), []byte(`fill="currentColor"`), 1))
}

var (
	iconAppWindow   = icon("app-window")
	iconFollow      = icon("arrow-down-to-line")
	iconBraces      = icon("braces")
	iconCheck       = icon("check")
	iconChevron     = icon("chevron-right")
	iconChevronUp   = icon("chevron-up")
	iconChevronDown = icon("chevron-down")
	iconError       = icon("circle-alert")
	iconSuccess     = icon("circle-check")
	iconCopy        = icon("copy")
	iconDownload    = icon("download")
	iconEye         = icon("eye")
	iconEyeOff      = icon("eye-off")
	iconArchive     = icon("file-archive")
	iconFileCode    = icon("file-code")
	iconFileOther   = icon("file-question-mark")
	iconFilm        = icon("film")
	iconFolderOpen  = icon("folder-open")
	iconFolderTree  = icon("folder-tree")
	iconGlobe       = icon("globe")
	iconImage       = icon("image")
	iconInfo        = icon("info")
	iconNetwork     = icon("network")
	iconPalette     = icon("palette")
	iconPause       = icon("pause")
	iconPlay        = filledIcon("play")
	iconScanSearch  = icon("scan-search")
	iconSearch      = icon("search")
	iconSettings    = icon("sliders-horizontal")
	iconSparkles    = icon("sparkles")
	iconStop        = filledIcon("square")
	iconTrash       = icon("trash")
	iconWarning     = icon("triangle-alert")
	iconType        = icon("type")
	iconClose       = icon("x")
	iconZap         = icon("zap")
)
