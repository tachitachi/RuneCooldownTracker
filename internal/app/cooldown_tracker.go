package app

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// CreateCooldownTrackerConfigWindow opens the cooldown tracker config window.
// If already open it brings it to the front instead of creating a duplicate.
func (a *App) CreateCooldownTrackerConfigWindow(app *application.App) {
	if a.cooldownTrackerWindow != nil {
		a.cooldownTrackerWindow.Focus()
		return
	}

	a.cooldownTrackerWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "Cooldown Tracker Config",
		Width:          700,
		Height:         800,
		Hidden:         false,
		BackgroundType: application.BackgroundTypeSolid,
		URL:            "/cooldown-tracker.html",
	})

	a.cooldownTrackerWindow.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		fmt.Println("cooldown tracker window closing...")
		a.cooldownTrackerWindow = nil
	})

	// Hydrate the new window with current state when it first shows.
	a.cooldownTrackerWindow.OnWindowEvent(events.Common.WindowShow, func(_ *application.WindowEvent) {
		a.app.Event.Emit("debug:mode", map[string]any{"enabled": a.debugMode})
		a.app.Event.Emit("tracking:enabled", map[string]any{"enabled": a.trackingEnabled})
		a.app.Event.Emit("profile:changed", map[string]any{"active": a.activeProfile})
		overlays := a.GetAbilityOverlayConfigs()
		if len(overlays) > 0 {
			a.app.Event.Emit("overlay:configs", overlays)
		}
	})
}

// GetAbilityOverlayConfigs returns the overlay icon configs for the active profile.
func (a *App) GetAbilityOverlayConfigs() map[string]AbilityOverlayConfig {
	for _, p := range a.profiles {
		if p.Name == a.activeProfile {
			if p.AbilityOverlays == nil {
				return map[string]AbilityOverlayConfig{}
			}
			return p.AbilityOverlays
		}
	}
	return map[string]AbilityOverlayConfig{}
}

// SetAbilityOverlayConfig stores a single ability's overlay config in the
// active profile and broadcasts the updated map to all windows.
func (a *App) SetAbilityOverlayConfig(name string, cfg AbilityOverlayConfig) {
	for i, p := range a.profiles {
		if p.Name == a.activeProfile {
			if a.profiles[i].AbilityOverlays == nil {
				a.profiles[i].AbilityOverlays = make(map[string]AbilityOverlayConfig)
			}
			a.profiles[i].AbilityOverlays[name] = cfg
			a.saveConfig()
			a.app.Event.Emit("overlay:configs", a.profiles[i].AbilityOverlays)
			return
		}
	}
}

// RemoveAbilityOverlay deletes a tracker icon config from the active profile.
func (a *App) RemoveAbilityOverlay(name string) {
	for i, p := range a.profiles {
		if p.Name == a.activeProfile {
			if a.profiles[i].AbilityOverlays != nil {
				delete(a.profiles[i].AbilityOverlays, name)
				a.saveConfig()
				a.app.Event.Emit("overlay:configs", a.profiles[i].AbilityOverlays)
			}
			return
		}
	}
}

// GetDebugMode returns whether debug overlays (grid lines, state tints) are shown.
func (a *App) GetDebugMode() bool { return a.debugMode }

// SetDebugMode toggles debug overlays on/off, persists the setting, and
// broadcasts a debug:mode event to all windows.
func (a *App) SetDebugMode(enabled bool) {
	a.debugMode = enabled
	a.saveConfig()
	a.app.Event.Emit("debug:mode", map[string]any{"enabled": enabled})
}

// GetTrackingEnabled returns whether cooldown tracker overlays are visible.
func (a *App) GetTrackingEnabled() bool { return a.trackingEnabled }

// SetTrackingEnabled shows or hides all tracker icon overlays, persists the
// setting, and broadcasts a tracking:enabled event to all windows.
func (a *App) SetTrackingEnabled(enabled bool) {
	a.trackingEnabled = enabled
	a.saveConfig()
	a.app.Event.Emit("tracking:enabled", map[string]any{"enabled": enabled})
}

// StartIconPlacement makes the overlay interactive so the user can click to
// position a tracker icon for the named ability.
func (a *App) StartIconPlacement(abilityName string) {
	a.overlayWindow.SetIgnoreMouseEvents(false)
	a.app.Event.Emit("tracker:place:start", map[string]any{"name": abilityName})
}

// ConfirmIconPlacement stores the clicked logical-pixel position for the
// named ability's tracker icon, restores click-through, and persists.
func (a *App) ConfirmIconPlacement(name string, x, y float64) {
	// Preserve existing style settings; only update position.
	cfg := AbilityOverlayConfig{Size: 48, GlowDuration: 1.0, OnCooldown: "translucent"}
	for _, p := range a.profiles {
		if p.Name == a.activeProfile {
			if existing, ok := p.AbilityOverlays[name]; ok {
				cfg = existing
			}
			break
		}
	}
	cfg.X = x
	cfg.Y = y

	a.overlayWindow.SetIgnoreMouseEvents(true)
	a.app.Event.Emit("tracker:place:end", map[string]any{"name": name})
	a.SetAbilityOverlayConfig(name, cfg)
}

// CancelIconPlacement aborts icon placement mode and restores click-through.
func (a *App) CancelIconPlacement() {
	a.overlayWindow.SetIgnoreMouseEvents(true)
	a.app.Event.Emit("tracker:place:end", map[string]any{"name": ""})
}

// AdjustIconPlacement nudges the named ability's tracker icon position or size
// by the given logical-pixel deltas and immediately broadcasts the update.
func (a *App) AdjustIconPlacement(name string, dX, dY, dSize float64) {
	for i, p := range a.profiles {
		if p.Name == a.activeProfile {
			if p.AbilityOverlays == nil {
				return
			}
			cfg, ok := a.profiles[i].AbilityOverlays[name]
			if !ok {
				return
			}
			cfg.X += dX
			cfg.Y += dY
			cfg.Size += dSize
			if cfg.Size < 8 {
				cfg.Size = 8
			}
			a.profiles[i].AbilityOverlays[name] = cfg
			a.saveConfig()
			a.app.Event.Emit("overlay:configs", a.profiles[i].AbilityOverlays)
			return
		}
	}
}
