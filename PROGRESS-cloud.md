# v0.19 workstream C — Blaze Cloud model routing + Custom models

Branch `feat/v019-cloud-models`, from `fix/glass-v0.18.1` @ 9d9a246. The brief named 7a5d3fc, but that commit isn't in the local repo, and 9d9a246 is the branch tip.

## Done
- **internal/cloudai** (new, pure + tested): a port of Blaze Android `Cloud.kt` / `ThinkBudget.kt` / `EmptyReply.kt`.
  - `Task()` picks the task hint in this order:
    - vision
    - long: more than 48 000 chars of wire JSON
    - agent: the request has tools
    - code: code cue in the message
    - quick: a background call, or the phone's adaptive think budget says QUICK
    - chat
    
    Private images (Private vision) never count as vision.
  - `Retryable`, `BackoffMs` (1 s, 3 s, 7 s; a Retry-After of 1–20 s wins), the `Friendly` / `Access` lines (signed out, expired, waiting list, rejected, busy, unreachable, daily allowance), and empty-reply handling (same route without streaming + a nudge, then `blaze_avoid`, then fail).
  - `Drive()` is the `Cloud.drive` retry loop: 3 tries, trim once on too-large; a shape error gets one retry with the tool turns flattened.
  - `Gateway` sends `POST {base}/chat/completions` with `Authorization: Bearer <Blaze session token>`, `model: blaze-auto`, `blaze_task`, `blaze_avoid`, streaming SSE. It reads `X-Blaze-Provider` / `X-Blaze-Model` for the route chip ("Blaze Cloud · groq · llama…"). `Check()` calls `GET {base}/models`, which also puts the account on the waiting list.
  - Base address: `BLAZE_GATEWAY_URL` env › `gateway_url` in models.json › `https://blaze-gateway.blaze-app.workers.dev/v1`. The paths come from BlazeMobile `gateway/src/index.ts` and its README. Mobile v0.34 itself now uses the alias `https://ai.blaze-app.workers.dev/v1/`.
  - `Choice` lives in `~/.blaze/desktop/models.json`, a new desktop-only file. Keys: `mode` (cloud|custom), `fallback_to_cloud` (default off), `overrides` {chat, code, vision}. On save it keeps unknown keys, writes atomically with mode 0600. With no file yet the mode is Cloud, except for a user who already chose a fixed default model in config.json (including a BlazeRouter preset): they get Custom.
  - `Router` (llm.Backend):
    - Cloud goes to the gateway.
    - Custom uses the classic provider router over config.json + keystore, unchanged. A per-task pin means that one model, with no fallback.
    - When Custom fails, the user is told so plainly ("Your custom model couldn't answer: … Blaze Cloud wasn't asked, because using it as a backup is off"). Blaze Cloud answers only if the user switched the backup on, and the reply's note says so.
    - It never restarts under words that already streamed.
- **internal/llm** (shared core, additive only):
  - `backend.go` adds the SetBackend hook, CompleteDirect/QuickDirect, Wire, CleanErr, ParseWhole, RateHeaders and the `Plainer` interface.
  - `CompleteWith` and `Quick` check the backend first.
  - `Plain()` uses a Plainer's line.
  - With no backend set (Blaze CLI), behaviour is identical.
- **Desktop**:
  - `cloudmodels_app.go` adds the bindings `ModelsChoice`, `SetModelsMode`, `SetCloudFallback`, `SetTaskModel` and `CloudStatus`.
  - Sign-in reuses the existing Blaze account (account.go; blaze-auth; token in keychain `blaze-account:session`). That is the same blaze-auth the phone uses, so mobile's sign-in flow didn't need porting.
  - Every CompleteWith/Quick caller now routes through the backend: chat, agents, background agents, watch, pair, titles, learning, longchat.
- **Frontend**: `cloudmodels.js` + `cloudmodels.css`.
  - Settings › Models shows "How Blaze answers": Blaze Cloud (recommended) / Custom, plus an account status row (Sign in / waiting list / Check, refreshed on each visit).
  - Custom adds "Your default", per-task models (chat and code must call tools), the "Use Blaze Cloud as a backup" switch, and then the existing BlazeRouter card + classic editor.
  - Model menu: Blaze Cloud comes first and is ticked in cloud mode, then "Custom · your own models" with the existing options. Picking one of those switches to Custom.
  - Onboarding doesn't reopen to ask for a key in cloud mode. Home shows "Sign in to start" instead of "No model yet".
- **Tests** (`internal/cloudai/*_test.go`):
  - task classification
  - think budget
  - retry/backoff
  - error lines
  - empty replies
  - the Drive loop
  - fake httptest gateway: request shape, signed out, 401 / 403 waiting list / 403 rejected / 429 allowance, 5xx retries with Retry-After, unreachable, empty → nudge → avoid, /v1/models check
  - choice load/save/legacy/normalize
  - cloud vs custom routing through `llm.CompleteWith` / `llm.Quick`, with a fake gateway and a fake custom provider
  - per-task pins
  - no silent fallback; opt-in fallback
- **Shots**: a block in ci/shots.py, dark + light. The screens are models-cloud, models-picker-cloud, models-waitlisted, models-custom, models-custom-tasks, models-picker-custom and models-signedout. The mock's default mode is "custom" so the older BlazeRouter shots stay unchanged. Only the new block was run locally: it passes, radius check included.

## Shared files touched (small)
- app.go: +1 line (`a.startCloudModels()` after `a.startCloud()`).
- models.go: modelName() returns "Blaze Cloud" in cloud mode, and SelectModel() switches to Custom (+5 lines, +1 import).
- frontend/dist/app.js: the Settings nav label changes from "BlazeRouter" to "Models", and the hero "No model yet" callout is wrapped so CloudModels can replace it.
- frontend/dist/index.html: cloudmodels.css (before round.css) and cloudmodels.js (after shell.js).
- internal/llm/router.go, quick.go, plain.go: backend hook lines (additive).
- ci/mock.js, ci/shots.py: appended blocks.

## Not done / follow-ups
- **CLI follow-up**: nothing required. The CLI never sets a backend, and config.json is untouched. Optional: a Blaze CLI `--cloud` / `blaze login` that calls `llm.SetBackend(cloudai.NewRouter(...))` with the token from the same keychain entry, to get Blaze Cloud in the terminal.
- memgraph.go (memory sort: `CanSort` / `SortMemories`) and Doctor (app.go) still check `llm.Targets`, which only counts provider keys. So a keyless Blaze Cloud user sees "add a model" there. I left these alone to avoid conflicts with the memory workstream. The fix is to treat `cloudModelsOn()` as having a model.
- mind_app.go calls BlazeRouter's local server directly with llm.Chat for the one-mind review, so that call isn't routed through Blaze Cloud.
- Onboarding's Model step still offers keys. A "Sign in to Blaze" step for cloud mode would be better (onboard.js, not touched).
- Wording: the desktop lines say "Retry" where the phone says "tap Retry", and "Check your internet connection" where the phone says "data or Wi-Fi".

## How to run
```
export PATH=/workspace/work/toolchain/arm64/go/bin:$PATH   # sandbox is arm64; toolchain/go is amd64
CGO_ENABLED=1 go vet -tags webkit2_41 ./... && go build -tags webkit2_41 ./...
go test ./...
NODE_PATH=/workspace/work/toolchain/node/node_modules node ci/picker.test.js  # also steps, specs (npm i jsdom there)
CHROME=/usr/bin/chromium python3 ci/shots.py   # > 2 min
```
