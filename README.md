# RuneCooldownTracker

A RuneScape ability cooldown overlay built with [Wails v3](https://wails.io/) (Go + React/TypeScript). Uses the Windows Graphics Capture API and Direct3D 11 to capture the game window and display a transparent overlay showing which abilities are ready, on cooldown, or unavailable due to insufficient resources.

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js / npm | 18+ | [nodejs.org](https://nodejs.org/) |
| wails3 CLI | latest | `go install github.com/wailsapp/wails/v3/cmd/wails3@latest` |
| go-task | v3 | `go install github.com/go-task/task/v3/cmd/task@latest` |

Windows 10 1903+ is required for the Windows Graphics Capture API.

## How It Works

The app has two windows:

- **Overlay window** — a transparent, click-through window layered over the game. Renders coloured highlights on each ability slot to indicate its state.
- **Config window** — a regular window for configuring the capture area, fine-tuning the grid, and controlling tracking.

### Detection Pipeline

1. **Grid detection** — when the capture area is set, the app analyses edge-gradient projections of the frame to automatically detect the ability bar's slot period and phase.
2. **Slot identification (Phase A)** — when tracking starts, each grid slot is compared against 19 embedded reference icons using Normalized Cross-Correlation (NCC) to identify which ability it contains.
3. **State detection (Phase B)** — each subsequent frame, identified slots are compared against game-pixel baselines captured at tracking start using Mean Absolute Error (MAE) with a donut mask. The donut mask excludes the centre circle (where the cooldown timer appears) from the MAE check; the centre is then separately sampled for brightness to distinguish cooldown (bright timer overlay) from no-resources (dim).

**Supported abilities** (19): Surge, Escape, Dive, Barricade, Reflect, Devotion, Resonance, Freedom, Debilitate, Immortality, Storm Shards, Shatter, Living Death, Death Skulls, Soul Sap, Touch of Death, Split Soul, Threads of Fate, Life Transfer

### Slot States

| State | Overlay colour | Description |
|-------|---------------|-------------|
| Ready | Green | Ability is available |
| Cooldown | Red | Ability is on cooldown (timer visible) |
| No resources | Orange | Ability is available but resources are insufficient |
| Unknown | Grey | Slot not identified or detection error |

## Usage

1. Launch the app. The transparent overlay and config window appear.
2. In the config window, click **Set Capture Area** and draw a rectangle over the RuneScape ability bar.
3. The grid is detected automatically. Use the **Adjust Grid** D-pad buttons if the lines need fine-tuning.
4. Optionally click **Export Icons** to save slot crops as PNG files (useful for adding new reference icons).
5. Click **Start Tracking** while your abilities are in their ready state — baselines are captured at this moment.
6. The overlay highlights each slot in real time as states change.
7. Click **Stop Tracking** to pause state detection.

> **Note:** Click Start Tracking while abilities are ready. The baseline comparison relies on the ready-state appearance being captured at tracking start.

## Package Structure

```
RuneCooldownTracker/
├── main.go                        # Wails v3 entry point
├── go.mod / go.sum
├── Taskfile.yml                   # Root task runner (delegates to build/ tasks)
│
├── build/                         # Wails build assets — commit to version control
│   ├── config.yml                 # App metadata and dev-mode config
│   ├── Taskfile.yml
│   └── windows/
│       ├── Taskfile.yml
│       └── icon.ico
│
├── internal/
│   ├── app/                       # Wails App service — all exported Go methods callable from JS
│   │   ├── app.go                 # App struct, window setup, lifecycle
│   │   ├── overlay.go             # Transparent overlay window, D3D11 capture loop, DPI tracking
│   │   ├── snipping.go            # Capture-area selection (snipping tool interaction)
│   │   ├── grid.go                # Grid layout: auto-emit, AdjustGridLayout, ExportIcons
│   │   └── tracking.go            # StartTracking / StopTracking, slot-state event emission
│   │
│   ├── capture/                   # Windows Graphics Capture + D3D11 pipeline
│   │   ├── capture.go             # CaptureHandler: frame loop, GPU→CPU staging texture
│   │   ├── dx11_extra.go          # D3D11 types/constants missing from winapi module
│   │   └── Direct3D11CaptureFramePool.go
│   │
│   └── detection/                 # Ability detection algorithms
│       ├── icons.go               # Embeds 19 reference PNGs; LoadReferenceIcons()
│       ├── identify.go            # NCC-based slot identification (Phase A)
│       ├── state.go               # MAE+donut-mask state detection (Phase B)
│       └── detection.go           # AbilityDetector: grid detection, ProcessFrame, StartTracking
│
└── frontend/                      # React + TypeScript + Vite
    ├── index.html
    ├── package.json
    ├── vite.config.ts
    ├── src/
    │   ├── main.tsx               # Mounts overlay (App) and config (ConfigApp) windows
    │   ├── App.tsx                # Transparent overlay: renders slot-state highlights
    │   └── ConfigApp.tsx          # Config window: capture area, grid controls, tracking
    ├── bindings/                  # ⚠ Generated — do not commit
    │   └── RuneCooldownTracker/
    │       └── app.ts             # Type-safe bindings to Go App methods
    └── dist/                      # ⚠ Generated — do not commit
```

## Development

Run in live-reload mode (frontend hot-reloads, Go recompiles on save):

```bash
wails3 dev
```

The dev server starts a Vite HMR server on port 9245.

Regenerate TypeScript bindings after adding or renaming exported Go methods:

```bash
wails3 generate bindings
```

## Building

Build a production binary:

```bash
task build
```

The binary is written to `bin/RuneCooldownTracker.exe`.

To build in dev (non-minified) mode:

```bash
task build DEV=true
```

## Gitignore Guidance

Add these to `.gitignore`:

```
# Generated build outputs
bin/
frontend/dist/
frontend/bindings/
*.syso

# Dependencies
frontend/node_modules/
```

The `build/` directory (config, task files, icons) **should be committed** — it contains scaffolded configuration that is part of the project, not generated output.
