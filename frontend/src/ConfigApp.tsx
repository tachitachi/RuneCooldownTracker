import {useState, useEffect, useCallback} from 'react'
import {Events} from '@wailsio/runtime'
import {
    StartSnipping, AdjustGridLayout, ExportIcons,
    StartTracking, StopTracking,
    AutoDetectAbilities, GetGridSlots, SetSlotAbility, ClearSlotAbility, GetAllAbilityNames,
    GetDetectionParams, SetDetectionParams, GetAbilityDetectionParams, SetAbilityDetectionParams, SetAllAbilityDetectionParams,
    GetAbilityIcon,
    GetProfiles, GetActiveProfile, CreateProfile, DeleteProfile, LoadProfile,
} from '../bindings/github.com/tachitachi/RuneCooldownTracker/internal/app/app'
import {DetectionParams} from '../bindings/github.com/tachitachi/RuneCooldownTracker/internal/detection/models'
import {GridSlot} from '../bindings/github.com/tachitachi/RuneCooldownTracker/internal/app/models'

interface SnipRegion { x: number; y: number; w: number; h: number }

function DPad({label, onLeft, onRight, onUp, onDown, leftTitle, rightTitle, upTitle, downTitle}: {
    label: string
    onLeft: () => void; onRight: () => void; onUp: () => void; onDown: () => void
    leftTitle: string; rightTitle: string; upTitle: string; downTitle: string
}) {
    const btn: React.CSSProperties = {
        width: 36, height: 36, fontSize: 18, cursor: 'pointer',
        background: '#333', color: 'white', border: '1px solid #555',
        borderRadius: 4, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 0,
    }
    return (
        <div style={{display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6, margin: 'auto'}}>
            <div style={{fontSize: 12, color: '#aaa', fontWeight: 600, letterSpacing: 1, textTransform: 'uppercase'}}>{label}</div>
            <div style={{display: 'grid', gridTemplateColumns: '36px 36px 36px', gridTemplateRows: '36px 36px 36px', gap: 4}}>
                <div/><button style={btn} title={upTitle} onClick={onUp}>↑</button><div/>
                <button style={btn} title={leftTitle} onClick={onLeft}>←</button>
                <div style={{width: 36, height: 36, background: '#222', borderRadius: 4}}/>
                <button style={btn} title={rightTitle} onClick={onRight}>→</button>
                <div/><button style={btn} title={downTitle} onClick={onDown}>↓</button><div/>
            </div>
        </div>
    )
}

function hr() {
    return <hr style={{borderColor: '#333', margin: '20px 0'}}/>
}

