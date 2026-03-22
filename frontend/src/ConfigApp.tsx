import {useState, useEffect} from 'react'
import {Events} from '@wailsio/runtime'
import {StartSnipping} from '../bindings/RuneCooldownTracker/app'

interface SnipRegion {
    x: number
    y: number
    w: number
    h: number
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
                {snipping ? 'Drawing...' : 'Set Capture Area'}
            </button>
            {snipping && (
                <p style={{fontSize: 13, color: '#aaa'}}>
                    Switch to the game window and drag to select an area.
                    Press Enter/F1 to confirm, Escape to cancel.
                </p>
            )}
        </div>
    )
}
