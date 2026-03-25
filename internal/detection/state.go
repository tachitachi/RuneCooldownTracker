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

// detectSlotState compares a live slot crop against its reference icon to
// determine the current ability state.
//
// Algorithm:
//  1. Resize both to maeMaskSize × maeMaskSize.
//  2. Compute MAE over non-masked pixels (normalised to [0,1]).
//  3. If MAE < maeThresholdReady → StateReady.
//  4. Otherwise sample centre brightness: bright → StateCooldown (spinning
//     timer overlay is present), dim → StateNoResources.
func detectSlotState(slot, ref image.Image) AbilityState {
	const maeThresholdReady = 0.08
	const brightnessThresholdCooldown = 180.0 / 255.0

	s := resizeTo(slot, maeMaskSize)
	r := resizeTo(ref, maeMaskSize)

	// Compute MAE over unmasked pixels.
	var sumErr float64
	var count int
	for i, masked := range donutMask {
		if masked {
			continue
		}
		x := i % maeMaskSize
		y := i / maeMaskSize
		sr, sg, sb, _ := s.At(x, y).RGBA()
		rr, rg, rb, _ := r.At(x, y).RGBA()
		dr := math.Abs(float64(sr)-float64(rr)) / 65535.0
		dg := math.Abs(float64(sg)-float64(rg)) / 65535.0
		db := math.Abs(float64(sb)-float64(rb)) / 65535.0
		sumErr += (dr + dg + db) / 3.0
		count++
	}
	if count == 0 {
		return StateUnknown
	}
	mae := sumErr / float64(count)
	if mae < maeThresholdReady {
		return StateReady
	}

	// Distinguish cooldown vs no-resources by centre brightness.
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
	avgBrightness := brightnessSum / float64(samples)
	if avgBrightness > brightnessThresholdCooldown {
		return StateCooldown
	}
	return StateNoResources
}
