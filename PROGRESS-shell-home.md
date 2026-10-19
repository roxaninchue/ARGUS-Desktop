# v0.19 workstream A: one infinite chat + Home tab

Branch `feat/v019-shell-home` (from `fix/glass-v0.18.1` 9d9a246). Clone: /workspace/work/ws-shell.

## Done
- **Sidebar removed.** `<aside id="side">` (Chats list, New chat, per-chat rows, Projects rail, view buttons) is gone from index.html. The badge counters the views picker reads (#ver, #jobCount, #connCount, #updCount) live in a hidden `#badges` div. The B logo moved into the top bar.
- **Top bar tabs: Home | Chat | Projects** (mobile order), centred; Search (Ctrl K), router dot, avatar on the right; Exit chip for /views pages (and Design → back to the chat). shell.js: `Shell.go(tab)`, `Shell.tabOf(view)`.
- **One infinite chat** (onechat.js + onechat_app.go + internal/chatpage):
  - `OpenMainPage(turns)` opens the main conversation (same as OpenMain for the model) but sends the UI only the last 12 turns (≤400 messages, always starting at a user message).
  - `MainHistory(before, turns)` pages older turns; an IntersectionObserver on the top bar loads them as you scroll up, keeping the scroll position.
  - After the start of the main conversation, the **earlier chats (pre-one-mind archive) page in, newest first**, each under an "Earlier chat · title · date" separator (read-only, click opens it on its own). Nothing in ~/.blaze is moved or deleted.
  - `/clear` keeps working (Fresh context separator is drawn at `context_from` when it's loaded).
  - **Search**: `SearchHistory(q)` over main + earlier chats; new `history` view (Ctrl K → "Search the conversation"); a main hit jumps to the message (loads pages until it's there, flashes it). Ctrl K also lists earlier chats.
  - Projects keep their own threads: Projects tab = projects list (new `projects` view) → project page → project chat (with a "‹ Project · Project chat" crumb). The Chat tab always leaves the project and shows the one conversation (mobile ChatNav rule).
  - `openChat("main")` anywhere is routed to the paged open.
- **Home tab** (home.js/home.css + home_app.go + internal/home):
  - Greeting ("Good afternoon, Luke." — name from memory), date, "Updated … ago", quick ask (sends to Chat), Refresh, Add a card (plain words + example chips).
  - Board in category stacks (collapsed = peeking cards + "+N", tap expands, Show less), sizes S/M/L (L spans two columns), card menu: Refresh, Move up/down, Size, Change city, Ask Blaze to change it, Remove. Skeletons, Stale tag, plain error lines, action buttons (Type a city, Tell Blaze what you're into, Ask Blaze).
  - Cards ported: weather (Open-Meteo, city or IP-based location via ipwho.is), news (Google News RSS + Bing pictures, or any RSS/Atom feed), prices (CoinGecko crypto, open.er-api FX, Yahoo chart stocks), sports (TheSportsDB), GitHub repo activity, custom, "into" starter. Desktop-only: **Schedule** (sched jobs/reminders = mobile routines), **Deadlines** (reminders + memories), **Pick up where you left off** (last chat message, recent projects, recent folders).
  - Lazy refresh: due cards (>50 min, local cards always) refresh on open, max 3 at a time, again every 10 min while Home is open.
  - First open seeds from memory when confident, else starters (weather, top stories, schedule, pick up, into).
  - Storage: new file `~/.blaze/desktop/home.json` (atomic writes; a broken file is moved to `.broken`). Interests from "into" are saved as a mind memory (preference/Interests).
  - **Launch tab**: Home on first run, then the tab the user last left (Home or Chat).
- **Fix found on the way**: specialists.js created a second strip whenever the chat pane was off-page (now common: Home is the launch tab) — `ensureStrip` now checks `wrap.contains(strip)`.
- Tests: internal/home (8 tests: parser, store, feeds, weather/prices, sports/GitHub, fetcher with fake net, desktop cards, greeting/name/memories), internal/chatpage (window + search), home_app_test (locale). ci/specs.test.js updated (no sidebar). ci/shots.py: new home, home-stack, home-add, home-menu, onechat, onechat-earlier, history-search, projects-tab, project-chat, mind-onechat shots (dark + light); sidebar shots (mind-sidebar, titles-typing, titles, rename) retired. ci/mock.js: Home board + 30-turn paged main chat + OpenProject.

## Skipped mobile Home parts
- Phone-only: Most-used apps (Usage access), Calendar (phone calendar provider), location permission (desktop uses approximate IP location or a typed city), battery/SIM/calls (not cards on mobile Home anyway).
- Not ported this round: Creators (YouTube channel resolve; a YouTube channel RSS URL still works as a news feed card), interest-picker chip screen (replaced by the "into" card's text field), board backgrounds/accent customise sheet, drag-to-reorder (Move up/down instead), Home heads-up notifications (HomeNews), the glass in-app browser window, the agent's home_widget_* tools (would need internal/tools edits).
- AI chat-title animation + double-click rename lived on sidebar rows; titles still show on the project page.

## How to run
```
. /workspace/work/ws-shell-env.sh   # Go 1.26 at /workspace/work/toolchain/go
CGO_ENABLED=1 go vet -tags webkit2_41 ./... && CGO_ENABLED=1 go test -tags webkit2_41 ./...
NODE_PATH=<dir with jsdom> node ci/steps.test.js; node ci/picker.test.js; node ci/specs.test.js
CHROME=/usr/bin/chromium python3 ci/shots.py
```

## Shared files touched (small)
- index.html: aside removed, #badges, brand in #top, 2 css + 2 js includes.
- app.js: boot (OneChat.boot), #newChat/#newProject guards, refreshSide early return without a sidebar.
- shell.js / shell.css (tabs, Exit chip, search items), specialists.js (one-line strip fix).
- No changes to internal/longchat, internal/cloud, internal/router, router.js, app.go, main.go.

## CLI follow-up
None required: home.json is new and desktop-only; chats are read as before.

## Open decisions
- Launch tab: Home first run, then last of Home/Chat (brief said Home default; mobile remembers Home). Easy to force Home always in OneChat.boot.
- Opening the app no longer restores a project as the current chat (Chat tab is always the one conversation); projects are one click away in Projects.
