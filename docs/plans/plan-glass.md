# Desktop glass look from Blaze Mobile (v0.18.0 + v0.18.1)

Frontend only. No Go, `internal/`, `~/.blaze` or prefs changes, so Blaze CLI is unaffected.

## v0.18.0 (shipped partial)
- Glass tokens and intensities (`GlassKit.kt`) and the dark-first palette (`Theme.kt`) in `glass.css` / `style.css`.
- Motion tokens and a Reduce motion toggle (`Motion.kt`) in `motion.css` / `motion.js`.
- New `logo.svg` (orb + halo). The canvas emblem was an approximation, and the streaming drop, shimmer, thinking marks, progress bar and icons were not done. v0.18.0's notes said StreamOrb was ported; it was not.

## v0.18.1 (completes the port)
| Mobile source | Desktop file | What |
|---|---|---|
| `DuoGlow.kt` drawHaloRing, drawDuoEmblem | `duo.js` (`window.Duo`) | Exact halo + drop maths, shared clock, `<blaze-emblem>` |
| `StreamOrb.kt` drawWaterOrb, drawSplash, fall/glide | `duo.js`, `stream.js` | Drop rides the last line, falls onto blocks, rests by the newest reply |
| `DuoGlow.kt` duoShimmer | `stream.js` | Blue glow on the newest words, end sweep |
| `BlazeGlass.kt` EdgePath, EdgeParticles | `edge.js` | 40 dots on the window edge, only during a run |
| `BlazeGlass.kt` BrandProgress | `progress.js` | Shimmer bar on update + Whisper downloads, skeletons |
| `Theme.kt` shapes | `round.css` | Radii for every control; CI radius check in `ci/shots.py` |
| `Motion.kt` | `motion.js` | `BlazeMotion.sync()` + `blazemotion` event |

Light theme: accent `#18181B`, new `--on-ink` token for text on ink fills, the emblem sits on a small dark lens so the white drop shows.

## Skipped (and why)
- Halo as a second speaker (gold shimmer, travelling ring): desktop has one speaker; waits on the Halo decision (Phase 2d).
- Tilt panes and swipe gestures: touch-only.
- Rich reply blocks, inline mermaid, selection pill, rapid messages: Phase 2a/2b.
