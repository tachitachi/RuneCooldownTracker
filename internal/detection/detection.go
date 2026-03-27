package detection

import (
	"fmt"
	"image"
	"math"
	"sync"
	"time"
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

// gcdDuration is the global cooldown window that follows any ability activation.
// During this window, all icons receive a shadow sweep animation but only the
// used ability has an actual cooldown timer.
const gcdDuration = 1800 * time.Millisecond

// gcdShadowThreshold is the minimum darkDelta for a slot to be considered
// affected by the GCD shadow sweep (as opposed to just noise).
const gcdShadowThreshold = 0.04

// gcdMajorityFrac is the minimum fraction of tracked slots that must show the
// shadow sweep simultaneously for it to be classified as a global cooldown rather
// than individual per-ability cooldowns. 0.75 = at least 75% of tracked slots.
const gcdMajorityFrac = 0.75

// AbilityDetector detects the action bar slot grid and processes frames.
type AbilityDetector struct {
	layout      *SlotLayout
	lastRect    image.Rectangle
	layoutFixed bool // when true, skip auto-detection until UnlockLayout is called
	// ClickHint is the first user click inside the top-left ability slot,
	// in crop-relative physical pixels. When set, the phase search is constrained
	// so the click always falls inside a valid slot, which prevents the top row
	// from being skipped when its border is absent or weak.
	ClickHint        *image.Point
	OnLayoutDetected func(layout SlotLayout)
	OnStateChange    func(states SlotStateMap)

	frameMu   sync.RWMutex
	lastFrame *image.RGBA

	trackingMu    sync.RWMutex
	tracking      bool
	slotRefs      map[SlotKey]slotReference
	refImages     map[string]*image.RGBA
	notReadyRefs  map[string]*image.RGBA // pre-resized not_ready icons, keyed by ability name
	slotBaselines map[SlotKey]*image.RGBA // game-pixel snapshot of each slot at tracking start
	lastStates    map[SlotKey]AbilityState
	gcdUntil      time.Time // wall-clock deadline of the current global cooldown window
}

func (ad *AbilityDetector) ProcessFrame(img *image.RGBA) {
	// Store a deep copy of the frame for on-demand export.
	ad.frameMu.Lock()
	copied := image.NewRGBA(img.Bounds())
	copy(copied.Pix, img.Pix)
	ad.lastFrame = copied
	ad.frameMu.Unlock()

	// Re-detect whenever the first frame arrives or the crop size changes,
	// unless a layout was explicitly set from saved config (layoutFixed).
	if !ad.layoutFixed && (ad.layout == nil || img.Rect != ad.lastRect) {
		layout := detectGrid(img, ad.ClickHint)
		fmt.Printf("Got grid: %v\n", layout)
		ad.layout = &layout
		ad.lastRect = img.Rect
		if ad.OnLayoutDetected != nil {
			ad.OnLayoutDetected(layout)
		}
	}

	// Phase B: per-frame state detection when tracking is active.
	ad.trackingMu.RLock()
	isTracking := ad.tracking
	refs := ad.slotRefs
	ad.trackingMu.RUnlock()

	if !isTracking || ad.layout == nil || len(refs) == 0 {
		return
	}

	ad.trackingMu.RLock()
	baselines := ad.slotBaselines
	notReadyRefs := ad.notReadyRefs
	ad.trackingMu.RUnlock()

	// Pass 1: compute raw detection results for every tracked slot and count
	// how many show the shadow-sweep dark delta.
	type slotResult struct {
		key        SlotKey
		ref        slotReference
		state      AbilityState
		mae        float64
		brightDelta float64
		darkDelta  float64
		brightness float64
	}
	results := make([]slotResult, 0, len(refs))
	darkCount := 0
	for key, ref := range refs {
		if ref.name == "unknown" {
			continue
		}
		baseline := baselines[key]
		if baseline == nil {
			continue
		}
		slot := cropSlot(img, *ad.layout, key.Col, key.Row)
		state, mae, brightDelta, darkDelta, brightness := detectSlotState(slot, baseline, notReadyRefs[ref.name])
		if darkDelta > gcdShadowThreshold {
			darkCount++
		}
		results = append(results, slotResult{key, ref, state, mae, brightDelta, darkDelta, brightness})
	}

	// GCD detection: if a supermajority of tracked slots show the sweep at the same
	// time, it is a global cooldown rather than individual per-ability cooldowns.
	// We latch gcdUntil so that suppression continues for the full GCD window even
	// if the sweep animation starts or ends unevenly across slots.
	trackedCount := len(results)
	gcdDetected := trackedCount >= 2 && float64(darkCount) >= float64(trackedCount)*gcdMajorityFrac
	ad.trackingMu.Lock()
	if gcdDetected {
		ad.gcdUntil = time.Now().Add(gcdDuration)
	}
	inGCD := time.Now().Before(ad.gcdUntil)
	ad.trackingMu.Unlock()

	// Pass 2: apply GCD suppression and emit state changes.
	changed := SlotStateMap{}
	for _, r := range results {
		state := r.state
		// During GCD every ability shows the sweep; suppress false not-ready reads.
		if inGCD && state == StateCooldown {
			state = StateReady
		}

		ad.trackingMu.RLock()
		prev, hasPrev := ad.lastStates[r.key]
		ad.trackingMu.RUnlock()

		if !hasPrev || prev != state {
			fmt.Printf("[state] col=%d row=%d %q: %s → %s (mae=%.4f bright=%.4f dark=%.4f brightness=%.3f gcd=%v darkCount=%d/%d)\n",
				r.key.Col, r.key.Row, r.ref.name, stateName(prev), stateName(state), r.mae, r.brightDelta, r.darkDelta, r.brightness, inGCD, darkCount, trackedCount)
			changed[r.key] = state
			ad.trackingMu.Lock()
			if ad.lastStates == nil {
				ad.lastStates = make(map[SlotKey]AbilityState)
			}
			ad.lastStates[r.key] = state
			ad.trackingMu.Unlock()
		}
	}

	if len(changed) > 0 && ad.OnStateChange != nil {
		ad.OnStateChange(changed)
	}
}

// GetLastFrame returns a snapshot of the most recently processed frame.
func (ad *AbilityDetector) GetLastFrame() *image.RGBA {
	ad.frameMu.RLock()
	defer ad.frameMu.RUnlock()
	return ad.lastFrame
}

// StartTracking runs Phase A identification on the most recent frame and
// activates per-frame Phase B state detection.
func (ad *AbilityDetector) StartTracking(refImages map[string]*image.RGBA, notReadyRefs map[string]*image.RGBA) {
	frame := ad.GetLastFrame()
	if frame == nil || ad.layout == nil {
		fmt.Println("[tracking] StartTracking: no frame or layout yet")
		return
	}
	fmt.Printf("[tracking] identifying slots (layout: colPeriod=%d colPhase=%d rowPeriod=%d rowPhase=%d)\n",
		ad.layout.ColPeriod, ad.layout.ColPhase, ad.layout.RowPeriod, ad.layout.RowPhase)
	refs := IdentifySlots(frame, *ad.layout, refImages)

	// Capture game-pixel baselines for each identified slot.
	// Comparing live frames against these baselines (same background, same rendering)
	// is far more reliable than comparing against embedded reference icons.
	baselines := make(map[SlotKey]*image.RGBA, len(refs))
	for key, ref := range refs {
		if ref.name != "unknown" {
			slot := cropSlot(frame, *ad.layout, key.Col, key.Row)
			baselines[key] = resizeTo(slot, maeMaskSize)
		}
	}

	ad.trackingMu.Lock()
	ad.slotRefs = refs
	ad.refImages = refImages
	ad.notReadyRefs = notReadyRefs
	ad.slotBaselines = baselines
	ad.lastStates = make(map[SlotKey]AbilityState)
	ad.tracking = true
	ad.trackingMu.Unlock()
	fmt.Printf("[tracking] started — tracking %d identified slots\n", func() int {
		n := 0
		for _, r := range refs {
			if r.name != "unknown" {
				n++
			}
		}
		return n
	}())
}

// StopTracking disables per-frame state detection.
func (ad *AbilityDetector) StopTracking() {
	ad.trackingMu.Lock()
	ad.tracking = false
	ad.trackingMu.Unlock()
	fmt.Println("[tracking] stopped")
}

// IsTracking reports whether state detection is currently active.
func (ad *AbilityDetector) IsTracking() bool {
	ad.trackingMu.RLock()
	defer ad.trackingMu.RUnlock()
	return ad.tracking
}

// SetLayout overwrites the detector's active grid layout. Call this whenever
// the user fine-tunes the layout via the config D-pad so that cropSlot uses
// the correct coordinates for identification and state detection.
func (ad *AbilityDetector) SetLayout(layout SlotLayout) {
	ad.layout = &layout
}

// SetLayoutAndBounds sets the layout and locks auto-detection, preventing
// ProcessFrame from overwriting it with a freshly detected grid.
// Call UnlockLayout when the user draws a new capture area.
func (ad *AbilityDetector) SetLayoutAndBounds(layout SlotLayout, rect image.Rectangle) {
	ad.layout = &layout
	ad.lastRect = rect
	ad.layoutFixed = true
}

// UnlockLayout re-enables auto-detection on the next frame. Call this when
// the user sets a new capture area so the grid can be re-detected.
func (ad *AbilityDetector) UnlockLayout() {
	ad.layoutFixed = false
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
