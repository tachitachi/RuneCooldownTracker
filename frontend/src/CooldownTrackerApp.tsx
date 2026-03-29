import {useState, useEffect, useCallback} from 'react'
import {Events} from '@wailsio/runtime'
import {
    GetTrackedAbilityNames, GetAbilityIcon,
    GetAbilityOverlayConfigs, SetAbilityOverlayConfig, RemoveAbilityOverlay, AdjustIconPlacement,
    StartIconPlacement, CancelIconPlacement,
    GetDebugMode, SetDebugMode,
    GetTrackingEnabled, SetTrackingEnabled,
    GetActiveProfile, GetProfiles, LoadProfile, CreateProfile, DeleteProfile,
} from '../bindings/github.com/tachitachi/RuneCooldownTracker/internal/app/app'
import {AbilityOverlayConfig} from '../bindings/github.com/tachitachi/RuneCooldownTracker/internal/app/models'

interface AbilityOverlayCfg {
    x: number
    y: number
    size: number
    glowEnabled: boolean
    glowDuration: number
    onCooldown: string
}

function DPad({label, onLeft, onRight, onUp, onDown, leftTitle, rightTitle, upTitle, downTitle}: {
    label: string
    onLeft: () => void; onRight: () => void; onUp: () => void; onDown: () => void
    leftTitle: string; rightTitle: string; upTitle: string; downTitle: string
}) {
    const btn: React.CSSProperties = {
        width: 18, height: 18, fontSize: 12, cursor: 'pointer',
        background: '#333', color: 'white', border: '1px solid #555',
        borderRadius: 4, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 0,
    }
    return (
        <div style={{display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6, margin: 'auto'}}>
            <div style={{fontSize: 12, color: '#aaa', fontWeight: 600, letterSpacing: 1, textTransform: 'uppercase'}}>{label}</div>
            <div style={{display: 'grid', gridTemplateColumns: '18px 18px 18px', gridTemplateRows: '18px 18px 18px', gap: 4}}>
                <div/><button style={btn} title={upTitle} onClick={onUp}>↑</button><div/>
                <button style={btn} title={leftTitle} onClick={onLeft}>←</button>
                <div style={{width: 18, height: 18, background: '#222', borderRadius: 4}}/>
                <button style={btn} title={rightTitle} onClick={onRight}>→</button>
                <div/><button style={btn} title={downTitle} onClick={onDown}>↓</button><div/>
            </div>
        </div>
    )
}

function hr() {
    return <hr style={{borderColor: '#333', margin: '20px 0'}}/>
}

