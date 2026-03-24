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
	colProj := projection(img, true)  // per-column mean brightness
	rowProj := projection(img, false) // per-row mean brightness

	colPeriod := findPeriod(colProj, 16)
	rowPeriod := findPeriod(rowProj, 16)

	colPhase := findPhase(colProj, colPeriod)
	rowPhase := findPhase(rowProj, rowPeriod)

	return SlotLayout{
		ColPeriod: colPeriod,
		ColPhase:  colPhase,
		RowPeriod: rowPeriod,
		RowPhase:  rowPhase,
	}
}

// projection collapses the image to a 1D brightness profile along one axis.
// byColumn=true → result[x] = mean brightness of column x.
// byColumn=false → result[y] = mean brightness of row y.
func projection(img *image.RGBA, byColumn bool) []float64 {
	bounds := img.Rect
	w, h := bounds.Dx(), bounds.Dy()

	size, other := w, h
	if !byColumn {
		size, other = h, w
	}

	proj := make([]float64, size)
	for i := 0; i < size; i++ {
		var sum float64
		for j := 0; j < other; j++ {
			var x, y int
			if byColumn {
				x, y = bounds.Min.X+i, bounds.Min.Y+j
			} else {
				x, y = bounds.Min.X+j, bounds.Min.Y+i
			}
			r, g, b, _ := img.At(x, y).RGBA()
			sum += (float64(r) + float64(g) + float64(b)) / (3.0 * 65535.0)
		}
		proj[i] = sum / float64(other)
	}
	return proj
}

// findPeriod finds the dominant period in proj using autocorrelation.
// minPeriod is the smallest slot size (in pixels) to consider.
func findPeriod(proj []float64, minPeriod int) int {
	n := len(proj)
	maxPeriod := n / 2
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

// findPhase folds the projection modulo the period and returns the index of the
// brightest recurring feature — typically the slot separator.
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
