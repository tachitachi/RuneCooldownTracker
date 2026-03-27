package app

import (
	"encoding/base64"
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
	notReadyRefs := detection.BuildRefImages(detection.LoadNotReadyIcons())

	a.detector.OnStateChange = func(changed detection.SlotStateMap) {
		a.emitSlotStates(changed)
	}
	a.detector.StartTracking(refImages, notReadyRefs)
	return "Tracking started."
}

// GetDetectionParams returns the current timer-detection hyperparameters.
func (a *App) GetDetectionParams() detection.DetectionParams {
	if a.detector == nil {
		return detection.DefaultDetectionParams()
	}
	return a.detector.GetDetectionParams()
}

// SetDetectionParams updates the timer-detection hyperparameters live (no
// restart required) and persists them to config.json.
func (a *App) SetDetectionParams(params detection.DetectionParams) {
	if a.detector == nil {
		return
	}
	a.detector.SetDetectionParams(params)
	a.saveConfig()
}

// StopTracking stops per-frame state detection.
func (a *App) StopTracking() {
	if a.detector != nil {
		a.detector.StopTracking()
	}
}

// GetAbilityIcon returns the ready-state PNG icon for the named ability as a
// base64-encoded string, suitable for use as a data URL in the frontend.
func (a *App) GetAbilityIcon(name string) string {
	data := detection.ReadyIconPNG(name)
	if data == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

// GetTrackedAbilityNames returns the sorted list of ability names currently
// identified in tracked slots.
func (a *App) GetTrackedAbilityNames() []string {
	if a.detector == nil {
		return nil
	}
	return a.detector.GetTrackedAbilityNames()
}

// GetAbilityDetectionParams returns the detection parameters for a specific
// ability. Falls back to the global params if no per-ability override exists.
func (a *App) GetAbilityDetectionParams(name string) detection.DetectionParams {
	if a.detector == nil {
		return detection.DefaultDetectionParams()
	}
	if p, ok := a.detector.GetAbilityParams(name); ok {
		return p
	}
	return a.detector.GetDetectionParams()
}

// SetAbilityDetectionParams stores a per-ability detection parameter override
// and persists it to config.
func (a *App) SetAbilityDetectionParams(name string, params detection.DetectionParams) {
	if a.detector == nil {
		return
	}
	a.detector.SetAbilityParams(name, params)
	a.saveConfig()
}

// SetAllAbilityDetectionParams applies the given params to every currently
// tracked ability and persists to config.
func (a *App) SetAllAbilityDetectionParams(params detection.DetectionParams) {
	if a.detector == nil {
		return
	}
	a.detector.SetAllAbilityParams(params)
	a.saveConfig()
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
