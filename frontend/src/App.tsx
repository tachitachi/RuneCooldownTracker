import {useState, useEffect, useRef, useCallback} from 'react'
import {Events} from '@wailsio/runtime'
import {ConfirmSnip, CancelSnipping} from '../bindings/github.com/tachitachi/RuneCooldownTracker/app'
import './App.css'

interface DragRect {
    x: number
    y: number
    w: number
    h: number
}

export default function App() {
    const [snipping, setSnipping] = useState(false)
    const [rect, setRect] = useState<DragRect | null>(null)
    const startRef = useRef<{x: number; y: number} | null>(null)
    const isDrawing = useRef(false)

    // Listen for snipping:start from Go
    useEffect(() => {
        const off = Events.On('snipping:start', () => {
            setSnipping(true)
            setRect(null)
        })
        return () => off()
    }, [])

    // Keyboard: Enter/F1 to confirm, Escape to cancel
    useEffect(() => {
        if (!snipping) return
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Enter' || e.key === 'F1') {
                e.preventDefault()
                if (rect && rect.w > 0 && rect.h > 0) {
                    ConfirmSnip(rect.x, rect.y, rect.w, rect.h)
                    setSnipping(false)
                    setRect(null)
                }
            } else if (e.key === 'Escape') {
                CancelSnipping()
                setSnipping(false)
                setRect(null)
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [snipping, rect])

    const onMouseDown = useCallback((e: React.MouseEvent) => {
        startRef.current = {x: e.clientX, y: e.clientY}
        isDrawing.current = true
        setRect({x: e.clientX, y: e.clientY, w: 0, h: 0})
    }, [])

    const onMouseMove = useCallback((e: React.MouseEvent) => {
        if (!isDrawing.current || !startRef.current) return
        const sx = startRef.current.x
        const sy = startRef.current.y
        setRect({
            x: Math.min(sx, e.clientX),
            y: Math.min(sy, e.clientY),
            w: Math.abs(e.clientX - sx),
            h: Math.abs(e.clientY - sy),
        })
    }, [])

    const onMouseUp = useCallback(() => {
        isDrawing.current = false
    }, [])

    if (snipping) {
        const hasRect = rect && rect.w > 0 && rect.h > 0
        return (
            <div
                style={{
                    position: 'fixed',
                    inset: 0,
                    background: 'rgba(0,0,0,0.45)',
                    cursor: 'crosshair',
                    userSelect: 'none',
                }}
                onMouseDown={onMouseDown}
                onMouseMove={onMouseMove}
                onMouseUp={onMouseUp}
            >
                {hasRect && (
                    <div
                        style={{
                            position: 'fixed',
                            left: rect!.x,
                            top: rect!.y,
                            width: rect!.w,
                            height: rect!.h,
                            border: '2px solid #00aaff',
                            boxShadow: '0 0 0 9999px rgba(0,0,0,0.45)',
                            background: 'rgba(0,170,255,0.1)',
                            boxSizing: 'border-box',
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
                    Drag to select area — Enter/F1 to confirm, Escape to cancel
                </div>
            </div>
        )
    }

    // Normal overlay content (transparent, non-interactive)
    return (
        <div id="App">
            {/* ability cooldown HUD will go here */}
        </div>
    )
}
