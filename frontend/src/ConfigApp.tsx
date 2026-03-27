import {useState, useEffect} from 'react'
import {Events} from '@wailsio/runtime'
import {StartSnipping, AdjustGridLayout, ExportIcons, StartTracking, StopTracking, GetDetectionParams, SetDetectionParams, GetTrackedAbilityNames, GetAbilityIcon, GetAbilityDetectionParams, SetAbilityDetectionParams, SetAllAbilityDetectionParams} from '../bindings/github.com/tachitachi/RuneCooldownTracker/internal/app/app'
import {DetectionParams} from '../bindings/github.com/tachitachi/RuneCooldownTracker/internal/detection/models'

interface SnipRegion {
    x: number
    y: number
    w: number
    h: number
}

interface DPadProps {
    label: string
    onLeft: () => void
    onRight: () => void
    onUp: () => void
    onDown: () => void
    leftTitle: string
    rightTitle: string
    upTitle: string
    downTitle: string
}

function DPad({label, onLeft, onRight, onUp, onDown, leftTitle, rightTitle, upTitle, downTitle}: DPadProps) {
    const btn: React.CSSProperties = {
        width: 36,
        height: 36,
        fontSize: 18,
        cursor: 'pointer',
        background: '#333',
        color: 'white',
        border: '1px solid #555',
        borderRadius: 4,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 0,
    }
    return (
        <div style={{display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6, margin: 'auto'}}>
            <div style={{fontSize: 12, color: '#aaa', fontWeight: 600, letterSpacing: 1, textTransform: 'uppercase'}}>
                {label}
            </div>
            <div style={{display: 'grid', gridTemplateColumns: '36px 36px 36px', gridTemplateRows: '36px 36px 36px', gap: 4}}>
                <div/>
                <button style={btn} title={upTitle} onClick={onUp}>↑</button>
                <div/>
                <button style={btn} title={leftTitle} onClick={onLeft}>←</button>
                <div style={{width: 36, height: 36, background: '#222', borderRadius: 4}}/>
                <button style={btn} title={rightTitle} onClick={onRight}>→</button>
                <div/>
                <button style={btn} title={downTitle} onClick={onDown}>↓</button>
                <div/>
            </div>
        </div>
    )
}

