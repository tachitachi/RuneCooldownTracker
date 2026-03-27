package detection

import (
	"image"
	"math"
)

// AbilityState represents the current state of an ability slot.
type AbilityState int

const (
	StateReady       AbilityState = iota // icon matches reference — ability is available
	StateCooldown                        // icon is darkened/spinning — on cooldown
	StateNoResources                     // icon is dimmed differently — insufficient resources
	StateUnknown                         // unidentified slot or detection error
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

// maeMaskSize is the canonical size for MAE comparison.
const maeMaskSize = 48

// donutMask is a precomputed mask for maeMaskSize×maeMaskSize.
// true = masked (skip), false = use in MAE.
var donutMask = buildDonutMask(maeMaskSize)

// buildDonutMask builds a mask that excludes:
//   - a central circle covering the inner 25% of area (radius = size/4)
//   - corner triangles covering ~15% of each corner (triangle leg = size/5)
func buildDonutMask(size int) []bool {
	mask := make([]bool, size*size)
	cx := float64(size-1) / 2.0
	cy := float64(size-1) / 2.0
	// Center circle: radius covers ~25% of area → r = size/4
	r := float64(size) / 4.0
	// Corner triangle leg length: ~15% linear coverage → leg = size/5
	leg := size / 5

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			// Center circle
			if math.Sqrt(dx*dx+dy*dy) < r {
				mask[y*size+x] = true
				continue
			}
			// Corner triangles: top-left, top-right, bottom-left, bottom-right
			if x < leg && y < leg-x {
				mask[y*size+x] = true
				continue
			}
			if x >= size-leg && y < leg-(size-1-x) {
				mask[y*size+x] = true
				continue
			}
			if x < leg && y >= size-leg+x {
				mask[y*size+x] = true
				continue
			}
			if x >= size-leg && y >= size-leg+(size-1-x) {
				mask[y*size+x] = true
				continue
			}
		}
	}
	return mask
}

// detectSlotState compares a live slot crop against the game-pixel baseline
// captured at tracking start to determine the current ability state.
//
// Algorithm:
//  1. Resize slot to maeMaskSize × maeMaskSize (baseline is already that size).
//  2. Compute signed MAE over non-masked pixels (excludes centre timer area and corners),
//     decomposed into brightDelta (live brighter than baseline) and darkDelta (live darker).
//  3. If brightDelta dominates and exceeds a threshold → white flash → StateReady.
//  4. If total MAE < maeThresholdReady → StateReady.
//  5. Otherwise sample centre brightness: bright → StateCooldown (spinning
//     timer overlay is present), dim → StateNoResources.
//
// Returns (state, mae, brightDelta, darkDelta, brightness) for debug logging.
func detectSlotState(slot image.Image, baseline *image.RGBA, notReadyRef *image.RGBA) (state AbilityState, mae, brightDelta, darkDelta, brightness float64) {
	// Game-to-game comparison: threshold can be tight because there is no
	// background colour mismatch between slot and baseline.
	const maeThresholdReady = 0.05
	const brightnessThresholdCooldown = 180.0 / 255.0
	const nccThresholdNoResources = 0.70
	// White flash: when an ability becomes ready (or GCD ends), the icon briefly
	// goes much brighter than baseline. Detect this as a positive ready signal.
	const whiteFlashThreshold = 0.15
	const whiteFlashRatio = 2.0

	s := resizeTo(slot, maeMaskSize)
	r := baseline

	// Compute signed MAE over unmasked pixels, decomposed into bright and dark components.
	var sumBright, sumDark float64
	var count int
	for i, masked := range donutMask {
		if masked {
			continue
		}
		x := i % maeMaskSize
		y := i / maeMaskSize
		sr, sg, sb, _ := s.At(x, y).RGBA()
		rr, rg, rb, _ := r.At(x, y).RGBA()
		// Signed per-pixel average: positive means live is brighter than baseline.
		pixAvg := ((float64(sr)-float64(rr)) + (float64(sg)-float64(rg)) + (float64(sb)-float64(rb))) / (3.0 * 65535.0)
		if pixAvg > 0 {
			sumBright += pixAvg
		} else {
			sumDark += -pixAvg
		}
		count++
	}
	if count == 0 {
		return StateUnknown, 0, 0, 0, 0
	}
	brightDelta = sumBright / float64(count)
	darkDelta = sumDark / float64(count)
	mae = brightDelta + darkDelta // identical total to the old |diff| / count

	// White flash: icon is significantly brighter than baseline with only minor
	// dark regions. This occurs when an ability transitions to ready (individual
	// cooldown expiry or GCD end flash). Treat as ready even though MAE is elevated.
	if brightDelta > whiteFlashThreshold && brightDelta > darkDelta*whiteFlashRatio {
		return StateReady, mae, brightDelta, darkDelta, 0
	}

	if mae < maeThresholdReady {
		return StateReady, mae, brightDelta, darkDelta, 0
	}

	// Distinguish cooldown vs no-resources.
	if notReadyRef != nil {
		// s is 48×48 (= nccSize), so it can be passed directly to nccScore.
		// High NCC against the not_ready reference → looks like no-resources (no timer).
		// Low NCC → a spinning timer overlay is present → cooldown.
		score := nccScore(s, notReadyRef)
		if score >= nccThresholdNoResources {
			return StateNoResources, mae, brightDelta, darkDelta, 0
		}
		return StateCooldown, mae, brightDelta, darkDelta, 0
	}

	// Fallback: centre brightness heuristic (no not_ready icon for this ability).
	cx, cy := maeMaskSize/2, maeMaskSize/2
	var brightnessSum float64
	samples := 0
	for dy := -3; dy <= 3; dy++ {
		for dx := -3; dx <= 3; dx++ {
			r8, g8, b8, _ := s.At(cx+dx, cy+dy).RGBA()
			lum := (float64(r8)*0.299 + float64(g8)*0.587 + float64(b8)*0.114) / 65535.0
			brightnessSum += lum
			samples++
		}
	}
	brightness = brightnessSum / float64(samples)
	if brightness > brightnessThresholdCooldown {
		return StateCooldown, mae, brightDelta, darkDelta, brightness
	}
	return StateNoResources, mae, brightDelta, darkDelta, brightness
}
