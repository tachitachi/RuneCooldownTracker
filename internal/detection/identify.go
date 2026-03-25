package detection

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"sync"

	xdraw "golang.org/x/image/draw"
)

// SlotKey identifies a grid position by column and row (0-indexed).
type SlotKey struct{ Col, Row int }

// slotReference holds the identified ability name for a slot.
type slotReference struct {
	name string
}

const (
	// nccSize is the canonical size both slot crops and reference icons are
	// resized to before computing NCC.
	nccSize = 48

	// nccThreshold is the minimum NCC score [0,1] to consider a slot matched.
	// NCC=1 means pixel-perfect match; typical good matches score 0.85+.
	nccThreshold = 0.70
)

// resizeTo returns img scaled to size×size using bilinear interpolation.
func resizeTo(img image.Image, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return dst
}

// nccScore computes the normalized cross-correlation between two same-sized
// RGBA images using luma (Y) channel only.
// Returns a value in [-1, 1] where 1 = identical, 0 = uncorrelated.
func nccScore(a, b *image.RGBA) float64 {
	n := a.Bounds().Dx() * a.Bounds().Dy()
	if n == 0 {
		return 0
	}

	// Compute means.
	var sumA, sumB float64
	for i := 0; i < n; i++ {
		off := i * 4
		ra, ga, ba := float64(a.Pix[off]), float64(a.Pix[off+1]), float64(a.Pix[off+2])
		rb, gb, bb := float64(b.Pix[off]), float64(b.Pix[off+1]), float64(b.Pix[off+2])
		sumA += 0.299*ra + 0.587*ga + 0.114*ba
		sumB += 0.299*rb + 0.587*gb + 0.114*bb
	}
	meanA := sumA / float64(n)
	meanB := sumB / float64(n)

	// Compute NCC numerator and denominators.
	var num, denA, denB float64
	for i := 0; i < n; i++ {
		off := i * 4
		ra, ga, ba := float64(a.Pix[off]), float64(a.Pix[off+1]), float64(a.Pix[off+2])
		rb, gb, bb := float64(b.Pix[off]), float64(b.Pix[off+1]), float64(b.Pix[off+2])
		ya := 0.299*ra + 0.587*ga + 0.114*ba - meanA
		yb := 0.299*rb + 0.587*gb + 0.114*bb - meanB
		num += ya * yb
		denA += ya * ya
		denB += yb * yb
	}
	den := math.Sqrt(denA * denB)
	if den == 0 {
		return 0
	}
	return num / den
}

// BuildRefImages precomputes resized reference icon images for NCC comparison.
func BuildRefImages(refs map[string]image.Image) map[string]*image.RGBA {
	out := make(map[string]*image.RGBA, len(refs))
	for name, img := range refs {
		out[name] = resizeTo(img, nccSize)
	}
	fmt.Printf("[identify] BuildRefImages: %d reference icons prepared\n", len(out))
	return out
}

// slotBorderPx is how many physical pixels to inset on each side of a raw
// slot crop. The RS ability bar has a visible border frame around each slot;
// stripping it gives a square icon region that better matches reference artwork.
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
	sub := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	draw.Draw(sub, sub.Bounds(), frame, image.Point{x0, y0}, draw.Src)
	return sub
}

// IdentifySlots runs Phase A: for every slot in the grid, finds the closest
// reference ability by NCC. Returns a SlotKey→slotReference map; unmatched
// slots get name="unknown".
func IdentifySlots(frame image.Image, layout SlotLayout, refImages map[string]*image.RGBA) map[SlotKey]slotReference {
	b := frame.Bounds()
	numCols := layout.NumCols(b.Dx())
	numRows := layout.NumRows(b.Dy())
	fmt.Printf("[identify] frame=%dx%d numCols=%d numRows=%d refs=%d\n",
		b.Dx(), b.Dy(), numCols, numRows, len(refImages))

	result := make(map[SlotKey]slotReference, numCols*numRows)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			wg.Add(1)
			go func(c, r int) {
				defer wg.Done()
				slot := cropSlot(frame, layout, c, r)
				ref := identifySlot(slot, refImages, c, r)
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

// identifySlot finds the best-matching reference for a single slot image using NCC.
func identifySlot(slot image.Image, refImages map[string]*image.RGBA, col, row int) slotReference {
	if len(refImages) == 0 {
		return slotReference{name: "unknown"}
	}
	slotResized := resizeTo(slot, nccSize)

	bestName := "unknown"
	bestScore := math.Inf(-1)
	for name, refImg := range refImages {
		score := nccScore(slotResized, refImg)
		if score > bestScore {
			bestScore = score
			bestName = name
		}
	}
	matched := bestScore >= nccThreshold
	if !matched {
		fmt.Printf("[identify] col=%d row=%d no match (best=%q ncc=%.3f < %.2f)\n", col, row, bestName, bestScore, nccThreshold)
		bestName = "unknown"
	} else {
		fmt.Printf("[identify] col=%d row=%d matched %q (ncc=%.3f)\n", col, row, bestName, bestScore)
	}
	return slotReference{name: bestName}
}
