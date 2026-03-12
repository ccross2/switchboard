# Switchboard

Unified messenger for sovereign communication. Native desktop app with direct protocol bridges for WhatsApp, Telegram, and more — no third-party relay, no data leaving your machine.

Part of the [Sovren Software](https://sovren.software) product suite.

---

## Status

Early development (`v0.1.0`). Core application shell established. Protocol bridges in progress.

---

## Stack

| Layer | Technology |
|-------|-----------|
| App framework | [Tauri 2](https://tauri.app) — Rust backend + system WebView |
| Frontend | React 18 + TypeScript + Vite |
| State | Zustand |
| Notifications | Tauri plugin-notification |
| Platforms | Linux, macOS, Windows |

---

## Development

**Prerequisites:** Node.js 18+, Rust stable, system WebView (Linux: `libwebkit2gtk-4.1`)

```bash
# Install dependencies
npm install

# Development — hot-reload frontend + native window
npm run tauri dev

# Production build
npm run tauri build
```

---

## The Sovren Stack

```
Esver OS      sovereign compute
Visage        sovereign identity
Switchboard   sovereign communication
MrHaven       sovereign capital
```

---

Copyright (c) 2026 Sovren Software. All rights reserved.
