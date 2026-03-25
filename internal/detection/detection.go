package detection

import (
	"fmt"
	"image"
	"math"
	"sync"
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
	// ClickHint is the first user click inside the top-left ability slot,
	// in crop-relative physical pixels. When set, the phase search is constrained
	// so the click always falls inside a valid slot, which prevents the top row
	// from being skipped when its border is absent or weak.
	ClickHint        *image.Point
	OnLayoutDetected func(layout SlotLayout)

	frameMu   sync.RWMutex
	lastFrame *image.RGBA
}

func (ad *AbilityDetector) ProcessFrame(img *image.RGBA) {
	// Store a deep copy of the frame for on-demand export.
	ad.frameMu.Lock()
	copied := image.NewRGBA(img.Bounds())
	copy(copied.Pix, img.Pix)
	ad.lastFrame = copied
	ad.frameMu.Unlock()

	// Re-detect whenever the first frame arrives or the crop size changes.
	if ad.layout == nil || img.Rect != ad.lastRect {
		layout := detectGrid(img, ad.ClickHint)
		fmt.Printf("Got grid: %v\n", layout)
		ad.layout = &layout
		ad.lastRect = img.Rect
		if ad.OnLayoutDetected != nil {
			ad.OnLayoutDetected(layout)
		}
	}
	// TODO: per-slot cooldown detection using ad.layout
}

// GetLastFrame returns a snapshot of the most recently processed frame.
func (ad *AbilityDetector) GetLastFrame() *image.RGBA {
	ad.frameMu.RLock()
	defer ad.frameMu.RUnlock()
	return ad.lastFrame
}

func detectGrid(img *image.RGBA, hint *image.Point) SlotLayout {
	// Smooth the gradient projection to merge the double-spike each thick border
	// produces into a single broad peak before scoring.
	colProj := smooth(projection(img, true), 4)
	rowProj := smooth(projection(img, false), 4)

	colHint, rowHint := -1, -1
	if hint != nil {
		colHint = hint.X
		rowHint = hint.Y
	}

	colPeriod, colPhase := findPeriodAndPhase(colProj, 54, 54, colHint)
	rowPeriod, rowPhase := findPeriodAndPhase(rowProj, 58, 58, rowHint)

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
// relative to the overall mean.
//
// hint, when >= 0, is a position known to be inside the first slot (the first
// user click relative to the crop). Only phases satisfying
//   phase ≤ hint < phase + period
// are considered, which guarantees the detected grid includes a border above the
// first slot even when that border has no visible gradient (e.g. absent top row border).
func findPeriodAndPhase(proj []float64, minPeriod, maxPeriod, hint int) (period, phase int) {
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
		// Constrain phase range so the hint position falls inside the first slot:
		//   phase ≤ hint  AND  hint < phase + p  →  hint - p + 1 ≤ phase ≤ hint
		phMin, phMax := 0, p-1
		if hint >= 0 {
			if hint-p+1 > phMin {
				phMin = hint - p + 1
			}
			if hint < phMax {
				phMax = hint
			}
		}

		for ph := phMin; ph <= phMax; ph++ {
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
