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

type appConfig struct {
	CropRegion  *capture.CropRegion   `json:"cropRegion,omitempty"`
	GridLayout  *detection.SlotLayout `json:"gridLayout,omitempty"`
	ClickInCrop *image.Point          `json:"clickInCrop,omitempty"`
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
}
