# Blaze 1.0 status

Plan: [plan-v1.0-redesign.md](plan-v1.0-redesign.md). Each step is its own PR and version bump.

## Step 1 — BlazeRouter (PR #44, branch `blazerouter`, v0.10.0) — MERGED (PR #46, 6a283a9)
Done:
- internal/router: OpenAI-compatible gateway on 127.0.0.1:20128 (OmniRoute port), every auto/* preset, combos, LKGP, chaos, dashboard, tests
- router_app.go + tray item + models.go wiring
- frontend/dist/router.js + router.css linked: #routerBtn (icon + status dot) before #themeBtn; show("router", page) = status bar, on/off, Open in browser, iframe of RouterDashboardURL(page); presets + combos at the top of the model popover ("All BlazeRouter presets" folds the rest); /model › BlazeRouter submenu; /router
- Settings: "BlazeRouter" card replaces Models; "Providers & keys" nav item gone; old editors behind "Classic editor"; show("settings","providers") and every old add-a-key link open show("router","providers")
- Test: internal/router/carryover_test.go (old custom provider + extra keys carry over and route)
- ci/mock.js router methods; ci/shots.py router shots (real dashboard page, /api stubbed from ci/router-state.json); picker shot clicks Google Gemini by name
- THIRD_PARTY_NOTICES.md, README credits, CHANGELOG v0.10.0, docs/plans/plan-blazerouter.md, VERSION + wails.json 0.10.0
- Checks: gofmt, go vet ./internal/..., go test ./internal/..., GOOS=windows go build ., node --check, ci/*.test.js (jsdom) all pass; curl :20128/v1/models lists auto/* presets, 401 without key

## Step 2 — Redesign part A (branch `redesign-a`, v0.11.0, PR #47) — MERGED (99f6ec6)
Done: shell.js/shell.css (Chat|Projects, Exit chip, Search Ctrl+K, avatar menu, rail modes), views.js (/views picker, view:open), view_app.go open_view tool + test + prompt line, ink user bubbles, live work card counter (liveMeta), + menu composer (Voice, @ File, Plan/Autopilot, Model, Views), mode chip, ? shortcuts tooltip, shots (shell-*), CHANGELOG, plan-redesign-a.md, VERSION/wails.json 0.11.0.

## Self-knowledge (PR #49, branch `self-knowledge`, v0.12.0) — MERGED (e586e3c), released v0.12.0 (6 assets)
internal/selfknow (ABOUT YOU block + about_blaze tool), identical in Blaze-cli (PR #15, v0.10.0). Blaze Link not in build yet (linkInBuild=false in selfknow_app.go) until Step 3b.

## Step 3 — Long-chat engine part B (PR #48, branch `long-chat`, v0.13.0) — MERGED (eabc153)
Done: internal/longchat (db, compact, artifacts) + tests; longchat_app.go wired into app.go; tools search_chat/read_turn/read_artifact/update_brief (core, next to about_blaze); token meter + fresh-chat offer; project Brief editor; shots longchat-*; CHANGELOG v0.13.0; plan-longchat.md; VERSION + wails.json 0.13.0.
Checks: gofmt, vet ./internal/..., go test ./internal/..., GOOS=windows build, node --check, ci/*.test.js.

## Step 3b — Blaze Link PC side (PR #50, branch `blaze-link`, v0.14.0) — MERGED (5b9f86d)
Done: pair.go (port of PR #3, protocol = Android DesktopLink.kt, documented in plan-blaze-link.md), phone_ask tool + Env.Phone, PAIRED PHONE prompt line, linkInBuild=true + live online phone in about_blaze, Phone view (link.js/css, /views, /phone, open_view phone), qrImage fix, pair_test.go, mock + shots link-*, CHANGELOG, VERSION/wails.json 0.14.0.

## Step 4 — Student pack (PR #51, branch `student-pack`, v0.15.0) — MERGED (066940b), released v0.15.0 (6 assets)
Done: classroom_courses/classroom_coursework (on-demand scopes), schedule `once`, 6 skills (34 total, index + README), flashcards pkg + tools + Learn view, low-data mode (prompt, see_screen hidden, MB per task), tests, mock + shots student-*, CHANGELOG, plan-student-pack.md, VERSION/wails.json 0.15.0.
Manual for Luke: add Classroom scopes + enable Classroom API in Cloud project blaze-511220 (one-click Google broker).

## Signed updates (PR #23, branch `signed-updates`, v0.15.1) — MERGED (ad34ff3), released v0.15.1 (8 assets incl. SHA256SUMS + SHA256SUMS.sig; signature verified against updsig.PublicKey)
Rebased onto main after v0.15.0; ed25519-signed SHA256SUMS + SHA256SUMS.sig in the release job (ci/sign, ci/verify), internal/updsig (PublicKey A/yWqdv1xeM1tQhvGj/2uzF2bFpp6OTBrONLxkkctUA=), updater refuses unsigned updates. BLAZE_UPDATE_SIGNING_KEY secret added by Luke. Publish-once-per-VERSION guard kept.

## One mind (PR #52, branch `one-mind`, v0.16.0) — MERGED (3ba375c), released v0.16.0 (8 assets incl. SHA256SUMS + .sig)
Done: internal/mind (journal + memories with FTS5, dedupe/newest wins, pinned core + subject index, ladder, shared prompt text); mind_app.go (one-time migration of old chats + blaze.db memories, save-as-you-go after each turn, idle review on BlazeRouter auto/best-free with llm.Quick fallback, memory_search/save/update, WHAT YOU KNOW block); search_chat/read_turn over the journal; ladder in longContext; main thread "main", /clear = fresh context; Archive view (read-only); Memory graph on the new table; Settings toggle; tests; mock + shots mind-*; CHANGELOG; plan-one-mind.md (schema + prompt text for Android); VERSION/wails.json 0.16.0.
Checks: gofmt, vet ./internal/..., go test ./internal/..., go test -tags webkit2_41 . (root), GOOS=windows build, node --check, ci/*.test.js, shots.py both themes.

## Step 5 — Marketer pack (PR #53, branch `marketer-pack`, v0.17.0) — MERGED (57eb888), released v0.17.0 (8 assets incl. SHA256SUMS + .sig)
Done: skills content-calendar, product-listing, ad-lab, research-brief, whatsapp-replies, weekly-digest (40 total, index + README); phone_ask send approval via Env.Approve (never auto-approved) + test; shots marketer-*; CHANGELOG; plan-marketer-pack.md; VERSION/wails.json 0.17.0.

## Mobile design (branch `feat/mobile-design`, v0.17.1) — PR pending
Done: tokens from mobile Theme.kt + radius tokens, no `* {border-radius:0}` reset, round.css (every component rounded), chat.css + bubbles.js (messaging bubbles, grouping, meta line Copy · Quote · Retry, follow-up pills, work strip, inline confirm, jump button), pill composer + rotating hints (composer.js Hints), greeting empty state, md.js rich replies (step-flow, pictures, deck, mermaid), mascot logo.svg, replyExtras prompt + TestReplyExtras, ci/nosquare.test.js, ci/design.test.js, ci/design_shots.py (called by shots.py), CHANGELOG, plan-mobile-design.md, VERSION/wails.json 0.17.1.

## Next, in order
- Plan 1.0 steps are all in. Ideas: Android mirrors one mind (plan-one-mind.md), Android-side shared skills sync check, Classroom scopes in Cloud project blaze-511220 (manual).
