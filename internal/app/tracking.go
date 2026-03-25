package app

import (
	"math"
	"sync/atomic"

	"github.com/tachitachi/RuneCooldownTracker/internal/detection"
)

// StartTracking identifies the abilities in each slot (Phase A) and activates
// per-frame state detection (Phase B).
func (a *App) StartTracking() string {
	if a.detector == nil {
		return "Capture not running."
	}
	if a.currentLayout == nil {
		return "No grid layout detected yet — set a capture area first."
	}
	if a.detector.GetLastFrame() == nil {
		return "No frame captured yet — make sure the game is visible."
	}

	refs := detection.LoadReferenceIcons()
	refImages := detection.BuildRefImages(refs)

	a.detector.OnStateChange = func(changed detection.SlotStateMap) {
		a.emitSlotStates(changed)
	}
	a.detector.StartTracking(refImages)
	return "Tracking started."
}

// StopTracking stops per-frame state detection.
func (a *App) StopTracking() {
	if a.detector != nil {
		a.detector.StopTracking()
	}
}

// emitSlotStates converts slot positions to logical pixels and emits the
// tracking:states event to the overlay frontend.
func (a *App) emitSlotStates(states detection.SlotStateMap) {
	if a.currentLayout == nil {
		return
	}
	scale := math.Float64frombits(uint64(atomic.LoadInt64(&a.dpiScale)))
	if scale == 0 {
		scale = 1.0
	}
	crop := a.handler.GetCropRegion()
	if crop == nil {
		return
	}
	layout := *a.currentLayout

	type slotEvent struct {
		Col   int     `json:"col"`
		Row   int     `json:"row"`
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
		W     float64 `json:"w"`
		H     float64 `json:"h"`
		State int     `json:"state"`
	}

	slots := make([]slotEvent, 0, len(states))
	for key, state := range states {
		x0 := layout.ColPhase + key.Col*layout.ColPeriod
		y0 := layout.RowPhase + key.Row*layout.RowPeriod
		slots = append(slots, slotEvent{
			Col:   key.Col,
			Row:   key.Row,
			X:     float64(crop.X+x0) / scale,
			Y:     float64(crop.Y+y0) / scale,
			W:     float64(layout.ColPeriod) / scale,
			H:     float64(layout.RowPeriod) / scale,
			State: int(state),
		})
	}

	a.app.Event.Emit("tracking:states", slots)
}
