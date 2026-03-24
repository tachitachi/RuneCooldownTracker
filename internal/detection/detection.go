package detection

import (
	"fmt"
	"image"
)

type AbilityDetector struct {
}

func (ad *AbilityDetector) ProcessFrame(img *image.RGBA) {
	fmt.Printf("Got image: %s", img.Rect)
}