export default function ConfigApp() {
    // Capture area
    const [region, setRegion] = useState<SnipRegion | null>(null)
    const [snipping, setSnipping] = useState(false)

    // Tracking
    const [tracking, setTracking] = useState(false)
    const [trackingStatus, setTrackingStatus] = useState<string | null>(null)

    // Auto-detect
    const [detecting, setDetecting] = useState(false)
    const [detectStatus, setDetectStatus] = useState<string | null>(null)

    // Config grid
    const [gridSlots, setGridSlots] = useState<GridSlot[]>([])
    const [abilityIcons, setAbilityIcons] = useState<Record<string, string>>({})
    const [selectedSlot, setSelectedSlot] = useState<{col: number; row: number} | null>(null)

    // Manual assignment dropdown
    const [allAbilityNames, setAllAbilityNames] = useState<string[]>([])
    const [assignDropdown, setAssignDropdown] = useState('')

    // Detection params
    const [timerAbsLuma, setTimerAbsLuma] = useState('0.75')
    const [timerBrightDiff, setTimerBrightDiff] = useState('0.15')
    const [timerEdgeDiff, setTimerEdgeDiff] = useState('0.15')
    const [timerMinPixels, setTimerMinPixels] = useState('4')
    const [paramsStatus, setParamsStatus] = useState<string | null>(null)

    // Profiles
    const [profiles, setProfiles] = useState<string[]>([])
    const [activeProfile, setActiveProfile] = useState('')
    const [profileDropdown, setProfileDropdown] = useState('')
    const [creatingProfile, setCreatingProfile] = useState(false)
    const [newProfileName, setNewProfileName] = useState('')
    const [confirmDelete, setConfirmDelete] = useState(false)
    const [profileStatus, setProfileStatus] = useState<string | null>(null)

    // Export
    const [exporting, setExporting] = useState(false)
    const [exportStatus, setExportStatus] = useState<string | null>(null)

    // ── Helpers ──────────────────────────────────────────────────────────────

    const numCols = gridSlots.length > 0 ? Math.max(...gridSlots.map(s => s.col)) + 1 : 0
    const numRows = gridSlots.length > 0 ? Math.max(...gridSlots.map(s => s.row)) + 1 : 0

    const selectedSlotData = selectedSlot
        ? gridSlots.find(s => s.col === selectedSlot.col && s.row === selectedSlot.row) ?? null
        : null

    const selectedAbilityName = selectedSlotData?.name ?? ''

    async function loadAbilityIcons(slots: GridSlot[]) {
        const needed = [...new Set(slots.map(s => s.name).filter(Boolean))]
        const icons: Record<string, string> = {...abilityIcons}
        await Promise.all(needed.map(async name => {
            if (!icons[name]) {
                const b64 = await GetAbilityIcon(name)
                if (b64) icons[name] = b64
            }
        }))
        setAbilityIcons(icons)
    }

    const refreshGrid = useCallback(async () => {
        const slots = await GetGridSlots()
        if (slots) {
            setGridSlots(slots)
            await loadAbilityIcons(slots)
        }
    }, [])

    async function loadDetectionParams(abilityName: string) {
        if (abilityName) {
            const p = await GetAbilityDetectionParams(abilityName)
            if (p) {
                setTimerAbsLuma(String(p.timerAbsLuma))
                setTimerBrightDiff(String(p.timerBrightDiff))
                setTimerEdgeDiff(String(p.timerEdgeDiff))
                setTimerMinPixels(String(p.timerMinPixels))
            }
        } else {
            const p = await GetDetectionParams()
            if (p) {
                setTimerAbsLuma(String(p.timerAbsLuma))
                setTimerBrightDiff(String(p.timerBrightDiff))
                setTimerEdgeDiff(String(p.timerEdgeDiff))
                setTimerMinPixels(String(p.timerMinPixels))
            }
        }
        setParamsStatus(null)
    }

    function buildParams(): DetectionParams | null {
        const params = new DetectionParams({
            timerAbsLuma: parseFloat(timerAbsLuma),
            timerBrightDiff: parseFloat(timerBrightDiff),
            timerEdgeDiff: parseFloat(timerEdgeDiff),
            timerMinPixels: parseInt(timerMinPixels, 10),
        })
        if ([params.timerAbsLuma, params.timerBrightDiff, params.timerEdgeDiff, params.timerMinPixels].some(isNaN)) {
            setParamsStatus('Invalid values — all fields must be numbers.')
            return null
        }
        return params
    }

    // ── Startup ───────────────────────────────────────────────────────────────

    useEffect(() => {
        GetDetectionParams().then(p => {
            if (!p) return
            setTimerAbsLuma(String(p.timerAbsLuma))
            setTimerBrightDiff(String(p.timerBrightDiff))
            setTimerEdgeDiff(String(p.timerEdgeDiff))
            setTimerMinPixels(String(p.timerMinPixels))
        })
        GetAllAbilityNames().then(names => {
            if (names) {
                setAllAbilityNames(names)
                setAssignDropdown(names[0] ?? '')
            }
        })
        refreshGrid()
        GetProfiles().then(ps => { if (ps) setProfiles(ps) })
        GetActiveProfile().then(ap => {
            if (ap != null) {
                setActiveProfile(ap)
                setProfileDropdown(ap)
            }
        })
    }, [])

    useEffect(() => {
        const offConfirmed = Events.On('snipping:confirmed', (ev: any) => {
            setRegion(ev.data)
            setSnipping(false)
        })
        const offCancelled = Events.On('snipping:cancelled', () => setSnipping(false))
        return () => { offConfirmed(); offCancelled() }
    }, [])

    // ── Event handlers ────────────────────────────────────────────────────────

    function handleSetCaptureArea() {
        setSnipping(true)
        StartSnipping()
    }

    async function handleAutoDetect() {
        setDetecting(true)
        setDetectStatus(null)
        const msg = await AutoDetectAbilities()
        setDetecting(false)
        setDetectStatus(msg ?? null)
        await refreshGrid()
        setSelectedSlot(null)
    }

    async function handleToggleTracking() {
        if (tracking) {
            StopTracking()
            setTracking(false)
            setTrackingStatus(null)
        } else {
            const msg = await StartTracking()
            const isError = !msg || msg.startsWith('No') || msg.startsWith('Capture') || msg.startsWith('Error')
            setTrackingStatus(msg ?? null)
            if (!isError) setTracking(true)
        }
    }

    async function handleSlotClick(col: number, row: number) {
        if (selectedSlot?.col === col && selectedSlot?.row === row) {
            setSelectedSlot(null)
            await loadDetectionParams('')
        } else {
            setSelectedSlot({col, row})
            const slot = gridSlots.find(s => s.col === col && s.row === row)
            await loadDetectionParams(slot?.name ?? '')
        }
        setParamsStatus(null)
    }

    async function handleClearSlot() {
        if (!selectedSlot) return
        await ClearSlotAbility(selectedSlot.col, selectedSlot.row)
        await refreshGrid()
        await loadDetectionParams('')
        setSelectedSlot(null)
    }

    async function handleAssignSlot() {
        if (!selectedSlot || !assignDropdown) return
        await SetSlotAbility(selectedSlot.col, selectedSlot.row, assignDropdown)
        await refreshGrid()
        await loadDetectionParams(assignDropdown)
        // preload icon
        if (!abilityIcons[assignDropdown]) {
            const b64 = await GetAbilityIcon(assignDropdown)
            if (b64) setAbilityIcons(prev => ({...prev, [assignDropdown]: b64}))
        }
    }

    async function handleUpdateParams() {
        const params = buildParams()
        if (!params) return
        if (selectedAbilityName) {
            await SetAbilityDetectionParams(selectedAbilityName, params)
            setParamsStatus(`Saved for ${selectedAbilityName.replace(/_/g, ' ')}.`)
        } else {
            await SetDetectionParams(params)
            setParamsStatus('Saved (global defaults).')
        }
    }

    async function handleUpdateAllParams() {
        const params = buildParams()
        if (!params) return
        await SetAllAbilityDetectionParams(params)
        setParamsStatus('Saved for all tracked abilities.')
    }

    async function handleExportIcons() {
        setExporting(true)
        setExportStatus(null)
        const msg = await ExportIcons()
        setExporting(false)
        if (msg) setExportStatus(msg)
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
            await refreshGrid()
            setSelectedSlot(null)
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

    // ── Render ────────────────────────────────────────────────────────────────

    const inputStyle: React.CSSProperties = {
        padding: '4px 8px', background: '#2a2a2a', color: 'white',
        border: '1px solid #555', borderRadius: 4, fontSize: 13,
    }
    const btnStyle: React.CSSProperties = {padding: '6px 14px', cursor: 'pointer'}
    const dangerBtnStyle: React.CSSProperties = {...btnStyle, background: '#7b1a1a', color: 'white', border: '1px solid #a33'}

    return (
        <div style={{background: '#1e1e1e', minHeight: '100vh', color: 'white', padding: 20, boxSizing: 'border-box', fontSize: 14}}>
            <h2 style={{marginTop: 0, marginBottom: 16}}>RuneCooldownTracker Config</h2>

            {/* ── Profiles ── */}
            <h3 style={{marginTop: 0, marginBottom: 10}}>Profiles</h3>
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

            {/* ── Capture Area ── */}
            <h3 style={{marginTop: 0, marginBottom: 8}}>Capture Area</h3>
            <p style={{marginTop: 0, marginBottom: 8, fontSize: 13, color: '#aaa'}}>
                {region
                    ? `X:${region.x} Y:${region.y} W:${region.w} H:${region.h} (physical px)`
                    : 'Not set'}
            </p>
            <button onClick={handleSetCaptureArea} disabled={snipping} style={btnStyle}>
                {snipping ? 'Waiting for clicks...' : 'Set Capture Area'}
            </button>
            {snipping && (
                <p style={{fontSize: 12, color: '#aaa', marginTop: 6}}>
                    Switch to the game window, click the top-left ability, then the bottom-right ability. Press Escape to cancel.
                </p>
            )}

            {hr()}

            {/* ── Grid Fine-Tuning ── */}
            <h3 style={{marginTop: 0, marginBottom: 8}}>Grid Fine-Tuning</h3>
            <p style={{fontSize: 12, color: '#888', marginTop: 0, marginBottom: 14}}>
                Adjust if the detected grid lines are slightly off. Each click moves by 1 physical pixel.
            </p>
            <div style={{display: 'flex', gap: 40}}>
                <DPad label="Columns"
                    onLeft={() => AdjustGridLayout(-1, 0, 0, 0)} onRight={() => AdjustGridLayout(1, 0, 0, 0)}
                    onUp={() => AdjustGridLayout(0, 1, 0, 0)} onDown={() => AdjustGridLayout(0, -1, 0, 0)}
                    leftTitle="Shift column lines left (ColPhase −1)" rightTitle="Shift column lines right (ColPhase +1)"
                    upTitle="Widen column spacing (ColPeriod +1)" downTitle="Narrow column spacing (ColPeriod −1)"
                />
                <DPad label="Rows"
                    onLeft={() => AdjustGridLayout(0, 0, -1, 0)} onRight={() => AdjustGridLayout(0, 0, 1, 0)}
                    onUp={() => AdjustGridLayout(0, 0, 0, 1)} onDown={() => AdjustGridLayout(0, 0, 0, -1)}
                    leftTitle="Shift row lines up (RowPhase −1)" rightTitle="Shift row lines down (RowPhase +1)"
                    upTitle="Widen row spacing (RowPeriod +1)" downTitle="Narrow row spacing (RowPeriod −1)"
                />
            </div>

            {hr()}

            {/* ── Auto-detect & Tracking ── */}
            <h3 style={{marginTop: 0, marginBottom: 10}}>Ability Detection</h3>
            <div style={{display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center'}}>
                <button onClick={handleAutoDetect} disabled={detecting} style={{...btnStyle, background: detecting ? '#333' : '#1a3a5a'}}>
                    {detecting ? 'Detecting...' : 'Auto-detect Abilities'}
                </button>
                <button
                    onClick={handleToggleTracking}
                    style={{...btnStyle, background: tracking ? '#c0392b' : undefined}}
                >
                    {tracking ? 'Stop Tracking' : 'Start Tracking'}
                </button>
            </div>
            {detectStatus && (
                <p style={{fontSize: 12, color: detectStatus.startsWith('No') || detectStatus.startsWith('Capture') || detectStatus.startsWith('Error') ? '#f88' : '#8f8', marginTop: 8, marginBottom: 0}}>
                    {detectStatus}
                </p>
            )}
            {trackingStatus && (
                <p style={{fontSize: 12, color: trackingStatus.startsWith('Tracking') || trackingStatus.includes('abilities') ? '#8f8' : '#f88', marginTop: 8, marginBottom: 0}}>
                    {trackingStatus}
                </p>
            )}

            {/* ── Config Grid ── */}
            {numCols > 0 && (
                <>
                    <p style={{fontSize: 12, color: '#888', marginTop: 12, marginBottom: 8}}>
                        Click a slot to select it. Green = identified, grey = unknown.
                    </p>
                    <div style={{
                        display: 'grid',
                        gridTemplateColumns: `repeat(${numCols}, 52px)`,
                        gap: 3,
                        marginBottom: 12,
                    }}>
                        {Array.from({length: numRows}, (_, row) =>
                            Array.from({length: numCols}, (_, col) => {
                                const slot = gridSlots.find(s => s.col === col && s.row === row)
                                const name = slot?.name ?? ''
                                const icon = name ? abilityIcons[name] : undefined
                                const isSelected = selectedSlot?.col === col && selectedSlot?.row === row
                                return (
                                    <button
                                        key={`${col}-${row}`}
                                        title={name ? name.replace(/_/g, ' ') : `Slot ${col},${row} (unassigned)`}
                                        onClick={() => handleSlotClick(col, row)}
                                        style={{
                                            width: 52, height: 52, padding: 2, cursor: 'pointer',
                                            background: isSelected ? '#1a3a1a' : (name ? '#1a2a1a' : '#2a2a2a'),
                                            border: isSelected ? '2px solid #4caf50' : (name ? '2px solid #2d5a2d' : '2px solid #444'),
                                            borderRadius: 4,
                                            display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
                                            gap: 2,
                                        }}
                                    >
                                        {icon
                                            ? <img src={`data:image/png;base64,${icon}`} width={44} height={44}
                                                style={{imageRendering: 'pixelated', display: 'block'}}/>
                                            : <div style={{width: 44, height: 44, background: '#3a3a3a', borderRadius: 3,
                                                display: 'flex', alignItems: 'center', justifyContent: 'center',
                                                fontSize: 10, color: '#666'}}>?</div>
                                        }
                                    </button>
                                )
                            })
                        )}
                    </div>

                    {/* ── Slot Info Panel ── */}
                    {selectedSlot && (
                        <div style={{
                            background: '#252525', border: '1px solid #444', borderRadius: 6,
                            padding: 12, marginBottom: 12,
                        }}>
                            <div style={{fontSize: 13, color: '#ccc', marginBottom: 8}}>
                                Slot ({selectedSlot.col}, {selectedSlot.row}){selectedAbilityName ? ': ' : ' — unassigned'}
                                {selectedAbilityName && (
                                    <strong style={{color: 'white'}}>{selectedAbilityName.replace(/_/g, ' ')}</strong>
                                )}
                            </div>
                            {selectedAbilityName ? (
                                <button onClick={handleClearSlot} style={{...dangerBtnStyle, fontSize: 12}}>
                                    Clear Ability
                                </button>
                            ) : (
                                <div style={{display: 'flex', gap: 8, alignItems: 'center'}}>
                                    <select
                                        value={assignDropdown}
                                        onChange={e => setAssignDropdown(e.target.value)}
                                        style={{...inputStyle, minWidth: 160}}
                                    >
                                        {allAbilityNames.map(n => (
                                            <option key={n} value={n}>{n.replace(/_/g, ' ')}</option>
                                        ))}
                                    </select>
                                    <button onClick={handleAssignSlot} style={{...btnStyle, fontSize: 12}}>
                                        Assign
                                    </button>
                                </div>
                            )}
                        </div>
                    )}
                </>
            )}

            {numCols === 0 && (
                <p style={{fontSize: 12, color: '#666', marginTop: 8}}>
                    No grid detected yet. Set a capture area, then click Auto-detect Abilities.
                </p>
            )}

            {hr()}

            {/* ── Detection Parameters ── */}
            <h3 style={{marginTop: 0, marginBottom: 8}}>Detection Parameters</h3>
            <p style={{fontSize: 12, color: '#888', marginTop: 0, marginBottom: 12}}>
                {selectedAbilityName
                    ? <>Editing: <strong style={{color: '#ccc'}}>{selectedAbilityName.replace(/_/g, ' ')}</strong></>
                    : 'Editing global defaults (select a slot with an ability to tune per-ability).'}
            </p>
            <div style={{display: 'grid', gridTemplateColumns: 'auto 120px', gap: '8px 12px', alignItems: 'center', maxWidth: 320}}>
                <label style={{fontSize: 13}}>Timer Abs Luma</label>
                <input type="number" step="0.01" value={timerAbsLuma} onChange={e => setTimerAbsLuma(e.target.value)} style={inputStyle}/>
                <label style={{fontSize: 13}}>Timer Bright Diff</label>
                <input type="number" step="0.01" value={timerBrightDiff} onChange={e => setTimerBrightDiff(e.target.value)} style={inputStyle}/>
                <label style={{fontSize: 13}}>Timer Edge Diff</label>
                <input type="number" step="0.01" value={timerEdgeDiff} onChange={e => setTimerEdgeDiff(e.target.value)} style={inputStyle}/>
                <label style={{fontSize: 13}}>Timer Min Pixels</label>
                <input type="number" step="1" min="1" value={timerMinPixels} onChange={e => setTimerMinPixels(e.target.value)} style={inputStyle}/>
            </div>
            <div style={{display: 'flex', gap: 8, marginTop: 12}}>
                <button onClick={handleUpdateParams} style={btnStyle}>
                    {selectedAbilityName ? `Update ${selectedAbilityName.replace(/_/g, ' ')}` : 'Update Global'}
                </button>
                <button onClick={handleUpdateAllParams} style={btnStyle}>Update All</button>
            </div>
            {paramsStatus && (
                <p style={{fontSize: 12, color: paramsStatus.startsWith('Invalid') ? '#f88' : '#8f8', marginTop: 8}}>
                    {paramsStatus}
                </p>
            )}

            {hr()}

            {/* ── Export ── */}
            <h3 style={{marginTop: 0, marginBottom: 8}}>Export</h3>
            <button onClick={handleExportIcons} disabled={exporting} style={btnStyle}>
                {exporting ? 'Exporting...' : 'Export Icons'}
            </button>
            {exportStatus && (
                <p style={{fontSize: 12, color: exportStatus.startsWith('Error') ? '#f88' : '#8f8', marginTop: 8}}>
                    {exportStatus}
                </p>
            )}
        </div>
    )
}
