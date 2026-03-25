package detection

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"sync"

	"github.com/corona10/goimagehash"
	xdraw "golang.org/x/image/draw"
)

// SlotKey identifies a grid position by column and row (0-indexed).
type SlotKey struct{ Col, Row int }

// slotReference holds the identified ability for a slot and its pHash for
// comparison.
type slotReference struct {
	name string
	hash *goimagehash.ImageHash
}

const (
	// phashSize is the canonical size we resize slots and reference icons to
	// before computing pHash.
	phashSize = 48

	// phashThreshold is the maximum Hamming distance considered a match.
	phashThreshold = 15
)

// resizeTo returns img scaled to size×size using bilinear interpolation.
func resizeTo(img image.Image, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return dst
}

// hashImage computes the perceptual hash of img after resizing to phashSize.
func hashImage(img image.Image) (*goimagehash.ImageHash, error) {
	return goimagehash.PerceptionHash(resizeTo(img, phashSize))
}

// BuildRefHashes precomputes pHashes for all reference icons.
func BuildRefHashes(refs map[string]image.Image) map[string]*goimagehash.ImageHash {
	out := make(map[string]*goimagehash.ImageHash, len(refs))
	for name, img := range refs {
		h, err := hashImage(img)
		if err == nil {
			out[name] = h
		}
	}
	return out
}

// cropSlot extracts the image region for a single slot from a full frame.
func cropSlot(frame image.Image, layout SlotLayout, col, row int) image.Image {
	b := frame.Bounds()
	x0 := b.Min.X + layout.ColPhase + col*layout.ColPeriod
	y0 := b.Min.Y + layout.RowPhase + row*layout.RowPeriod
	x1 := x0 + layout.ColPeriod
	y1 := y0 + layout.RowPeriod
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	// Materialise the sub-image as a new RGBA so downstream code can read it freely.
	sub := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	draw.Draw(sub, sub.Bounds(), frame, image.Point{x0, y0}, draw.Src)
	return sub
}

// IdentifySlots runs Phase A: for every slot in the grid, finds the closest
// reference ability by pHash Hamming distance. Returns a SlotKey→slotReference
// map; unmatched slots get name="unknown" and hash=nil.
func IdentifySlots(frame image.Image, layout SlotLayout, refHashes map[string]*goimagehash.ImageHash) map[SlotKey]slotReference {
	b := frame.Bounds()
	numCols := layout.NumCols(b.Dx())
	numRows := layout.NumRows(b.Dy())

	result := make(map[SlotKey]slotReference, numCols*numRows)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			wg.Add(1)
			go func(c, r int) {
				defer wg.Done()
				slot := cropSlot(frame, layout, c, r)
				ref := identifySlot(slot, refHashes)
				mu.Lock()
				result[SlotKey{Col: c, Row: r}] = ref
				mu.Unlock()
			}(col, row)
		}
	}
	wg.Wait()
	identified := 0
	for _, ref := range result {
		if ref.name != "unknown" {
			identified++
		}
	}
	fmt.Printf("[identify] done: %d/%d slots identified\n", identified, len(result))
	return result
}

// identifySlot finds the best-matching reference for a single slot image.
func identifySlot(slot image.Image, refHashes map[string]*goimagehash.ImageHash) slotReference {
	h, err := hashImage(slot)
	if err != nil || len(refHashes) == 0 {
		return slotReference{name: "unknown"}
	}

	bestName := "unknown"
	bestDist := math.MaxInt32
	for name, refHash := range refHashes {
		d, err := h.Distance(refHash)
		if err != nil {
			continue
		}
		if d < bestDist {
			bestDist = d
			bestName = name
		}
	}
	matched := bestDist <= phashThreshold
	if !matched {
		fmt.Printf("[identify] no match (best=%q dist=%d > threshold=%d)\n", bestName, bestDist, phashThreshold)
		bestName = "unknown"
	} else {
		fmt.Printf("[identify] matched %q (dist=%d)\n", bestName, bestDist)
	}
	return slotReference{name: bestName, hash: h}
}
