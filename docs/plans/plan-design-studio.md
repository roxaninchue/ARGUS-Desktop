# Design studio (from the approved v3 concept)

Target: the Design tab becomes a studio for pages, 3D, motion and sound, matching the
approved desktop concept (1440×900): mode rail on the left, a canvas with floating glass
toolbars, a versions/timeline drawer at the bottom, and a side panel with Chat · Inspect ·
Critic · Brand. Light and dark follow the app theme. Branch `feat/design-studio`, built on
`feat/mobile-design` (v0.18.2). Nothing from the existing Design tab is removed: the studio
re-arranges and extends app.js / studio.js / artboard.js.

## Phase 1: studio shell, Page mode, colour wheel, move/resize (this MR)

Built:
- **Studio layout** (`frontend/dist/dstudio.js`, `dstudio.css`): mode rail (Page, with 3D,
  Motion and Audio marked Soon; History; New), floating toolbar over the canvas (preview
  picker, device widths, frame theme, Select, Directions, Compare, States, Copy), top-right
  Phone / Ship / Open / Export, critic score bottom-left, zoom pill bottom-right, versions
  drawer, side panel tabs. Keys: `V` select, `1` Page, `Ctrl +/−/0` zoom, arrows nudge.
- **Page mode at real widths**: desktop 1280, tablet 820, phone 390, scaled to fit; zoom
  25–400 % and pan. **Compare is side by side under one shared scale**, so both versions
  are always at equal zoom (the old slider stays as an option).
- **Move / resize** (`picker.js`): selection frame with eight handles, drag the element to
  move it, sides resize one way, corners both (Shift keeps proportions), size readout,
  arrow nudges (Shift ×10) written once after a pause. Every change is written back to the
  source through `ApplyEdit` and saved as a version.
- **Inspect tab**: exact X / Y / W / H, Reset position, Text and Fill colours, font size,
  padding, radius, Edit text, Ask Blaze.
- **Colour wheel** (`colorwheel.js`): HSV wheel, brightness, opacity, hex (3/4/6/8 digits),
  system picker, brand kit + page swatches, recent colours. Used by Inspect and the Brand
  tab, and every plain colour input in Design (restyle, swap colour, 3D parts) gets it too.
- **Brand tab**: the existing brand kit and taste, plus editing ink, paper and accents with
  the wheel. **Critic tab**: the critic's card.
- **Go** (`design_studio.go`, `internal/studio/transform.go`): `SetStudioMode` (per project),
  `DesignPalette` (brand + page colours), and every style edit is cleaned before it is
  written: whole-pixel moves and sizes with bounds, scale/rotate normalised, colours as
  lowercase hex (`rgba()` → `#rrggbbaa`). Version names say "Moved h1.hero to 10px 4px" /
  "Resized … to 320 × 80".

Checks: `go vet`, `go build` (webkit2_41), `go test ./internal/studio/` and the new
main-package tests; jsdom: steps 80/80, specs 51/51 (17 new), picker 48/48, handles 19/19
(new), nosquare 6/6, design 35/35. Handles also checked in headless Chromium.

## Phase 2: 3D and Motion
- 3D mode: the existing blaze-parts mockups (three-d-design skill) in the studio canvas:
  orbit/zoom, turntable, wireframe, explode, click a part, Inspect for colour (wheel),
  scale, turn and position, written back to the parts JSON (`ApplyPart`).
- Motion mode: a timeline drawer for a page's CSS animations / keyframes (tracks, drag to
  retime, edges to stretch, scrub, play/pause, easing), written back to the source.
- Per-mode export: 3D → PNG turntable frames / GLB where available; Motion → MP4/WebM/GIF
  through ffmpeg when present.

## Phase 3: Audio and the rest
- Audio mode: tracks with waveforms, mute/solo, play, real WAV export (Web Audio render
  from a `blaze-audio` spec the model writes), sound-logo templates.
- Notes rail button (phone + shared-link comments), share for review, quick-prompt chips
  per mode, New design picker for 3D / motion / sound, mobile-concept parity check.
