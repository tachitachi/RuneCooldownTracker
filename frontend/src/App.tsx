import {useState, useEffect, useCallback, useMemo, useRef} from 'react'
import {Events} from '@wailsio/runtime'
import {ConfirmSnip, CancelSnipping, ConfirmIconPlacement, CancelIconPlacement, GetAbilityOverlayConfigs, GetAbilityIcon, GetCombatTimeout} from '../bindings/github.com/tachitachi/RuneCooldownTracker/internal/app/app'
import './App.css'

interface GridLines {
    xLines: number[]
    yLines: number[]
}

interface SlotState {
    col: number
    row: number
    name: string
    x: number
    y: number
    w: number
    h: number
    state: number  // 0=ready, 1=cooldown, 2=no_resources, 3=unknown
}

interface AbilityOverlayCfg {
    x: number
    y: number
    size: number
    glowEnabled: boolean
    glowDuration: number
    onCooldown: string
}

// STATE_COLORS maps AbilityState int values to overlay tint colours.
const STATE_COLORS: Record<number, string> = {
    0: '#00ff0033', // ready      — green tint
    1: '#ff000066', // cooldown   — red tint
    2: '#ffa50066', // no resources — orange tint
    3: '#88888833', // unknown    — grey tint
}

// Padding added around the two clicked points (logical px).
// One slot is ~27–54 logical px depending on DPI; 60 px safely covers the
// full outermost slot on each side at any common DPI scale.
const SNIP_MARGIN = 30

