package app

import (
	"math"
	"sync/atomic"

	"github.com/tachitachi/RuneCooldownTracker/internal/detection"
)

// emitGridLines converts a SlotLayout to logical-pixel line positions and emits
// them to the overlay frontend via the grid:detected event.
func (a *App) emitGridLines(layout detection.SlotLayout) {
	scale := math.Float64frombits(uint64(atomic.LoadInt64(&a.dpiScale)))
	if scale == 0 {
		scale = 1.0
	}
	crop := a.handler.GetCropRegion()
	if crop == nil {
		return
	}

	var xLines []float64
	for x := layout.ColPhase; x <= crop.W; x += layout.ColPeriod {
		if x >= 0 {
			xLines = append(xLines, float64(crop.X+x)/scale)
		}
	}
	var yLines []float64
	for y := layout.RowPhase; y <= crop.H; y += layout.RowPeriod {
		if y >= 0 {
			yLines = append(yLines, float64(crop.Y+y)/scale)
		}
	}

	a.app.Event.Emit("grid:detected", map[string]any{
		"xLines": xLines,
		"yLines": yLines,
	})
}

// AdjustGridLayout nudges the current grid layout by the given deltas (physical pixels)
// and immediately re-emits the updated grid lines to the overlay.
// colPhaseDelta/rowPhaseDelta shift the grid left/right or up/down.
// colPeriodDelta/rowPeriodDelta widen or narrow the slot spacing.
func (a *App) AdjustGridLayout(colPhaseDelta, colPeriodDelta, rowPhaseDelta, rowPeriodDelta int) {
	if a.currentLayout == nil {
		return
	}
	layout := *a.currentLayout
	layout.ColPhase += colPhaseDelta
	layout.ColPeriod += colPeriodDelta
	layout.RowPhase += rowPhaseDelta
	layout.RowPeriod += rowPeriodDelta
	a.currentLayout = &layout
	a.emitGridLines(layout)
}
