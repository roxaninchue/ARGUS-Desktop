# Blaze Desktop

Blaze, the Android assistant, as a desktop app for **Windows, macOS and Linux**, with dark frosted glass, rounded everything, and the drop in its gold halo.

Built with [Wails](https://wails.io) on the same Go core as Blaze CLI (PC detection, free model router,
tools, scheduler). Settings, keys, memory and skills live in `~/.blaze` and are shared with the CLI.

## Install (latest release)
- **Windows**: `Blaze-…-windows-amd64-setup.exe` (or the portable exe). SmartScreen: More info › Run anyway.
- **macOS**: `Blaze-…-macos-universal.dmg`, drag to Applications, then right-click › Open the first time.
- **Arch**: `sudo pacman -U blaze-desktop-*.pkg.tar.zst` (needs `webkit2gtk-4.1`).
- **Debian/Ubuntu**: `sudo apt install ./blaze-desktop_*_amd64.deb`.
- **Other Linux**: unpack the tar.gz and run `blaze-desktop` (needs GTK 3 + WebKitGTK 4.1).

## What it does
- **Chat that works on your computer**: streaming answers with plain-English steps, Plan and Build modes, Auto or Autopilot approvals, and a daily spend cap.
- **Any model**: pick a provider and model or use Auto, a free model router with live health, several keys per provider, and usage per model.
- **Design studio**: live preview at phone, tablet and desktop widths, edits that write back to your source, Directions as live branches, brand kit, the critic, visual history, phone preview, and one-click ship to Cloudflare or GitHub Pages.
- **3D mockups**: any 3D request starts as a Three.js mockup in Design. Click a part to recolour, scale, turn or nudge it, with every change written back to the file. When you named a program (Blender, FreeCAD, OpenSCAD and others), Blaze builds it there next, from the same parts.
- **Specialists on request**: Blaze does your tasks itself. Ask it to split a big task and parallel agents each work on their own git worktree, reviewed before anything lands.
- **Lives in the background**: tray icon, quick ask from anywhere (Ctrl+Shift+Space, Option+Space on a Mac), native notifications, and optional proactive suggestions.
- **Sees and hears**: reads the front window, attaches images, files and folders, and turns speech into text with Whisper on your own PC.
- **Memory and skills**: learns facts and workflows as you go, with 28 bundled skills you can edit or turn off. Blaze combines them when a request falls between skills (a hand-drawn poster, a pixel-art game with music), and saves a combination that keeps coming up as a skill of its own.
- **Connected**: one-click Google, GitHub, Spotify, Notion and Slack, MCP servers shared with Blaze CLI (`~/.blaze/mcp.json`), and keys that sync through your Blaze account.
- **Updates itself** from GitHub releases.

What changed in each version: [`CHANGELOG.md`](CHANGELOG.md).

## Build
Go 1.26 + `go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0`, then `wails build`
(Linux: `-tags webkit2_41`). CI builds every platform on push and publishes a release;
UI screenshots go to the `shots` branch.

**Why `frontend/dist` is committed.** The frontend is hand-written vanilla JS, CSS and HTML that
lives directly in `frontend/dist`. There is no `package.json`, bundler or build step
(`frontend:install` and `frontend:build` are empty in `wails.json`), so `dist` *is* the source:
edit files there. Only Wails' generated bindings (`frontend/wailsjs/`) are ignored.

## Security
How sign-in and one-click connectors work, what the `blaze-auth` Worker receives, stores and logs,
where your tokens end up, and how to revoke access: [`docs/SECURITY-auth.md`](docs/SECURITY-auth.md).
