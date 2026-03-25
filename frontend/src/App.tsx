import {useState, useEffect, useCallback} from 'react'
import {Events} from '@wailsio/runtime'
import {ConfirmSnip, CancelSnipping} from '../bindings/github.com/tachitachi/RuneCooldownTracker/internal/app/app'
import './App.css'

interface GridLines {
    xLines: number[]
    yLines: number[]
}

// Padding added around the two clicked points (logical px).
// One slot is ~27–54 logical px depending on DPI; 60 px safely covers the
// full outermost slot on each side at any common DPI scale.
const SNIP_MARGIN = 30

export default function App() {
    const [snipping, setSnipping] = useState(false)
    const [firstClick, setFirstClick] = useState<{x: number; y: number} | null>(null)
    const [gridLines, setGridLines] = useState<GridLines | null>(null)

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

    // Escape to cancel at any point
    useEffect(() => {
        if (!snipping) return
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                CancelSnipping()
                setSnipping(false)
                setFirstClick(null)
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [snipping])

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
            {gridLines?.xLines.map((x, i) => (
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
            {gridLines?.yLines.map((y, i) => (
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
        </div>
    )
}
