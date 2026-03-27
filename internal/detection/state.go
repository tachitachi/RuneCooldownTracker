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
// Detection priority (highest to lowest):
//  1. Timer text: white number digits in the centre circle are brighter than
//     baseline → definitive StateCooldown regardless of anything else.
//  2. White flash: icon is much brighter than baseline overall → StateReady
//     (transition flash at cooldown expiry or GCD end).
//  3. MAE < threshold → StateReady (icon matches baseline).
//  4. NCC against not_ready icon (if available) or centre brightness heuristic
//     → StateCooldown vs StateNoResources.
//
// Returns (state, mae, brightDelta, darkDelta, brightness, timerDetected).
func detectSlotState(slot image.Image, baseline *image.RGBA, notReadyRef *image.RGBA) (state AbilityState, mae, brightDelta, darkDelta, brightness float64, timerDetected bool) {
	const maeThresholdReady = 0.05
	const brightnessThresholdCooldown = 180.0 / 255.0
	const nccThresholdNoResources = 0.70
	const whiteFlashThreshold = 0.15
	const whiteFlashRatio = 2.0
	// Timer text: white digit pixels are much brighter than the darkened baseline.
	// The centre circle (radius = maeMaskSize/4) is the region to check; it is
	// excluded from MAE specifically because the timer occupies that area.
	const timerBrightDiff = 0.25 // live luma must exceed baseline luma by this much
	const timerMinPixels = 4     // minimum qualifying pixels to confirm timer

	s := resizeTo(slot, maeMaskSize)
	r := baseline

	// Priority 1 — timer text detection.
	// Scan the centre circle for pixels that are significantly brighter than
	// baseline. The cooldown overlay darkens the icon while the white digit
	// strokes appear as isolated very-bright pixels against that dark background.
	cx, cy := maeMaskSize/2, maeMaskSize/2
	timerRadius := float64(maeMaskSize) / 4.0
	timerCount := 0
	for dy := -maeMaskSize / 4; dy <= maeMaskSize/4; dy++ {
		for dx := -maeMaskSize / 4; dx <= maeMaskSize/4; dx++ {
			if math.Sqrt(float64(dx*dx+dy*dy)) > timerRadius {
				continue
			}
			x, y := cx+dx, cy+dy
			lr, lg, lb, _ := s.At(x, y).RGBA()
			br, bg, bb, _ := r.At(x, y).RGBA()
			liveLum := (float64(lr)*0.299 + float64(lg)*0.587 + float64(lb)*0.114) / 65535.0
			baseLum := (float64(br)*0.299 + float64(bg)*0.587 + float64(bb)*0.114) / 65535.0
			if liveLum-baseLum > timerBrightDiff {
				timerCount++
			}
		}
	}
	if timerCount >= timerMinPixels {
		// Timer is present — ability is definitely on cooldown.
		// Compute mae/bright/dark for logging; brightness is not needed here.
		var sumB, sumD float64
		var n int
		for i, masked := range donutMask {
			if masked {
				continue
			}
			px := i % maeMaskSize
			py := i / maeMaskSize
			sr, sg, sb, _ := s.At(px, py).RGBA()
			rr, rg, rb, _ := r.At(px, py).RGBA()
			pix := ((float64(sr)-float64(rr)) + (float64(sg)-float64(rg)) + (float64(sb)-float64(rb))) / (3.0 * 65535.0)
			if pix > 0 {
				sumB += pix
			} else {
				sumD += -pix
			}
			n++
		}
		if n > 0 {
			brightDelta = sumB / float64(n)
			darkDelta = sumD / float64(n)
			mae = brightDelta + darkDelta
		}
		return StateCooldown, mae, brightDelta, darkDelta, 0, true
	}

	// Compute signed MAE over unmasked pixels, decomposed into bright and dark.
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
		pixAvg := ((float64(sr)-float64(rr)) + (float64(sg)-float64(rg)) + (float64(sb)-float64(rb))) / (3.0 * 65535.0)
		if pixAvg > 0 {
			sumBright += pixAvg
		} else {
			sumDark += -pixAvg
		}
		count++
	}
	if count == 0 {
		return StateUnknown, 0, 0, 0, 0, false
	}
	brightDelta = sumBright / float64(count)
	darkDelta = sumDark / float64(count)
	mae = brightDelta + darkDelta

	// Priority 2 — white flash → ready.
	if brightDelta > whiteFlashThreshold && brightDelta > darkDelta*whiteFlashRatio {
		return StateReady, mae, brightDelta, darkDelta, 0, false
	}

	// Priority 3 — MAE match → ready.
	if mae < maeThresholdReady {
		return StateReady, mae, brightDelta, darkDelta, 0, false
	}

	// Priority 4 — distinguish cooldown vs no-resources.
	if notReadyRef != nil {
		score := nccScore(s, notReadyRef)
		if score >= nccThresholdNoResources {
			return StateNoResources, mae, brightDelta, darkDelta, 0, false
		}
		return StateCooldown, mae, brightDelta, darkDelta, 0, false
	}

	// Fallback: centre brightness heuristic.
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
		return StateCooldown, mae, brightDelta, darkDelta, brightness, false
	}
	return StateNoResources, mae, brightDelta, darkDelta, brightness, false
}
