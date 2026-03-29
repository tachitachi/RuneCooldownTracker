package app

import (
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"sync/atomic"

	"github.com/tachitachi/RuneCooldownTracker/internal/detection"
)

// GridSlot holds the grid position and identified ability name for a single slot.
type GridSlot struct {
	Col  int    `json:"col"`
	Row  int    `json:"row"`
	Name string `json:"name"` // ability name, or "" when unknown
}

// AutoDetectAbilities runs Phase A slot identification on the current frame
// without starting tracking. Call StartTracking separately to begin tracking.
func (a *App) AutoDetectAbilities() string {
	if a.detector == nil {
		return "Capture not running."
	}
	if a.currentLayout == nil {
		return "No grid layout detected yet — set a capture area first."
	}
	if a.detector.GetLastFrame() == nil {
		return "No frame captured yet — make sure the game is visible."
	}

	a.detector.IdentifyAbilities(a.refImages, a.notReadyRefs)
	a.saveCurrentToActiveProfile()
	return "Auto-detect complete."
}

// StartTracking activates per-frame state detection (Phase B) using whatever
// slot assignments are currently set. Use AutoDetectAbilities first to identify
// abilities, or assign them manually via SetSlotAbility.
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

	slots := a.detector.GetSlotRefs()
	identified := 0
	for _, name := range slots {
		if name != "unknown" && name != "" {
			identified++
		}
	}
	if identified == 0 {
		return "No abilities identified — use Auto-detect first."
	}

	a.detector.OnStateChange = func(changed detection.SlotStateMap) {
		a.emitSlotStates(changed)
	}
	a.detector.StartTracking()
	return fmt.Sprintf("Tracking started (%d abilities).", identified)
}

// GetGridSlots returns every slot position with its current ability assignment.
func (a *App) GetGridSlots() []GridSlot {
	if a.detector == nil || a.currentLayout == nil {
		return nil
	}
	crop := a.handler.GetCropRegion()
	if crop == nil {
		return nil
	}
	layout := *a.currentLayout
	numCols := layout.NumCols(crop.W)
	numRows := layout.NumRows(crop.H)

	refs := a.detector.GetSlotRefs()
	slots := make([]GridSlot, 0, numCols*numRows)
	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			name := refs[detection.SlotKey{Col: col, Row: row}]
			if name == "unknown" {
				name = ""
			}
			slots = append(slots, GridSlot{Col: col, Row: row, Name: name})
		}
	}
	return slots
}

// SetSlotAbility manually assigns an ability to a grid slot and persists the
// change to the active profile.
func (a *App) SetSlotAbility(col, row int, name string) {
	if a.detector == nil {
		return
	}
	a.detector.SetSlotRef(detection.SlotKey{Col: col, Row: row}, name, a.refImages)
	a.saveCurrentToActiveProfile()
	a.saveConfig()
}

// ClearSlotAbility removes the ability assignment from a grid slot and persists
// the change to the active profile.
func (a *App) ClearSlotAbility(col, row int) {
	if a.detector == nil {
		return
	}
	a.detector.ClearSlotRef(detection.SlotKey{Col: col, Row: row})
	a.saveCurrentToActiveProfile()
	a.saveConfig()
}

// GetAllAbilityNames returns the names of all embedded reference icons, sorted
// alphabetically. Used to populate the manual-assignment dropdown.
func (a *App) GetAllAbilityNames() []string {
	refs := detection.LoadReferenceIcons()
	names := make([]string, 0, len(refs))
	for name := range refs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
		Name  string  `json:"name"`
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
		W     float64 `json:"w"`
		H     float64 `json:"h"`
		State int     `json:"state"`
	}

	slotRefs := a.detector.GetSlotRefs()
	slots := make([]slotEvent, 0, len(states))
	for key, state := range states {
		x0 := layout.ColPhase + key.Col*layout.ColPeriod
		y0 := layout.RowPhase + key.Row*layout.RowPeriod
		slots = append(slots, slotEvent{
			Col:   key.Col,
			Row:   key.Row,
			Name:  slotRefs[key],
			X:     float64(crop.X+x0) / scale,
			Y:     float64(crop.Y+y0) / scale,
			W:     float64(layout.ColPeriod) / scale,
			H:     float64(layout.RowPeriod) / scale,
			State: int(state),
		})
	}

	a.app.Event.Emit("tracking:states", slots)
}