export default function App() {
    const [snipping, setSnipping] = useState(false)
    const [firstClick, setFirstClick] = useState<{x: number; y: number} | null>(null)
    const [gridLines, setGridLines] = useState<GridLines | null>(null)
    const [slotStates, setSlotStates] = useState<Map<string, SlotState>>(new Map())

    // Cooldown tracker overlay state
    const [debugMode, setDebugMode] = useState(true)
    const [trackingEnabled, setTrackingEnabled] = useState(false)
    const [overlayConfigs, setOverlayConfigs] = useState<Record<string, AbilityOverlayCfg>>({})
    const [placingAbility, setPlacingAbility] = useState<string | null>(null)
    const [abilityIcons, setAbilityIcons] = useState<Record<string, string>>({})
    const [mousePos, setMousePos] = useState<{x: number; y: number} | null>(null)

    // Combat timeout: hide overlay when no ability goes on cooldown for N seconds.
    // -1 = indefinite (never hide). Default 10s until backend hydrates value.
    const [combatTimeout, setCombatTimeout] = useState(10)
    const combatTimeoutRef = useRef(10)
    const [inCombat, setInCombat] = useState(false)
    const combatTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

    // Listen for snipping:start from Go
    useEffect(() => {
        const off = Events.On('snipping:start', () => {
            setSnipping(true)
            setFirstClick(null)
        })
        return () => off()
    }, [])

    // Listen for grid detection results from Go
    useEffect(() => {
        const off = Events.On('grid:detected', (ev: any) => {
            setGridLines(ev.data)
        })
        return () => off()
    }, [])

    // Listen for per-slot state updates from Go
    useEffect(() => {
        const off = Events.On('tracking:states', (ev: any) => {
            const updates: SlotState[] = ev.data
            setSlotStates(prev => {
                const next = new Map(prev)
                for (const s of updates) {
                    next.set(`${s.col}_${s.row}`, s)
                }
                return next
            })
            // If any ability just went on cooldown, reset the combat visibility timer
            const hasCooldown = updates.some((s: SlotState) => s.state === 1)
            if (hasCooldown && combatTimeoutRef.current >= 0) {
                if (combatTimerRef.current) clearTimeout(combatTimerRef.current)
                setInCombat(true)
                combatTimerRef.current = setTimeout(
                    () => setInCombat(false),
                    combatTimeoutRef.current * 1000
                )
            }
        })
        return () => off()
    }, [])

    // Listen for debug mode, tracking enabled, overlay configs, placement events
    useEffect(() => {
        const offs = [
            Events.On('debug:mode', (ev: any) => setDebugMode(ev.data?.enabled ?? true)),
            Events.On('tracking:enabled', (ev: any) => setTrackingEnabled(ev.data?.enabled ?? false)),
            Events.On('overlay:configs', (ev: any) => {
                setOverlayConfigs((ev.data as Record<string, AbilityOverlayCfg>) ?? {})
            }),
            Events.On('tracker:place:start', (ev: any) => {
                setPlacingAbility(ev.data?.name ?? null)
                setMousePos(null)
            }),
            Events.On('tracker:place:end', () => {
                setPlacingAbility(null)
                setMousePos(null)
            }),
            Events.On('profile:changed', async () => {
                const cfgs = await GetAbilityOverlayConfigs()
                setOverlayConfigs((cfgs as Record<string, AbilityOverlayCfg>) ?? {})
                const t = await GetCombatTimeout()
                const timeout = t ?? 10
                setCombatTimeout(timeout)
                combatTimeoutRef.current = timeout
            }),
            Events.On('combat:timeout', (ev: any) => {
                const t = ev.data?.seconds ?? 10
                setCombatTimeout(t)
                combatTimeoutRef.current = t
            }),
        ]
        return () => offs.forEach(off => off())
    }, [])

    // Preload icons when overlayConfigs changes
    useEffect(() => {
        const needed = Object.keys(overlayConfigs).filter(n => !abilityIcons[n])
        if (needed.length === 0) return
        Promise.all(needed.map(async name => {
            const b64 = await GetAbilityIcon(name)
            return [name, b64 ?? ''] as [string, string]
        })).then(entries => {
            setAbilityIcons(prev => {
                const next = {...prev}
                for (const [name, b64] of entries) {
                    if (b64) next[name] = b64
                }
                return next
            })
        })
    }, [overlayConfigs])

    // Build state lookup by ability name for tracker icons
    const stateByName = useMemo(() =>
        new Map(Array.from(slotStates.values()).filter(s => s.name).map(s => [s.name, s])),
    [slotStates])

    // Escape to cancel snipping or placement
    useEffect(() => {
        if (!snipping && !placingAbility) return
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                if (snipping) {
                    CancelSnipping()
                    setSnipping(false)
                    setFirstClick(null)
                } else if (placingAbility) {
                    CancelIconPlacement()
                    setPlacingAbility(null)
                    setMousePos(null)
                }
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [snipping, placingAbility])

    const onMouseDown = useCallback((e: React.MouseEvent) => {
        if (!firstClick) {
            // First click: record top-left ability position
            setFirstClick({x: e.clientX, y: e.clientY})
        } else {
            // Second click: expand to cover both slots fully, then confirm
            const x1 = Math.min(firstClick.x, e.clientX)
            const y1 = Math.min(firstClick.y, e.clientY)
            const x2 = Math.max(firstClick.x, e.clientX)
            const y2 = Math.max(firstClick.y, e.clientY)
            ConfirmSnip(
                x1 - SNIP_MARGIN,
                y1 - SNIP_MARGIN,
                (x2 - x1) + 2 * SNIP_MARGIN,
                (y2 - y1) + 2 * SNIP_MARGIN,
                SNIP_MARGIN, // first click x-offset from crop left (always SNIP_MARGIN)
                SNIP_MARGIN, // first click y-offset from crop top  (always SNIP_MARGIN)
            )
            setSnipping(false)
            setFirstClick(null)
        }
    }, [firstClick])

    const onPlacementClick = useCallback((e: React.MouseEvent) => {
        if (!placingAbility) return
        // Center the icon on the cursor position
        const size = overlayConfigs[placingAbility]?.size ?? 48
        ConfirmIconPlacement(placingAbility, e.clientX - size / 2, e.clientY - size / 2)
        setPlacingAbility(null)
        setMousePos(null)
    }, [placingAbility, overlayConfigs])

    if (snipping) {
        const instruction = firstClick
            ? 'Click on the bottom-right ability — Escape to cancel'
            : 'Click on the top-left ability — Escape to cancel'
        return (
            <div
                style={{
                    position: 'fixed',
                    inset: 0,
                    background: 'rgba(0, 0, 0, 0.45)',
                    cursor: 'crosshair',
                    userSelect: 'none',
                }}
                onMouseDown={onMouseDown}
            >
                {firstClick && (
                    <div
                        style={{
                            position: 'fixed',
                            left: firstClick.x - 6,
                            top: firstClick.y - 6,
                            width: 12,
                            height: 12,
                            borderRadius: '50%',
                            background: '#00aaff',
                            pointerEvents: 'none',
                        }}
                    />
                )}
                <div
                    style={{
                        position: 'absolute',
                        bottom: 16,
                        left: '50%',
                        transform: 'translateX(-50%)',
                        color: 'white',
                        fontSize: 13,
                        background: 'rgba(0,0,0,0.7)',
                        padding: '6px 14px',
                        borderRadius: 6,
                        pointerEvents: 'none',
                        whiteSpace: 'nowrap',
                    }}
                >
                    {instruction}
                </div>
            </div>
        )
    }

    // The actual detection area doesn't exactly align with where we are drawing the grid
    let offsetCorrections = {
        x: 5,
        y: -1
    }

    // Normal overlay content (transparent, non-interactive)
    return (
        <div id="App">
            {/* Debug overlays: state tints and grid lines — only shown in debug mode */}
            {debugMode && Array.from(slotStates.values()).map(s => (
                <div key={`ss_${s.col}_${s.row}`} style={{
                    position: 'fixed',
                    left: s.x + offsetCorrections.x,
                    top: s.y + offsetCorrections.y,
                    width: s.w,
                    height: s.h,
                    background: STATE_COLORS[s.state] ?? STATE_COLORS[3],
                    pointerEvents: 'none',
                }}/>
            ))}
            {debugMode && gridLines?.xLines.map((x, i) => (
                <div key={`gx${i}`} style={{
                    position: 'fixed',
                    left: x + offsetCorrections.x,
                    top: gridLines?.yLines ? Math.min(...gridLines?.yLines) + offsetCorrections.y: 0,
                    width: 1,
                    height: gridLines?.yLines ? Math.max(...gridLines?.yLines) - Math.min(...gridLines?.yLines): '100vh',
                    background: '#ffff00',
                    opacity: 0.8,
                    pointerEvents: 'none',
                }}/>
            ))}
            {debugMode && gridLines?.yLines.map((y, i) => (
                <div key={`gy${i}`} style={{
                    position: 'fixed',
                    left: gridLines?.xLines ? Math.min(...gridLines?.xLines) + offsetCorrections.x : 0,
                    top: y + offsetCorrections.y,
                    width: gridLines?.xLines ? Math.max(...gridLines?.xLines) - Math.min(...gridLines?.xLines): '100vw',
                    height: 1,
                    background: '#ffff00',
                    opacity: 0.8,
                    pointerEvents: 'none',
                }}/>
            ))}

            {/* Icon placement mode: transparent click-capture + preview icon following cursor */}
            {placingAbility && (() => {
                const size = overlayConfigs[placingAbility]?.size ?? 48
                return (
                    <div
                        style={{position: 'fixed', inset: 0, cursor: 'crosshair', userSelect: 'none', zIndex: 9999}}
                        onMouseMove={e => setMousePos({x: e.clientX, y: e.clientY})}
                        onMouseDown={onPlacementClick}
                    >
                        {/* Preview icon centered on cursor */}
                        {mousePos && (
                            <div style={{
                                position: 'fixed',
                                left: mousePos.x - size / 2,
                                top: mousePos.y - size / 2,
                                width: size,
                                height: size,
                                pointerEvents: 'none',
                                opacity: 0.75,
                                border: '2px dashed white',
                                borderRadius: 4,
                                overflow: 'hidden',
                                boxSizing: 'border-box',
                            }}>
                                {abilityIcons[placingAbility] && (
                                    <img
                                        src={`data:image/png;base64,${abilityIcons[placingAbility]}`}
                                        width={size}
                                        height={size}
                                        style={{imageRendering: 'pixelated', display: 'block'}}
                                    />
                                )}
                            </div>
                        )}
                        {/* Instruction label */}
                        <div style={{
                            position: 'absolute', bottom: 16, left: '50%', transform: 'translateX(-50%)',
                            color: 'white', fontSize: 13, background: 'rgba(0,0,0,0.7)',
                            padding: '6px 14px', borderRadius: 6, pointerEvents: 'none', whiteSpace: 'nowrap',
                        }}>
                            Click to place {placingAbility.replace(/_/g, ' ')} — Esc to cancel
                        </div>
                    </div>
                )
            })()}

            {/* Tracker icon overlays — shown when tracking is enabled and in combat (or timeout is indefinite) */}
            {(trackingEnabled && (combatTimeout < 0 || inCombat)) && Object.entries(overlayConfigs).map(([name, cfg]) => {
                const s = stateByName.get(name)
                const state = s?.state ?? 3

                if (cfg.onCooldown === 'hidden' && state === 1) return null

                const opacity = (cfg.onCooldown === 'translucent' && state === 1) ? 0.25 : 1.0
                const glow = cfg.glowEnabled && state === 0

                return (
                    <div key={`tracker_${name}`} style={{
                        position: 'fixed',
                        left: cfg.x,
                        top: cfg.y,
                        width: cfg.size,
                        height: cfg.size,
                        opacity,
                        borderRadius: 4,
                        border: '2px solid transparent',
                        animation: glow ? `glow-pulse ${cfg.glowDuration}s ease-in-out infinite` : 'none',
                        pointerEvents: 'none',
                        overflow: 'hidden',
                        boxSizing: 'border-box',
                    }}>
                        {abilityIcons[name] && (
                            <img
                                src={`data:image/png;base64,${abilityIcons[name]}`}
                                width={cfg.size}
                                height={cfg.size}
                                style={{imageRendering: 'pixelated', display: 'block'}}
                            />
                        )}
                    </div>
                )
            })}
        </div>
    )
}
