package detection

import (
	"fmt"
	"image"
	"math"
)

// SlotLayout describes the detected grid in physical pixels relative to the crop region.
type SlotLayout struct {
	ColPeriod int // width of one slot in physical pixels
	ColPhase  int // x position of the first column separator
	RowPeriod int // height of one slot in physical pixels
	RowPhase  int // y position of the first row separator
}

// NumCols returns how many complete slot columns fit in the given width.
func (l SlotLayout) NumCols(width int) int {
	if l.ColPeriod == 0 {
		return 0
	}
	return (width - l.ColPhase) / l.ColPeriod
}

// NumRows returns how many complete slot rows fit in the given height.
func (l SlotLayout) NumRows(height int) int {
	if l.RowPeriod == 0 {
		return 0
	}
	return (height - l.RowPhase) / l.RowPeriod
}

// AbilityDetector detects the action bar slot grid and processes frames.
type AbilityDetector struct {
	layout           *SlotLayout
	lastRect         image.Rectangle
	OnLayoutDetected func(layout SlotLayout)
}

func (ad *AbilityDetector) ProcessFrame(img *image.RGBA) {
	// Re-detect whenever the first frame arrives or the crop size changes.
	if ad.layout == nil || img.Rect != ad.lastRect {
		layout := detectGrid(img)
		fmt.Printf("Got grid: %s\n", layout)
		ad.layout = &layout
		ad.lastRect = img.Rect
		if ad.OnLayoutDetected != nil {
			ad.OnLayoutDetected(layout)
		}
	}
	// TODO: per-slot cooldown detection using ad.layout
}

func detectGrid(img *image.RGBA) SlotLayout {
	// Smooth the gradient projection to merge the double-spike each thick border
	// produces into a single broad peak before scoring.
	colProj := smooth(projection(img, true), 4)
	rowProj := smooth(projection(img, false), 4)

	colPeriod, colPhase := findPeriodAndPhase(colProj, 54, 54)
	rowPeriod, rowPhase := findPeriodAndPhase(rowProj, 58, 58)

	return SlotLayout{
		ColPeriod: colPeriod,
		ColPhase:  colPhase,
		RowPeriod: rowPeriod,
		RowPhase:  rowPhase,
	}
}

// projection collapses the image to a 1D edge-energy profile along one axis.
// byColumn=true → result[x] = mean horizontal gradient energy of column x.
// byColumn=false → result[y] = mean vertical gradient energy of row y.
// Grid separators produce consistent high-gradient edges regardless of icon content.
func projection(img *image.RGBA, byColumn bool) []float64 {
	bounds := img.Rect
	w, h := bounds.Dx(), bounds.Dy()

	if byColumn {
		proj := make([]float64, w)
		for x := 1; x < w; x++ {
			var sum float64
			for y := 0; y < h; y++ {
				r1, g1, b1, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
				r0, g0, b0, _ := img.At(bounds.Min.X+x-1, bounds.Min.Y+y).RGBA()
				dr := math.Abs(float64(r1) - float64(r0))
				dg := math.Abs(float64(g1) - float64(g0))
				db := math.Abs(float64(b1) - float64(b0))
				sum += (dr + dg + db) / (3.0 * 65535.0)
			}
			proj[x] = sum / float64(h)
		}
		return proj
	}

	proj := make([]float64, h)
	for y := 1; y < h; y++ {
		var sum float64
		for x := 0; x < w; x++ {
			r1, g1, b1, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			r0, g0, b0, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y-1).RGBA()
			dr := math.Abs(float64(r1) - float64(r0))
			dg := math.Abs(float64(g1) - float64(g0))
			db := math.Abs(float64(b1) - float64(b0))
			sum += (dr + dg + db) / (3.0 * 65535.0)
		}
		proj[y] = sum / float64(w)
	}
	return proj
}

// smooth applies a box filter of the given radius to proj.
func smooth(proj []float64, radius int) []float64 {
	n := len(proj)
	out := make([]float64, n)
	for i := range proj {
		var sum float64
		var count int
		for d := -radius; d <= radius; d++ {
			j := i + d
			if j >= 0 && j < n {
				sum += proj[j]
				count++
			}
		}
		out[i] = sum / float64(count)
	}
	return out
}

// findPeriodAndPhase tries every (period, phase) pair in the candidate range and
// picks the one where the comb teeth have the highest average gradient energy
// relative to the overall mean. This directly measures "how well does a regular
// comb at this period+phase align with the actual separator edges?" and is robust
// to extra non-grid content at the edges of the capture area.
func findPeriodAndPhase(proj []float64, minPeriod, maxPeriod int) (period, phase int) {
	n := len(proj)
	if n == 0 || maxPeriod < minPeriod {
		return minPeriod, 0
	}

	var totalSum float64
	for _, v := range proj {
		totalSum += v
	}
	meanVal := totalSum / float64(n)
	if meanVal == 0 {
		return minPeriod, 0
	}

	bestScore := math.Inf(-1)
	bestPeriod, bestPhase := minPeriod, 0

	for p := minPeriod; p <= maxPeriod; p++ {
		for ph := 0; ph < p; ph++ {
			var sum float64
			var count int
			for pos := ph; pos < n; pos += p {
				sum += proj[pos]
				count++
			}
			// Require at least 3 teeth so a single bright spike can't win.
			if count < 3 {
				continue
			}
			// Score: mean tooth energy relative to the image average.
			// A score > 1 means the teeth are above the background level.
			score := (sum / float64(count)) / meanVal
			if score > bestScore {
				bestScore = score
				bestPeriod = p
				bestPhase = ph
			}
		}
	}
	return bestPeriod, bestPhase
}