export default function CooldownTrackerApp() {
    const [abilities, setAbilities] = useState<string[]>([])
    const [overlayConfigs, setOverlayConfigs] = useState<Record<string, AbilityOverlayCfg>>({})
    const [abilityIcons, setAbilityIcons] = useState<Record<string, string>>({})
    const [debugMode, setDebugModeState] = useState(false)
    const [trackingEnabled, setTrackingEnabledState] = useState(false)
    const [activeProfile, setActiveProfile] = useState('')
    const [placingAbility, setPlacingAbility] = useState<string | null>(null)
    const [expandedAbility, setExpandedAbility] = useState<string | null>(null)

    // Profiles
    const [profiles, setProfiles] = useState<string[]>([])
    const [profileDropdown, setProfileDropdown] = useState('')
    const [creatingProfile, setCreatingProfile] = useState(false)
    const [newProfileName, setNewProfileName] = useState('')
    const [confirmDelete, setConfirmDelete] = useState(false)
    const [profileStatus, setProfileStatus] = useState<string | null>(null)

    // ── Load icons for abilities ──────────────────────────────────────────────

    const loadIcons = useCallback(async (names: string[]) => {
        const needed = names.filter(n => n && !abilityIcons[n])
        if (needed.length === 0) return
        const entries = await Promise.all(needed.map(async name => {
            const b64 = await GetAbilityIcon(name)
            return [name, b64 ?? ''] as [string, string]
        }))
        setAbilityIcons(prev => {
            const next = {...prev}
            for (const [name, b64] of entries) {
                if (b64) next[name] = b64
            }
            return next
        })
    }, [abilityIcons])

    const refreshAbilities = useCallback(async () => {
        const names = await GetTrackedAbilityNames()
        const list = names ?? []
        setAbilities(list)
        await loadIcons(list)
    }, [loadIcons])

    const refreshOverlayConfigs = useCallback(async () => {
        const cfgs = await GetAbilityOverlayConfigs()
        setOverlayConfigs((cfgs as Record<string, AbilityOverlayCfg>) ?? {})
    }, [])

    // ── Startup ───────────────────────────────────────────────────────────────

    useEffect(() => {
        GetDebugMode().then(v => setDebugModeState(v ?? false))
        GetTrackingEnabled().then(v => setTrackingEnabledState(v ?? false))
        GetActiveProfile().then(ap => { if (ap != null) setActiveProfile(ap) })
        GetProfiles().then(ps => { if (ps) setProfiles(ps) })
        refreshAbilities()
        refreshOverlayConfigs()
    }, [])

    // When overlayConfigs changes, preload any missing icons
    useEffect(() => {
        const names = Object.keys(overlayConfigs)
        if (names.length > 0) loadIcons(names)
    }, [overlayConfigs])

    // ── Event listeners ───────────────────────────────────────────────────────

    useEffect(() => {
        const offs = [
            Events.On('overlay:configs', (ev: any) => {
                setOverlayConfigs((ev.data as Record<string, AbilityOverlayCfg>) ?? {})
            }),
            Events.On('debug:mode', (ev: any) => setDebugModeState(ev.data?.enabled ?? false)),
            Events.On('tracking:enabled', (ev: any) => setTrackingEnabledState(ev.data?.enabled ?? false)),
            Events.On('tracker:place:end', () => setPlacingAbility(null)),
            Events.On('profile:changed', async (ev: any) => {
                const ap = ev.data?.active ?? ''
                setActiveProfile(ap)
                setProfileDropdown(ap)
                const ps = await GetProfiles()
                if (ps) setProfiles(ps)
                await refreshAbilities()
                await refreshOverlayConfigs()
            }),
        ]
        return () => offs.forEach(off => off())
    }, [])

    // Escape to cancel placement
    useEffect(() => {
        if (!placingAbility) return
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                CancelIconPlacement()
                setPlacingAbility(null)
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [placingAbility])

    // ── Handlers ──────────────────────────────────────────────────────────────

    function getOrDefaultCfg(name: string): AbilityOverlayCfg {
        return overlayConfigs[name] ?? {x: 0, y: 0, size: 48, glowEnabled: false, glowDuration: 1.0, onCooldown: 'translucent'}
    }

    async function handlePlace(name: string) {
        setPlacingAbility(name)
        await StartIconPlacement(name)
    }

    async function handleToggleDebug() {
        await SetDebugMode(!debugMode)
    }

    async function handleToggleTracking() {
        await SetTrackingEnabled(!trackingEnabled)
    }

    async function handleCfgChange(name: string, patch: Partial<AbilityOverlayCfg>) {
        const cfg = {...getOrDefaultCfg(name), ...patch}
        await SetAbilityOverlayConfig(name, new AbilityOverlayConfig({
            x: cfg.x, y: cfg.y, size: cfg.size,
            glowEnabled: cfg.glowEnabled, glowDuration: cfg.glowDuration,
            onCooldown: cfg.onCooldown,
        }))
    }

    async function handleRemove(name: string) {
        await RemoveAbilityOverlay(name)
        if (expandedAbility === name) setExpandedAbility(null)
    }

    // Profile handlers
    async function handleLoadProfile() {
        if (!profileDropdown) return
        const err = await LoadProfile(profileDropdown)
        if (err) {
            setProfileStatus(err)
        } else {
            setActiveProfile(profileDropdown)
            setProfileStatus(null)
            await refreshAbilities()
            await refreshOverlayConfigs()
        }
    }

    async function handleCreateProfile() {
        const err = await CreateProfile(newProfileName)
        if (err) {
            setProfileStatus(err)
        } else {
            setProfileStatus(null)
            setCreatingProfile(false)
            setNewProfileName('')
            const ps = await GetProfiles()
            if (ps) setProfiles(ps)
            setActiveProfile(newProfileName)
            setProfileDropdown(newProfileName)
        }
    }

    async function handleDeleteProfile() {
        if (!profileDropdown) return
        await DeleteProfile(profileDropdown)
        setConfirmDelete(false)
        const ps = await GetProfiles()
        if (ps) {
            setProfiles(ps)
            const newActive = await GetActiveProfile()
            setActiveProfile(newActive ?? '')
            setProfileDropdown(ps[0] ?? '')
        }
        setProfileStatus(null)
    }

    // ── Styles ────────────────────────────────────────────────────────────────

    const inputStyle: React.CSSProperties = {
        padding: '4px 8px', background: '#2a2a2a', color: 'white',
        border: '1px solid #555', borderRadius: 4, fontSize: 13,
    }
    const btnStyle: React.CSSProperties = {padding: '6px 14px', cursor: 'pointer'}
    const dangerBtnStyle: React.CSSProperties = {...btnStyle, background: '#7b1a1a', color: 'white', border: '1px solid #a33'}
    const toggleBtn = (active: boolean, activeColor: string): React.CSSProperties => ({
        ...btnStyle,
        background: active ? activeColor : '#333',
        color: 'white',
        border: `1px solid ${active ? activeColor : '#555'}`,
    })

    // ── Render ────────────────────────────────────────────────────────────────

    return (
        <div style={{background: '#1e1e1e', minHeight: '100vh', color: 'white', padding: 20, boxSizing: 'border-box', fontSize: 14}}>
            <h2 style={{marginTop: 0, marginBottom: 16}}>Cooldown Tracker Config</h2>

            {/* ── Global Toggles ── */}
            <div style={{display: 'flex', gap: 10, marginBottom: 16, flexWrap: 'wrap'}}>
                <button style={toggleBtn(trackingEnabled, '#1a6a1a')} onClick={handleToggleTracking}>
                    {trackingEnabled ? 'Overlays: Enabled' : 'Overlays: Disabled'}
                </button>
                <button style={toggleBtn(debugMode, '#6a4a1a')} onClick={handleToggleDebug}>
                    {debugMode ? 'Debug Mode: On' : 'Debug Mode: Off'}
                </button>
            </div>

            {/* ── Placement Banner ── */}
            {placingAbility && (
                <div style={{
                    background: '#1a3a5a', border: '1px solid #3a6a9a', borderRadius: 6,
                    padding: '10px 14px', marginBottom: 16, fontSize: 13,
                }}>
                    Click on the game screen to place the <strong>{placingAbility.replace(/_/g, ' ')}</strong> icon.
                    Press <kbd style={{background: '#333', padding: '1px 5px', borderRadius: 3}}>Esc</kbd> to cancel.
                </div>
            )}

            {hr()}

            {/* ── Profiles ── */}
            <h3 style={{marginTop: 0, marginBottom: 10}}>Profile</h3>
            <div style={{display: 'flex', flexWrap: 'wrap', gap: 8, alignItems: 'center'}}>
                <select
                    value={profileDropdown}
                    onChange={e => setProfileDropdown(e.target.value)}
                    style={{...inputStyle, minWidth: 140}}
                >
                    {profiles.length === 0 && <option value="">No profiles</option>}
                    {profiles.map(p => (
                        <option key={p} value={p}>{p}{p === activeProfile ? ' ✓' : ''}</option>
                    ))}
                </select>
                <button style={btnStyle} onClick={handleLoadProfile} disabled={!profileDropdown || profiles.length === 0}>
                    Load
                </button>
                {!creatingProfile && (
                    <button style={btnStyle} onClick={() => { setCreatingProfile(true); setNewProfileName('') }}>
                        Create
                    </button>
                )}
                {creatingProfile && (
                    <>
                        <input
                            autoFocus
                            value={newProfileName}
                            onChange={e => setNewProfileName(e.target.value)}
                            onKeyDown={e => { if (e.key === 'Enter') handleCreateProfile(); if (e.key === 'Escape') setCreatingProfile(false) }}
                            placeholder="Profile name"
                            style={{...inputStyle, width: 130}}
                        />
                        <button style={btnStyle} onClick={handleCreateProfile}>Save</button>
                        <button style={btnStyle} onClick={() => setCreatingProfile(false)}>Cancel</button>
                    </>
                )}
                {!confirmDelete && profiles.length > 0 && (
                    <button style={dangerBtnStyle} onClick={() => setConfirmDelete(true)} disabled={!profileDropdown}>
                        Delete
                    </button>
                )}
                {confirmDelete && (
                    <>
                        <span style={{fontSize: 12, color: '#f88'}}>Delete "{profileDropdown}"?</span>
                        <button style={dangerBtnStyle} onClick={handleDeleteProfile}>Confirm</button>
                        <button style={btnStyle} onClick={() => setConfirmDelete(false)}>Cancel</button>
                    </>
                )}
            </div>
            {activeProfile && (
                <p style={{fontSize: 12, color: '#888', marginTop: 6, marginBottom: 0}}>
                    Active profile: <strong style={{color: '#aaa'}}>{activeProfile}</strong>
                </p>
            )}
            {profileStatus && (
                <p style={{fontSize: 12, color: '#f88', marginTop: 6, marginBottom: 0}}>{profileStatus}</p>
            )}

            {hr()}

            {/* ── Tracked Abilities ── */}
            <h3 style={{marginTop: 0, marginBottom: 10}}>Tracked Abilities</h3>
            {abilities.length === 0 ? (
                <p style={{fontSize: 12, color: '#666'}}>
                    No abilities tracked yet. Use the Config window to set up detection and start tracking.
                </p>
            ) : (
                <div style={{display: 'flex', flexDirection: 'column', gap: 6}}>
                    {abilities.map(name => {
                        const cfg = getOrDefaultCfg(name)
                        const icon = abilityIcons[name]
                        const isExpanded = expandedAbility === name
                        const isPlacing = placingAbility === name
                        const hasPlacement = overlayConfigs[name] !== undefined

                        return (
                            <div key={name} style={{
                                background: '#252525', border: `1px solid ${isExpanded ? '#555' : '#333'}`,
                                borderRadius: 6, overflow: 'hidden',
                            }}>
                                {/* Row */}
                                <div style={{
                                    display: 'flex', alignItems: 'center', gap: 10, padding: '8px 12px',
                                    cursor: 'pointer',
                                }} onClick={() => setExpandedAbility(isExpanded ? null : name)}>
                                    {icon
                                        ? <img src={`data:image/png;base64,${icon}`} width={36} height={36}
                                            style={{imageRendering: 'pixelated', display: 'block', borderRadius: 3, flexShrink: 0}}/>
                                        : <div style={{width: 36, height: 36, background: '#3a3a3a', borderRadius: 3, flexShrink: 0,
                                            display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 10, color: '#666'}}>?</div>
                                    }
                                    <div style={{flex: 1}}>
                                        <div style={{fontWeight: 600}}>{name.replace(/_/g, ' ')}</div>
                                        {hasPlacement
                                            ? <div style={{fontSize: 11, color: '#888', marginTop: 2}}>
                                                X: {Math.round(cfg.x)}&nbsp;&nbsp;Y: {Math.round(cfg.y)}&nbsp;&nbsp;Size: {Math.round(cfg.size)}
                                              </div>
                                            : <div style={{fontSize: 11, color: '#666', marginTop: 2}}>Not placed</div>
                                        }
                                    </div>
                                    <button
                                        style={{
                                            ...btnStyle,
                                            fontSize: 12,
                                            background: isPlacing ? '#c0392b' : '#1a3a5a',
                                            color: 'white', border: 'none',
                                        }}
                                        onClick={e => { e.stopPropagation(); handlePlace(name) }}
                                    >
                                        {isPlacing ? 'Placing...' : hasPlacement ? 'Re-place' : 'Place'}
                                    </button>
                                    <span style={{color: '#666', fontSize: 12}}>{isExpanded ? '▲' : '▼'}</span>
                                </div>

                                {/* Expanded panel */}
                                {isExpanded && (
                                    <div style={{
                                        borderTop: '1px solid #333', padding: '12px 12px 14px',
                                        display: 'flex', flexDirection: 'column', gap: 14,
                                    }}>
                                        {/* Position D-pad + Size */}
                                        <div style={{display: 'flex', gap: 30, alignItems: 'flex-start', flexWrap: 'wrap'}}>
                                            <DPad
                                                label="Position"
                                                onLeft={() => AdjustIconPlacement(name, -4, 0, 0)}
                                                onRight={() => AdjustIconPlacement(name, 4, 0, 0)}
                                                onUp={() => AdjustIconPlacement(name, 0, -4, 0)}
                                                onDown={() => AdjustIconPlacement(name, 0, 4, 0)}
                                                leftTitle="Move left 4px" rightTitle="Move right 4px"
                                                upTitle="Move up 4px" downTitle="Move down 4px"
                                            />
                                            <div style={{display: 'flex', flexDirection: 'column', gap: 8}}>
                                                <div style={{fontSize: 12, color: '#aaa', fontWeight: 600, letterSpacing: 1, textTransform: 'uppercase'}}>Size</div>
                                                <div style={{display: 'flex', alignItems: 'center', gap: 8}}>
                                                    <button style={{...btnStyle, fontSize: 14, padding: '2px 10px'}}
                                                        onClick={() => AdjustIconPlacement(name, 0, 0, -4)}>−</button>
                                                    <span style={{minWidth: 30, textAlign: 'center', fontSize: 13}}>{Math.round(cfg.size)}</span>
                                                    <button style={{...btnStyle, fontSize: 14, padding: '2px 10px'}}
                                                        onClick={() => AdjustIconPlacement(name, 0, 0, 4)}>+</button>
                                                </div>
                                            </div>
                                        </div>

                                        {/* Glow options */}
                                        <div style={{display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap'}}>
                                            <label style={{display: 'flex', alignItems: 'center', gap: 6, cursor: 'pointer'}}>
                                                <input
                                                    type="checkbox"
                                                    checked={cfg.glowEnabled}
                                                    onChange={e => handleCfgChange(name, {glowEnabled: e.target.checked})}
                                                />
                                                <span style={{fontSize: 13}}>Glow when ready</span>
                                            </label>
                                            {cfg.glowEnabled && (
                                                <label style={{display: 'flex', alignItems: 'center', gap: 6}}>
                                                    <span style={{fontSize: 13, color: '#aaa'}}>Duration (s)</span>
                                                    <input
                                                        type="number" step="0.1" min="0.1" max="10"
                                                        value={cfg.glowDuration}
                                                        onChange={e => handleCfgChange(name, {glowDuration: parseFloat(e.target.value) || 1.0})}
                                                        style={{...inputStyle, width: 70}}
                                                    />
                                                </label>
                                            )}
                                        </div>

                                        {/* On cooldown behavior */}
                                        <div style={{display: 'flex', alignItems: 'center', gap: 10}}>
                                            <span style={{fontSize: 13}}>On cooldown:</span>
                                            <select
                                                value={cfg.onCooldown}
                                                onChange={e => handleCfgChange(name, {onCooldown: e.target.value})}
                                                style={{...inputStyle, minWidth: 140}}
                                            >
                                                <option value="translucent">Translucent</option>
                                                <option value="hidden">Hidden</option>
                                            </select>
                                        </div>

                                        {/* Remove */}
                                        {hasPlacement && (
                                            <div>
                                                <button style={{...dangerBtnStyle, fontSize: 12}} onClick={() => handleRemove(name)}>
                                                    Remove Overlay
                                                </button>
                                            </div>
                                        )}
                                    </div>
                                )}
                            </div>
                        )
                    })}
                </div>
            )}
        </div>
    )
}
