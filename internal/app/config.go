package app

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/tachitachi/RuneCooldownTracker/internal/capture"
	"github.com/tachitachi/RuneCooldownTracker/internal/detection"
)

// AbilityOverlayConfig stores the per-ability tracker-icon placement and style.
type AbilityOverlayConfig struct {
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Size         float64 `json:"size"`         // logical px, default 48
	GlowEnabled  bool    `json:"glowEnabled"`
	GlowDuration float64 `json:"glowDuration"` // animation seconds, default 1.0
	OnCooldown   string  `json:"onCooldown"`   // "translucent" | "hidden"
}

// ProfileConfig stores the named slot-to-ability assignments for a single profile.
type ProfileConfig struct {
	Name            string                          `json:"name"`
	SlotRefs        map[string]string               `json:"slotRefs,omitempty"`       // "col:row" → abilityName
	AbilityOverlays map[string]AbilityOverlayConfig `json:"abilityOverlays,omitempty"` // abilityName → overlay config
	CombatTimeout   *float64                        `json:"combatTimeout,omitempty"`   // seconds; nil → 10s default; -1 → indefinite
}

type appConfig struct {
	CropRegion             *capture.CropRegion                  `json:"cropRegion,omitempty"`
	GridLayout             *detection.SlotLayout                `json:"gridLayout,omitempty"`
	ClickInCrop            *image.Point                         `json:"clickInCrop,omitempty"`
	DetectionParams        *detection.DetectionParams           `json:"detectionParams,omitempty"`
	AbilityDetectionParams map[string]detection.DetectionParams `json:"abilityDetectionParams,omitempty"`
	Profiles               []ProfileConfig                      `json:"profiles,omitempty"`
	ActiveProfile          string                               `json:"activeProfile,omitempty"`
	// DebugMode uses *bool so that absence in JSON defaults to true (preserves
	// existing behaviour for users who haven't opened the new config window yet).
	DebugMode       *bool `json:"debugMode,omitempty"`
	TrackingEnabled bool  `json:"trackingEnabled"`
}

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "RuneCooldownTracker", "config.json")
}

func (a *App) saveConfig() {
	cfg := appConfig{}
	if a.handler != nil {
		cfg.CropRegion = a.handler.GetCropRegion()
	}
	if a.currentLayout != nil {
		layout := *a.currentLayout
		cfg.GridLayout = &layout
	}
	if a.clickInCrop != (image.Point{}) {
		pt := a.clickInCrop
		cfg.ClickInCrop = &pt
	}
	if a.detector != nil {
		params := a.detector.GetDetectionParams()
		cfg.DetectionParams = &params
		ap := a.detector.GetAllAbilityParams()
		if len(ap) > 0 {
			cfg.AbilityDetectionParams = ap
		}
	}
	if len(a.profiles) > 0 {
		cfg.Profiles = a.profiles
	}
	cfg.ActiveProfile = a.activeProfile
	cfg.DebugMode = &a.debugMode
	cfg.TrackingEnabled = a.trackingEnabled

	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Printf("[config] failed to create config dir: %v\n", err)
		return
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Printf("[config] failed to marshal config: %v\n", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Printf("[config] failed to write config: %v\n", err)
		return
	}
	fmt.Printf("[config] saved to %s\n", path)
}

func (a *App) loadConfig() {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("[config] failed to read config: %v\n", err)
		}
		return
	}
	var cfg appConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Printf("[config] failed to parse config: %v\n", err)
		return
	}

	if cfg.CropRegion != nil && a.handler != nil {
		a.handler.SetCropRegion(cfg.CropRegion)
		fmt.Printf("[config] restored crop region: %+v\n", *cfg.CropRegion)
	}
	if cfg.ClickInCrop != nil {
		a.clickInCrop = *cfg.ClickInCrop
		if a.detector != nil {
			a.detector.ClickHint = &a.clickInCrop
		}
	}
	if cfg.GridLayout != nil && a.detector != nil {
		var bounds image.Rectangle
		if cfg.CropRegion != nil {
			bounds = image.Rect(0, 0, cfg.CropRegion.W, cfg.CropRegion.H)
		}
		a.currentLayout = cfg.GridLayout
		a.detector.SetLayoutAndBounds(*cfg.GridLayout, bounds)
		fmt.Printf("[config] restored grid layout: %+v\n", *cfg.GridLayout)
		a.emitGridLines(*cfg.GridLayout)
	}
	if cfg.DetectionParams != nil && a.detector != nil {
		a.detector.SetDetectionParams(*cfg.DetectionParams)
		fmt.Printf("[config] restored detection params: %+v\n", *cfg.DetectionParams)
	}
	if cfg.AbilityDetectionParams != nil && a.detector != nil {
		a.detector.SetAbilityParamsMap(cfg.AbilityDetectionParams)
		fmt.Printf("[config] restored per-ability detection params for %d abilities\n", len(cfg.AbilityDetectionParams))
	}
	if len(cfg.Profiles) > 0 {
		a.profiles = cfg.Profiles
		fmt.Printf("[config] restored %d profiles\n", len(cfg.Profiles))
	}
	if cfg.DebugMode != nil {
		a.debugMode = *cfg.DebugMode
	} else {
		a.debugMode = true // first-run default: show debug overlays
	}
	a.trackingEnabled = cfg.TrackingEnabled
	if cfg.ActiveProfile != "" {
		a.activeProfile = cfg.ActiveProfile
		// Restore slot refs from the active profile.
		for _, p := range a.profiles {
			if p.Name == cfg.ActiveProfile && a.detector != nil {
				refs := make(map[detection.SlotKey]string, len(p.SlotRefs))
				for encoded, abilityName := range p.SlotRefs {
					if key, ok := decodeSlotKey(encoded); ok {
						refs[key] = abilityName
					}
				}
				a.detector.ApplySlotRefs(refs, a.refImages)
				fmt.Printf("[config] restored slot refs from active profile %q (%d slots)\n",
					cfg.ActiveProfile, len(refs))
				break
			}
		}
	}
}
