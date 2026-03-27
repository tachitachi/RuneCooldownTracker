package detection

import (
	"embed"
	"image"
	"image/png"
	"path"
	"strings"
)

//go:embed icons/ready icons/not_ready
var iconFS embed.FS

func loadIconDir(dir string) map[string]image.Image {
	entries, err := iconFS.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make(map[string]image.Image, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		f, err := iconFS.Open(path.Join(dir, e.Name()))
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

// LoadReferenceIcons returns a map of ability name → decoded image for every
// PNG in the embedded icons/ready directory.
func LoadReferenceIcons() map[string]image.Image { return loadIconDir("icons/ready") }

// LoadNotReadyIcons returns a map of ability name → decoded image for every
// PNG in the embedded icons/not_ready directory.
func LoadNotReadyIcons() map[string]image.Image { return loadIconDir("icons/not_ready") }
