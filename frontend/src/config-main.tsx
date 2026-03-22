import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import ConfigApp from './ConfigApp'

createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
        <ConfigApp/>
    </React.StrictMode>
)
