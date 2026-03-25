import {useState, useEffect} from 'react'
import {Events} from '@wailsio/runtime'
import {StartSnipping, AdjustGridLayout} from '../bindings/github.com/tachitachi/RuneCooldownTracker/internal/app/app'

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
        </div>
    )
}
