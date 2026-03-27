package detection

import (
	"image"
	"math"
)

// AbilityState represents the current state of an ability slot.
type AbilityState int

const (
	StateReady       AbilityState = iota // timer absent — ability is available
	StateCooldown                        // white timer digits visible in centre — on cooldown
	StateNoResources                     // kept for API compatibility; not returned by detection
	StateUnknown                         // unidentified slot
)

// SlotStateMap maps grid positions to their current detected state.
type SlotStateMap map[SlotKey]AbilityState

// stateName returns a human-readable label for an AbilityState.
func stateName(s AbilityState) string {
	switch s {
	case StateReady:
		return "ready"
	case StateCooldown:
		return "cooldown"
	case StateNoResources:
		return "no_resources"
	default:
		return "unknown"
	}
}

// detectSize is the canonical size slot crops are resized to for state detection.
const detectSize = 48

// detectSlotState determines whether an ability slot is on cooldown by scanning
// the centre circle for white countdown timer digits.
//
// baseline is the pre-resized ready-state reference icon for this ability.
// Comparing live pixels against it removes false positives from icons that
// naturally have bright centres, and correctly handles tracking that starts
// while an ability is already on cooldown (the reference always represents
// the ready appearance, not the captured game state at tracking start).
//
// Returns the detected state and the raw bright-pixel count for debug logging.
func detectSlotState(slot image.Image, baseline *image.RGBA) (AbilityState, int) {
	// A timer digit pixel must be both absolutely bright (it is white text) and
	// significantly brighter than the reference icon at the same position.
	// Using both guards avoids false positives from icons whose in-game rendering
	// is slightly brighter than the reference PNG but nowhere near timer-white.
	const timerAbsLuma = 0.75  // pixel must be at least this bright in absolute terms
	const timerBrightDiff = 0.35 // and must also exceed baseline luma by this much
	const timerMinPixels = 4     // minimum qualifying pixels to confirm timer presence

	s := resizeTo(slot, detectSize)
	cx, cy := detectSize/2, detectSize/2
	timerRadius := float64(detectSize) / 4.0

	brightCount := 0
	for dy := -detectSize / 4; dy <= detectSize/4; dy++ {
		for dx := -detectSize / 4; dx <= detectSize/4; dx++ {
			if math.Sqrt(float64(dx*dx+dy*dy)) > timerRadius {
				continue
			}
			x, y := cx+dx, cy+dy
			lr, lg, lb, _ := s.At(x, y).RGBA()
			br, bg, bb, _ := baseline.At(x, y).RGBA()
			liveLum := (float64(lr)*0.299 + float64(lg)*0.587 + float64(lb)*0.114) / 65535.0
			baseLum := (float64(br)*0.299 + float64(bg)*0.587 + float64(bb)*0.114) / 65535.0
			if liveLum > timerAbsLuma && liveLum-baseLum > timerBrightDiff {
				brightCount++
			}
		}
	}
	if brightCount >= timerMinPixels {
		return StateCooldown, brightCount
	}
	return StateReady, brightCount
}