export default function ConfigApp() {
    const [region, setRegion] = useState<SnipRegion | null>(null)
    const [snipping, setSnipping] = useState(false)
    const [exportStatus, setExportStatus] = useState<string | null>(null)
    const [exporting, setExporting] = useState(false)
    const [tracking, setTracking] = useState(false)
    const [trackingStatus, setTrackingStatus] = useState<string | null>(null)
    const [timerAbsLuma, setTimerAbsLuma] = useState('0.75')
    const [timerBrightDiff, setTimerBrightDiff] = useState('0.15')
    const [timerEdgeDiff, setTimerEdgeDiff] = useState('0.15')
    const [timerMinPixels, setTimerMinPixels] = useState('4')
    const [paramsStatus, setParamsStatus] = useState<string | null>(null)
    const [abilityNames, setAbilityNames] = useState<string[]>([])
    const [abilityIcons, setAbilityIcons] = useState<Record<string, string>>({})
    const [selectedAbility, setSelectedAbility] = useState<string | null>(null)

    useEffect(() => {
        GetDetectionParams().then(p => {
            if (!p) return
            setTimerAbsLuma(String(p.timerAbsLuma))
            setTimerBrightDiff(String(p.timerBrightDiff))
            setTimerEdgeDiff(String(p.timerEdgeDiff))
            setTimerMinPixels(String(p.timerMinPixels))
        })
    }, [])

    async function loadAbilityList() {
        const names = await GetTrackedAbilityNames()
        if (!names || names.length === 0) {
            setAbilityNames([])
            setAbilityIcons({})
            setSelectedAbility(null)
            return
        }
        setAbilityNames(names)
        const icons: Record<string, string> = {}
        await Promise.all(names.map(async name => {
            const b64 = await GetAbilityIcon(name)
            if (b64) icons[name] = b64
        }))
        setAbilityIcons(icons)
    }

    useEffect(() => {
        const offConfirmed = Events.On('snipping:confirmed', (ev: any) => {
            setRegion(ev.data)
            setSnipping(false)
        })
        const offCancelled = Events.On('snipping:cancelled', () => {
            setSnipping(false)
        })
        return () => {
            offConfirmed()
            offCancelled()
        }
    }, [])

    function handleSetCaptureArea() {
        setSnipping(true)
        StartSnipping()
    }

    async function handleToggleTracking() {
        if (tracking) {
            StopTracking()
            setTracking(false)
            setTrackingStatus(null)
        } else {
            const msg = await StartTracking()
            if (msg && msg.startsWith('Error') || msg === 'Capture not running.' || msg === 'No grid layout detected yet — set a capture area first.' || msg === 'No frame captured yet — make sure the game is visible.') {
                setTrackingStatus(msg)
            } else {
                setTracking(true)
                setTrackingStatus(msg)
            }
        }
    }

    function buildParams() {
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

    async function handleUpdateParams() {
        const params = buildParams()
        if (!params) return
        if (selectedAbility) {
            await SetAbilityDetectionParams(selectedAbility, params)
            setParamsStatus(`Saved for ${selectedAbility}.`)
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

    async function handleSelectAbility(name: string) {
        if (selectedAbility === name) {
            // deselect — load global params
            setSelectedAbility(null)
            const p = await GetDetectionParams()
            if (p) {
                setTimerAbsLuma(String(p.timerAbsLuma))
                setTimerBrightDiff(String(p.timerBrightDiff))
                setTimerEdgeDiff(String(p.timerEdgeDiff))
                setTimerMinPixels(String(p.timerMinPixels))
            }
        } else {
            setSelectedAbility(name)
            const p = await GetAbilityDetectionParams(name)
            if (p) {
                setTimerAbsLuma(String(p.timerAbsLuma))
                setTimerBrightDiff(String(p.timerBrightDiff))
                setTimerEdgeDiff(String(p.timerEdgeDiff))
                setTimerMinPixels(String(p.timerMinPixels))
            }
        }
        setParamsStatus(null)
    }

    async function handleExportIcons() {
        setExporting(true)
        setExportStatus(null)
        const msg = await ExportIcons()
        setExporting(false)
        if (msg) setExportStatus(msg)
    }

    return (
        <div style={{background: '#1e1e1e', minHeight: '100vh', color: 'white', padding: 24, boxSizing: 'border-box'}}>
            <h2 style={{marginTop: 0}}>RuneCooldownTracker Config</h2>

            <p>
                Capture region:{' '}
                {region
                    ? `X:${region.x} Y:${region.y} W:${region.w} H:${region.h} (physical px)`
                    : 'Not set'}
            </p>
            <button
                onClick={handleSetCaptureArea}
                disabled={snipping}
                style={{padding: '8px 16px', cursor: snipping ? 'default' : 'pointer'}}
            >
                {snipping ? 'Waiting for clicks...' : 'Set Capture Area'}
            </button>
            {snipping && (
                <p style={{fontSize: 13, color: '#aaa'}}>
                    Switch to the game window, click the top-left ability, then the bottom-right ability.
                    Press Escape to cancel.
                </p>
            )}

            <hr style={{borderColor: '#333', margin: '24px 0'}}/>

            <h3 style={{marginTop: 0, marginBottom: 12}}>Export</h3>
            <button
                onClick={handleExportIcons}
                disabled={exporting}
                style={{padding: '8px 16px', cursor: exporting ? 'default' : 'pointer'}}
            >
                {exporting ? 'Exporting...' : 'Export Icons'}
            </button>
            {exportStatus && (
                <p style={{fontSize: 13, color: exportStatus.startsWith('Error') ? '#f88' : '#8f8', marginTop: 8}}>
                    {exportStatus}
                </p>
            )}

            <hr style={{borderColor: '#333', margin: '24px 0'}}/>

            <h3 style={{marginTop: 0, marginBottom: 12}}>Tracking</h3>
            <button
                onClick={handleToggleTracking}
                style={{padding: '8px 16px', cursor: 'pointer',
                    background: tracking ? '#c0392b' : undefined}}
            >
                {tracking ? 'Stop Tracking' : 'Start Tracking'}
            </button>
            {trackingStatus && (
                <p style={{fontSize: 13, color: trackingStatus.startsWith('Tracking') ? '#8f8' : '#f88', marginTop: 8}}>
                    {trackingStatus}
                </p>
            )}

            <hr style={{borderColor: '#333', margin: '24px 0'}}/>

            <h3 style={{marginTop: 0, marginBottom: 16}}>Grid Fine-Tuning</h3>
            <p style={{fontSize: 12, color: '#888', marginTop: 0, marginBottom: 16}}>
                Adjust if the detected grid lines are slightly off. Each click moves by 1 physical pixel.
            </p>
            <div style={{display: 'flex', gap: 40}}>
                <DPad
                    label="Columns"
                    onLeft={() => AdjustGridLayout(-1, 0, 0, 0)}
                    onRight={() => AdjustGridLayout(1, 0, 0, 0)}
                    onUp={() => AdjustGridLayout(0, 1, 0, 0)}
                    onDown={() => AdjustGridLayout(0, -1, 0, 0)}
                    leftTitle="Shift column lines left (ColPhase −1)"
                    rightTitle="Shift column lines right (ColPhase +1)"
                    upTitle="Widen column spacing (ColPeriod +1)"
                    downTitle="Narrow column spacing (ColPeriod −1)"
                />
                <DPad
                    label="Rows"
                    onLeft={() => AdjustGridLayout(0, 0, -1, 0)}
                    onRight={() => AdjustGridLayout(0, 0, 1, 0)}
                    onUp={() => AdjustGridLayout(0, 0, 0, 1)}
                    onDown={() => AdjustGridLayout(0, 0, 0, -1)}
                    leftTitle="Shift row lines up (RowPhase −1)"
                    rightTitle="Shift row lines down (RowPhase +1)"
                    upTitle="Widen row spacing (RowPeriod +1)"
                    downTitle="Narrow row spacing (RowPeriod −1)"
                />
            </div>

            <hr style={{borderColor: '#333', margin: '24px 0'}}/>

            <h3 style={{marginTop: 0, marginBottom: 8}}>Detection Parameters</h3>
            <p style={{fontSize: 12, color: '#888', marginTop: 0, marginBottom: 12}}>
                Select an ability to tune its individual thresholds, or leave none selected to edit global defaults.
            </p>

            <div style={{display: 'flex', alignItems: 'center', gap: 10, marginBottom: 10}}>
                <button onClick={loadAbilityList} style={{padding: '5px 12px', cursor: 'pointer', fontSize: 12}}>
                    Refresh Abilities
                </button>
                {selectedAbility && (
                    <span style={{fontSize: 12, color: '#aaa'}}>Selected: <strong style={{color: 'white'}}>{selectedAbility.replace(/_/g, ' ')}</strong></span>
                )}
                {!selectedAbility && abilityNames.length > 0 && (
                    <span style={{fontSize: 12, color: '#888'}}>No ability selected — editing global defaults</span>
                )}
            </div>

            {abilityNames.length === 0 && (
                <p style={{fontSize: 12, color: '#666', marginTop: 0, marginBottom: 12}}>
                    No abilities tracked yet. Start tracking first, then click Refresh Abilities.
                </p>
            )}

            {abilityNames.length > 0 && (
                <div style={{display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 16}}>
                    {abilityNames.map(name => {
                        const isSelected = selectedAbility === name
                        const icon = abilityIcons[name]
                        return (
                            <button
                                key={name}
                                title={name.replace(/_/g, ' ')}
                                onClick={() => handleSelectAbility(name)}
                                style={{
                                    padding: 3,
                                    cursor: 'pointer',
                                    background: isSelected ? '#1a4a2a' : '#2a2a2a',
                                    border: isSelected ? '2px solid #4caf50' : '2px solid #444',
                                    borderRadius: 6,
                                    display: 'flex',
                                    flexDirection: 'column',
                                    alignItems: 'center',
                                    gap: 3,
                                    width: 62,
                                }}
                            >
                                {icon
                                    ? <img src={`data:image/png;base64,${icon}`} width={48} height={48} style={{imageRendering: 'pixelated', display: 'block'}}/>
                                    : <div style={{width: 48, height: 48, background: '#333', borderRadius: 3}}/>
                                }
                                <span style={{fontSize: 9, color: '#aaa', lineHeight: 1.1, textAlign: 'center', wordBreak: 'break-word', maxWidth: 56}}>
                                    {name.replace(/_/g, ' ')}
                                </span>
                            </button>
                        )
                    })}
                </div>
            )}

            <div style={{display: 'grid', gridTemplateColumns: 'auto 120px', gap: '10px 12px', alignItems: 'center', maxWidth: 320}}>
                <label style={{fontSize: 13}}>Timer Abs Luma</label>
                <input type="number" step="0.01" value={timerAbsLuma} onChange={e => setTimerAbsLuma(e.target.value)}
                    style={{padding: '4px 8px', background: '#2a2a2a', color: 'white', border: '1px solid #555', borderRadius: 4, fontSize: 13}}/>
                <label style={{fontSize: 13}}>Timer Bright Diff</label>
                <input type="number" step="0.01" value={timerBrightDiff} onChange={e => setTimerBrightDiff(e.target.value)}
                    style={{padding: '4px 8px', background: '#2a2a2a', color: 'white', border: '1px solid #555', borderRadius: 4, fontSize: 13}}/>
                <label style={{fontSize: 13}}>Timer Edge Diff</label>
                <input type="number" step="0.01" value={timerEdgeDiff} onChange={e => setTimerEdgeDiff(e.target.value)}
                    style={{padding: '4px 8px', background: '#2a2a2a', color: 'white', border: '1px solid #555', borderRadius: 4, fontSize: 13}}/>
                <label style={{fontSize: 13}}>Timer Min Pixels</label>
                <input type="number" step="1" min="1" value={timerMinPixels} onChange={e => setTimerMinPixels(e.target.value)}
                    style={{padding: '4px 8px', background: '#2a2a2a', color: 'white', border: '1px solid #555', borderRadius: 4, fontSize: 13}}/>
            </div>
            <div style={{display: 'flex', gap: 8, marginTop: 14}}>
                <button onClick={handleUpdateParams} style={{padding: '8px 16px', cursor: 'pointer'}}>
                    {selectedAbility ? `Update ${selectedAbility.replace(/_/g, ' ')}` : 'Update Global'}
                </button>
                <button onClick={handleUpdateAllParams} style={{padding: '8px 16px', cursor: 'pointer'}}>
                    Update All
                </button>
            </div>
            {paramsStatus && (
                <p style={{fontSize: 13, color: paramsStatus.startsWith('Invalid') ? '#f88' : '#8f8', marginTop: 8}}>
                    {paramsStatus}
                </p>
            )}
        </div>
    )
}
