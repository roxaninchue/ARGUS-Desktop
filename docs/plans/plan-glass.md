# Desktop v0.18.0 plan: glass look from Blaze Mobile

## Ported from the Kotlin reference
- `GlassKit.kt`: frosted glass tokens, glass intensities, and the dark/light glass recipe.
- `Theme.kt`: dark-first palette, radii tokens, typography, and the monochrome accent scheme.
- `BlazeGlass.kt`: glass surfaces, progress bars, and the edge-particle pattern.
- `DuoGlow.kt`: the gold halo, orb emblem, shimmer and blur treatment.
- `StreamOrb.kt`: drop motion, splash effect, and the streaming reply travel behaviour.
- `Motion.kt`: motion timings, reduced motion, scale press state, shimmer and enter/exit transitions.

## This phase focuses on the desktop frontend only
- CSS tokens and glass shell restyle in `frontend/dist/style.css` and `frontend/dist/glass.css`.
- Motion timing hooks and reduced-motion utilities in `frontend/dist/motion.css` and `frontend/dist/motion.js`.
- Live emblem drawing in `frontend/dist/duo.js` for the sidebar and onboarding surfaces.
- Settings appearance toggle for motion reduction without any Go or prefs-key change.

## What was intentionally skipped
- Rich reply blocks (charts, forms, compare cards, choice cards, mermaid diagrams) are not part of this phase.
- Halo persona, token ledger, rapid-message polish and Go/backend work stay out of scope.
- The app keeps the existing event wiring and class names; the styling pass is additive and non-invasive.
