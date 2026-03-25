package app

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
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
	if a.detector != nil {
		a.detector.SetLayout(layout)
	}
	a.emitGridLines(layout)
}

// ExportIcons opens a folder-picker dialog, then saves each grid slot from the
// most recent captured frame as icon_{col}_{row}.png in the chosen directory.
// Returns a status message suitable for displaying to the user.
func (a *App) ExportIcons() string {
	if a.currentLayout == nil {
		return "No grid layout detected yet — set a capture area first."
	}
	if a.detector == nil {
		return "Capture not running."
	}
	frame := a.detector.GetLastFrame()
	if frame == nil {
		return "No frame captured yet — make sure the game is visible."
	}

	dir, err := a.app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		SetTitle("Select folder to save icons").
		PromptForSingleSelection()
	if err != nil || dir == "" {
		return "" // cancelled
	}

	layout := *a.currentLayout
	bounds := frame.Bounds()

	numCols := (bounds.Dx() - layout.ColPhase) / layout.ColPeriod
	numRows := (bounds.Dy() - layout.RowPhase) / layout.RowPeriod

	saved := 0
	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			x0 := bounds.Min.X + layout.ColPhase + col*layout.ColPeriod
			y0 := bounds.Min.Y + layout.RowPhase + row*layout.RowPeriod
			x1 := x0 + layout.ColPeriod
			y1 := y0 + layout.RowPeriod
			if x1 > bounds.Max.X || y1 > bounds.Max.Y {
				continue
			}

			slot := frame.SubImage(image.Rect(x0, y0, x1, y1))
			path := filepath.Join(dir, fmt.Sprintf("icon_%d_%d.png", col, row))
			f, err := os.Create(path)
			if err != nil {
				return fmt.Sprintf("Error saving %s: %v", path, err)
			}
			if err := png.Encode(f, slot); err != nil {
				f.Close()
				return fmt.Sprintf("Error encoding %s: %v", path, err)
			}
			f.Close()
			saved++
		}
	}

	return fmt.Sprintf("Saved %d icons to %s", saved, dir)
}
