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
		if err != nil {
			fmt.Printf("[identify] BuildRefHashes: hash error for %q: %v\n", name, err)
			continue
		}
		out[name] = h
	}
	fmt.Printf("[identify] BuildRefHashes: %d/%d icons hashed successfully\n", len(out), len(refs))
	return out
}

// slotBorderPx is how many physical pixels to inset on each side of a raw
// slot crop before hashing/comparing. The RS ability bar has a visible border
// frame around each slot; stripping it gives a square icon region that better
// matches the reference icon artwork.
const slotBorderPx = 3

// cropSlot extracts the image region for a single slot from a full frame,
// inset by slotBorderPx to remove the slot border.
func cropSlot(frame image.Image, layout SlotLayout, col, row int) image.Image {
	b := frame.Bounds()
	x0 := b.Min.X + layout.ColPhase + col*layout.ColPeriod + slotBorderPx
	y0 := b.Min.Y + layout.RowPhase + row*layout.RowPeriod + slotBorderPx
	x1 := b.Min.X + layout.ColPhase + (col+1)*layout.ColPeriod - slotBorderPx
	y1 := b.Min.Y + layout.RowPhase + (row+1)*layout.RowPeriod - slotBorderPx
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	if x1 <= x0 || y1 <= y0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
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
	fmt.Printf("[identify] frame=%dx%d numCols=%d numRows=%d refHashes=%d\n",
		b.Dx(), b.Dy(), numCols, numRows, len(refHashes))

	result := make(map[SlotKey]slotReference, numCols*numRows)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			wg.Add(1)
			go func(c, r int) {
				defer wg.Done()
				slot := cropSlot(frame, layout, c, r)
				ref := identifySlot(slot, refHashes, c, r)
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
func identifySlot(slot image.Image, refHashes map[string]*goimagehash.ImageHash, col, row int) slotReference {
	if len(refHashes) == 0 {
		fmt.Println("[identify] refHashes is empty — no reference icons loaded")
		return slotReference{name: "unknown"}
	}
	h, err := hashImage(slot)
	if err != nil {
		fmt.Printf("[identify] col=%d row=%d hashImage error: %v (slot bounds: %v)\n", col, row, err, slot.Bounds())
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
		fmt.Printf("[identify] col=%d row=%d no match (best=%q dist=%d > threshold=%d)\n", col, row, bestName, bestDist, phashThreshold)
		bestName = "unknown"
	} else {
		fmt.Printf("[identify] col=%d row=%d matched %q (dist=%d)\n", col, row, bestName, bestDist)
	}
	return slotReference{name: bestName, hash: h}
}
