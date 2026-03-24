package detection

import (
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
		ad.layout = &layout
		ad.lastRect = img.Rect
		if ad.OnLayoutDetected != nil {
			ad.OnLayoutDetected(layout)
		}
	}
	// TODO: per-slot cooldown detection using ad.layout
}

func detectGrid(img *image.RGBA) SlotLayout {
	colProj := smooth(projection(img, true), 8)  // per-column edge energy, smoothed
	rowProj := smooth(projection(img, false), 8) // per-row edge energy, smoothed

	colPeriod := findPeriod(colProj, 40, 60)
	rowPeriod := findPeriod(rowProj, 40, 60)

	colPhase := findPhase(colProj, colPeriod)
	rowPhase := findPhase(rowProj, rowPeriod)

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

// findPeriod finds the dominant period in proj using autocorrelation.
// minPeriod is the smallest slot size (in pixels) to consider.
func findPeriod(proj []float64, minPeriod, maxPeriod int) int {
	n := len(proj)
	// maxPeriod := n / 2
	if maxPeriod < minPeriod {
		return n // not enough data — treat whole axis as one slot
	}

	var mean float64
	for _, v := range proj {
		mean += v
	}
	mean /= float64(n)

	bestLag, bestVal := minPeriod, math.Inf(-1)
	for lag := minPeriod; lag <= maxPeriod; lag++ {
		count := n - lag
		var sum float64
		for i := 0; i < count; i++ {
			sum += (proj[i] - mean) * (proj[i+lag] - mean)
		}
		val := sum / float64(count)
		if val > bestVal {
			bestVal = val
			bestLag = lag
		}
	}
	return bestLag
}

// smooth applies a box filter of the given half-width to proj.
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

// findPhase folds the projection modulo the period and returns the index of the
// highest edge energy — the slot separator position.
func findPhase(proj []float64, period int) int {
	if period <= 0 {
		return 0
	}
	folded := make([]float64, period)
	for i, v := range proj {
		folded[i%period] += v
	}
	maxVal, maxIdx := math.Inf(-1), 0
	for i, v := range folded {
		if v > maxVal {
			maxVal = v
			maxIdx = i
		}
	}
	return maxIdx
}
