# v0.17.1: the mobile design on the desktop

Source: Blaze-agent (Android) v0.24.0 messaging chat, v0.24.1 pill composer, v0.24.5 mascot, v0.24.8 no square UI, v0.24.9 rich replies, v0.25.2 solid Blaze blue. Patch-only bump (repo rule).

## Mapping
| Android | Desktop |
| --- | --- |
| `ui/Theme.kt` LightColors / DarkColors, `BlazeBrush.blue` | `style.css` `:root` / `[data-theme=dark]` tokens, `--primary #4C7DFF` |
| `BlazeShapes` xs 8 / small 12 / medium 16 / large 24 / pill | `--r-xs` / `--r-sm` / `--r-md` / `--r-lg` / `--r-pill`, applied in `round.css` |
| `ui/NoSquareTest` | `ci/nosquare.test.js` (Linux build + PR checks) |
| `phone/Mascot.kt` + `BrandMark` | `logo.svg` (generated from the same body path, gradient stops, highlight, eyes, glow; dark `#212226` circle) |
| `ui/Bubbles.kt` (positions, corners 18/6/4, gapAbove 3/16, time, MetaLine) | `bubbles.js` (`Bubbles.positions`, `time`, `regroup` via a MutationObserver on the thread), `chat.css` |
| `ReplyParts` follow-ups | `MD.splitFollowups`, `Bubbles.followups`, prompt `replyExtras()` |
| `ui/Shell.kt Composer`, `agent/ComposerHints.kt` | `buildPane` markup (`.irow`), `composerLead/Tail`, `Hints` in `composer.js` |
| `ui/RichBlocks.kt` | `md.js` (flow, rside, rgroup, deck, mermaid) + `chat.css` |

## Kept on desktop (mobile dropped them)
Chat history in the sidebar, Archive, Search over chats, the model chip/route line, `?` shortcuts, Plan / Autopilot modes, Design / Studio. Only restyled.

## Left out
Kroki (graphviz/dot/plantuml/d2 needs deflate encoding), the deck's card-flip 3D animation (a slide/tilt transition instead), per-message timestamps for chats saved before this version (messages don't store a time; only live ones show it), the BlazeRouter dashboard page itself (its own iframe page in `internal/router/web`).
