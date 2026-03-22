# RuneCooldownTracker

A RuneScape ability cooldown overlay built with [Wails v3](https://wails.io/) (Go + React/TypeScript). Uses the Windows Graphics Capture API and Direct3D 11 to capture the game window and track ability cooldowns as a transparent overlay.

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js / npm | 18+ | [nodejs.org](https://nodejs.org/) |
| wails3 CLI | latest | `go install github.com/wailsapp/wails/v3/cmd/wails3@latest` |
| go-task | v3 | `go install github.com/go-task/task/v3/cmd/task@latest` |

Windows 10 1903+ is required for the Windows Graphics Capture API.

## Package Structure

```
RuneCooldownTracker/
├── main.go                        # Wails v3 entry point (application setup)
├── app.go                         # App service: window following, capture lifecycle
├── go.mod
├── go.sum
├── Taskfile.yml                   # Root task runner (delegates to build/ tasks)
│
├── build/                         # Wails build assets — commit to version control
│   ├── config.yml                 # App metadata and dev-mode config
│   ├── Taskfile.yml               # Common tasks: bindings, frontend build, icons
│   ├── windows/                   # Windows-specific tasks and resources
│   │   ├── Taskfile.yml
│   │   └── icon.ico
│   └── darwin/                    # macOS resources (if applicable)
│
├── frontend/                      # React + TypeScript + Vite
│   ├── index.html
│   ├── package.json
│   ├── vite.config.ts
│   ├── src/
│   │   ├── main.tsx
│   │   └── App.tsx
│   ├── bindings/                  # ⚠ Generated — do not commit
│   │   └── RuneCooldownTracker/
│   │       └── app.ts             # Type-safe bindings to Go service methods
│   └── dist/                      # ⚠ Generated — do not commit
│
└── internal/
    └── capture/                   # Windows Graphics Capture + D3D11 pipeline
        ├── capture.go             # Frame capture handler, GPU→CPU staging texture
        ├── dx11_extra.go          # D3D11 types/constants missing from winapi module
        └── Direct3D11CaptureFramePool.go
```

## Development

Run the app in live-reload mode (frontend hot-reloads, Go recompiles on save):

```bash
wails3 dev
```

The dev server starts a Vite HMR server on port 9245. The Wails window loads from it automatically.

Bindings are regenerated automatically during dev/build. If you add or rename exported methods on `App` in `app.go`, the TypeScript bindings in `frontend/bindings/` will update on the next build.

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
