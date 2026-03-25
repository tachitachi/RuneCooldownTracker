package detection

import (
	"embed"
	"image"
	"image/png"
	"path/filepath"
	"strings"
)

//go:embed icons/*.png
var iconFS embed.FS

// LoadReferenceIcons returns a map of ability name → decoded image for every
// PNG in the embedded icons directory.
func LoadReferenceIcons() map[string]image.Image {
	entries, err := iconFS.ReadDir("icons")
	if err != nil {
		return nil
	}
	out := make(map[string]image.Image, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		f, err := iconFS.Open(filepath.Join("icons", e.Name()))
		if err != nil {
			continue
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".png")
		out[name] = img
	}
	return out
}
