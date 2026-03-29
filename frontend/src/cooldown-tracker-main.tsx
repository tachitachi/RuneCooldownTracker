import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import CooldownTrackerApp from './CooldownTrackerApp'

createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
        <CooldownTrackerApp/>
    </React.StrictMode>
)
