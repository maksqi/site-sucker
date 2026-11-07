package desktop

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

// The interface's fonts (fonts/LICENSE, the SIL Open Font License): Inter
// as a variable font, which draws every weight, and IBM Plex Mono.
var (
	//go:embed fonts/InterVariable.ttf
	interFont []byte
	//go:embed fonts/IBMPlexMono-Regular.ttf
	plexMonoFont []byte
)

func init() {
	for _, f := range []struct {
		data   []byte
		family string
	}{{interFont, fontUI}, {plexMonoFont, fontMono}} {
		if err := ui.RegisterFont(f.data, f.family); err != nil {
			panic(err)
		}
	}
}
