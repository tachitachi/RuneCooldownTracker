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
	// A genuine timer digit pixel has three properties:
	//   1. Absolutely bright  — it is white text (luma near 1.0).
	//   2. Brighter than the reference icon at the same position — eliminates
	//      icons whose in-game rendering is slightly lighter than their PNG.
	//   3. Much brighter than its immediate live neighbours — the white stroke
	//      sits on a dark cooldown overlay, producing a sharp local edge.
	//      Smoothly-bright icon artwork fails this test even if it clears 1 & 2.
	const timerAbsLuma = 0.75    // minimum absolute luma
	const timerBrightDiff = 0.15 // minimum excess over baseline luma
	const timerEdgeDiff = 0.15   // minimum excess over mean of 4 live neighbours
	const timerMinPixels = 4     // qualifying pixels needed to confirm timer

	s := resizeTo(slot, detectSize)
	cx, cy := detectSize/2, detectSize/2
	timerRadius := float64(detectSize) / 4.0

	luma := func(img *image.RGBA, x, y int) float64 {
		r, g, b, _ := img.At(x, y).RGBA()
		return (float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114) / 65535.0
	}

	brightCount := 0
	for dy := -detectSize / 4; dy <= detectSize/4; dy++ {
		for dx := -detectSize / 4; dx <= detectSize/4; dx++ {
			if math.Sqrt(float64(dx*dx+dy*dy)) > timerRadius {
				continue
			}
			x, y := cx+dx, cy+dy
			liveLum := luma(s, x, y)
			if liveLum <= timerAbsLuma {
				continue
			}
			if liveLum-luma(baseline, x, y) <= timerBrightDiff {
				continue
			}
			// Edge check: pixel must be sharply brighter than its 4 neighbours.
			neighbourMean := (luma(s, x-1, y) + luma(s, x+1, y) +
				luma(s, x, y-1) + luma(s, x, y+1)) / 4.0
			if liveLum-neighbourMean > timerEdgeDiff {
				brightCount++
			}
		}
	}
	if brightCount >= timerMinPixels {
		return StateCooldown, brightCount
	}
	return StateReady, brightCount
}
