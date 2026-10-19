# Changelog

Release notes for Blaze Desktop, newest first. Design notes per release are in `docs/plans/`.

## v0.17.0 (marketer pack)
- **Six skills for running an online store** (40 bundled): **content-calendar** (a week of posts with visuals from the Design studio in your brand kit, each saved as a Schedule reminder), **product-listing** (supplier URL from AliExpress, Zendrop and others → title, description, tags, SEO, price and image prompts, plus a Shopify import CSV), **ad-lab** (5 ad variants scored on hook, clarity and CTA, best first), **research-brief** (a one-page competitor and pricing brief, sourced with deep-research), **whatsapp-replies** (drafted here, sent through your paired phone), **weekly-digest** (a weekly job with orders, best sellers, top posts and what to post next).
- **Sending through your phone always asks you first**: any `phone_ask` that sends a message in your name shows "Send through your phone?" with the exact text, even in Autopilot. Your phone asks again.
- Design notes: `docs/plans/plan-marketer-pack.md`.

## v0.16.0 (one mind)
- **One continuous conversation**: Chat mode is one thread with Blaze; `/clear` opens a fresh context window instead of a new chat. Projects keep their own threads.
- **Nothing lost**: every old chat is imported into a searchable journal on first launch and stays readable, read-only, in the new **Archive** view (`/views › Archive`). `search_chat` and `read_turn` reach all of it.
- **Memory is the source of truth**: a new memories table (newest wins, no duplicates, inferences marked "(inferred)"), shown in the Memory graph and kept in step with Blaze CLI. The prompt carries only a pinned core and an index of subjects; Blaze looks the rest up with `memory_search`, `memory_save`, `memory_update`.
- **Save as you go** after each turn, plus an **idle review** of older conversation on BlazeRouter's free `auto/best-free` (throttled: idle 10 min, on mains, CPU quiet). Toggle in Settings › Behaviour › Learning.
- **Context ladder**: recent turns → rolling summary at 60% → when even the summary won't fit, core + index + the last turns, and Blaze looks things up instead of guessing.
- Design notes (schema and shared prompt text for Android): `docs/plans/plan-one-mind.md`.

## v0.15.1 (signed updates)
- **Releases are signed**: every release now carries `SHA256SUMS` (sha256 of every asset) and `SHA256SUMS.sig`, an ed25519 signature made in CI by `ci/sign` from the `BLAZE_UPDATE_SIGNING_KEY` Actions secret. `ci/verify` checks a release folder by hand.
- **The updater refuses unsigned updates**: before installing, Blaze downloads both files, checks the signature against the public key built into the app (`internal/updsig`, `A/yWqdv1xeM1tQhvGj/2uzF2bFpp6OTBrONLxkkctUA=`) and the file's SHA-256. A missing or bad signature or a hash mismatch deletes the download and installs nothing. No grace period; `BLAZE_ALLOW_UNSIGNED_UPDATE=1` is for local testing only.
- `ci/sign` refuses a key whose public half isn't trusted, so a wrong secret fails the release instead of shipping files no app accepts. Key rotation steps are in the README.
- Tests: `internal/updsig`, `update_verify_test.go`.

## v0.15.0 (student pack)
- **Google Classroom**: `classroom_courses` and `classroom_coursework` read your classes and what's due (local time, points, turned in or not) through the Google connector. Classroom permission is asked only the first time you use it.
- **Six new skills** (34 bundled): **study-planner** (exam date + topics → day-by-day plan in Schedule, re-plans a missed day), **tutor** (explain, quiz, flashcards from PDFs or slides), **lecture-notes** (local Whisper → notes, summary, key terms, likely exam questions), **essay-coach** (outline, feedback, APA 7 / Harvard; won't write graded essays), **group-project** (tasks list + a WhatsApp summary), **classroom-sync** (each assignment → a reminder 24 h before).
- **Learn view** (`/learn`): flashcard decks with spaced repetition (Again / Hard / Good / Easy).
- **Low-data mode** (Settings › Behaviour › Data): text first, no screenshots, images or big downloads unless asked, short answers, and the MB each task used in its work card.
- **One-time schedules**: `once 2026-11-16 08:00` for reminders; they remove themselves after running.
- Design notes: `docs/plans/plan-student-pack.md`.

## v0.14.0 (Blaze Link: pair your phone)
- **Phone view** (`/views › Phone`, `/phone`): show a QR code, scan it in Blaze for Android › Settings › Blaze Desktop, and the phone is paired. See paired phones, which one is online, and unpair.
- **Phone → PC**: ask Blaze on the phone to do something on this PC (files, code, installs). It runs here with its own conversation per phone, and approvals pop up on the phone.
- **PC → phone**: new `phone_ask` tool for texts, WhatsApp, calls, contacts and alarms. Every send needs your yes on the phone.
- Direct HTTPS on port 7377 with a pinned certificate and a one-time code (5 minutes, single use). Same Wi-Fi or Tailscale; no server in the middle. Same protocol as Blaze Android's DesktopLink.
- **Blaze knows**: `about_blaze` and ABOUT YOU now give the real pairing steps and the live paired-phone status.
- Fix: QR codes (pairing and the Design phone preview) are drawn full size instead of tiny in a corner.
- Tests: the whole handshake, approvals on the phone, about_blaze status, QR scale. Design notes: `docs/plans/plan-blaze-link.md`.

## v0.13.0 (long chats stay fast)
- **Rolling summary**: once a chat passes 12k prompt tokens (or 60% of the model's context, if smaller), everything before the last ~12 messages folds into a state note (Goal, Decisions, Facts, Open threads, Files touched; at most ~800 tokens). The note is updated as the chat grows, never redone from scratch. A 300-turn test chat stays under ~5k input tokens a turn.
- **search_chat and read_turn**: every message is indexed (SQLite FTS5), so Blaze finds exact earlier words, even ones folded into the summary, instead of asking you to repeat.
- **Project brief**: a pinned brief per project (purpose, rules, key files, in progress; up to 2,400 characters) goes with every turn. Blaze keeps it current with `update_brief`, and you can edit it on the project page.
- **Big outputs aside**: a tool result over 4,000 characters is saved to `~/.blaze/artifacts/`. Blaze sees the first 40 and last 20 lines and reads the rest on demand with `read_artifact`.
- **Token meter**: the live card shows `in 4.2k · out 0.6k`. Near 80% of the model's context, Blaze offers **Start a fresh chat with a summary**.
- The index, summaries and briefs live in `~/.blaze/desktop/longchat.db`, separate from `blaze.db`, so Blaze CLI keeps reading your memory.
- Each chat's tools now use that chat's own id (find_tools, search, brief), even when it runs in the background.
- Tests: `internal/longchat` (300-turn budget, summary keeps a seeded fact, search finds turn 5, artifact truncation and read-back, brief cap), app tests for the brief, indexing and set-aside rules. Design notes: `docs/plans/plan-longchat.md`.

## v0.12.0 (Blaze knows itself)
- **ABOUT YOU**: every chat now starts with a short, live block telling Blaze what it is: Blaze Desktop and its version, made by Luke (Lackson Kapapa, Blazeinchue), the three apps (Desktop, CLI, Android) and how they connect, BlazeRouter on/off, Blaze Link status, the Blaze account and this PC (OS, desktop/Wayland, CPU, RAM). About 300 tokens.
- **A new rule**: questions about Blaze itself, its apps or connecting them ("how do I connect my phone?") are answered from ABOUT YOU and `about_blaze`, never by asking which apps you are building.
- **`about_blaze` tool** (always on, optional `section`): the full report, read live: identity, apps, version and build, Blaze account (blaze-auth, signed in or not, last key sync), Blaze Link (how it works, pairing steps, paired phones from `~/.blaze/pair`), BlazeRouter (address, status, all `auto/*` presets), views and slash commands read from this build's own frontend registries, the real `~/.blaze` layout, connected accounts, MCP servers, skills, memory count and database, and the PC. Same name as Blaze Android's `SelfKnowledge.kt`; Blaze CLI gets the same.
- Honest about gaps: Blaze Link's PC side is not in this build yet, so Blaze says so and points to what works today (the Blaze account and Design › Phone) instead of inventing steps.
- Tests: `internal/selfknow` (brief size, sections, link status, devices, layout, desktop env, registries from the real frontend), the system prompt carries ABOUT YOU and the rule, the tool output; a mock shot of "how do I connect my phone?". Design notes: `docs/plans/plan-self-knowledge.md`.

## v0.11.0 (a calmer shell)
- **Top bar**: the B logo goes back to the chat; **Chat | Projects** in the middle; **Search** (Ctrl+K) finds chats, projects, views and commands; the BlazeRouter dot; and an **avatar menu** with Account, Settings, BlazeRouter dashboard, Model, Theme, All views, Check for updates and Sign out.
- **Rail**: in Chat it is just your recent chats (it folds away below 1100px); in Projects it lists projects with a *Working* chip on the busy one.
- **Nothing deleted, everything in `/views`**: Memory, Jobs, Schedule, Accounts, Doctor, Settings, Design, BlazeRouter, Skills and Usage left the sidebar for one picker (filter, arrow keys, Enter, badges). `/memory`, `/jobs`, `/design`, `/router` and the rest still work, and Ctrl+K lists them too.
- **Blaze switches views itself**: the new `open_view` tool opens Design without asking for visual work, and asks first for other views. While a view is open the top bar shows `DESIGN · Exit` (Esc works too).
- **Chat**: your messages sit on an ink bubble; Blaze answers as plain text in a 760px column. Each turn has one live card, `Working · 4 steps · 1m 12s`, that you can open while it runs and that collapses to `Worked · 13 steps · 3m 11s`.
- **Composer**: `+` on the left (Image, File, Folder, Voice, @ File, Plan mode, Autopilot, Model, Views, Commands), Send on the right. A small chip shows the mode only when it isn't Build · Auto, and the shortcuts moved into a `?` tooltip.
- Tests: Go test for `open_view` (asks, refusals, unknown views); shots for the shell, avatar menu, /views, + menu, mode chip, Search, Design with Exit, Projects and the live card. Design notes: `docs/plans/plan-redesign-a.md`.

## v0.10.0 (BlazeRouter)
- **BlazeRouter**: a local OpenAI-compatible gateway (`127.0.0.1:20128/v1`, next free port if taken) starts with Blaze and routes each request across your free providers, falling back on rate limits, outages and empty quota. It is a native Go port of OmniRoute (MIT): every `auto/*` preset (auto/best-free, auto/best-coding, auto/best-fast, auto/best-reasoning, auto/best-vision, auto/chaos, auto/lkgp and any `auto/<category>:<tier>`), combos with priority, round-robin, quota-aware and cheapest strategies, last-known-good preference and chaos fan-out to five providers. Anthropic and Gemini request formats are translated, so Claude Code and Gemini clients work too.
- **Router button** in the top bar (icon + green/amber/red status dot) and in the tray. It opens the BlazeRouter dashboard inside Blaze (Home, Providers, Combos, Usage & logs, Quota, Settings), with an on/off switch and **Open in browser**. `/router` opens it too.
- **Model picker**: BlazeRouter presets and your combos come first, then **All BlazeRouter presets**, then each provider directly. `/model` has a BlazeRouter submenu.
- **Settings › BlazeRouter** replaces Models; *Providers & keys* moved into the dashboard. The old editors are still there behind **Classic editor**, and every old "add a key" link opens BlazeRouter › Providers.
- **Nothing to re-enter**: keys stay in Blaze's shared store (`~/.blaze`, system keychain), so Blaze CLI keeps working; your old Auto chain becomes a "My chain" combo, and custom providers and extra keys carry over (tested).
- Credits: `THIRD_PARTY_NOTICES.md`. Design notes: `docs/plans/plan-blazerouter.md`.

## v0.9.5 (parallel tools, eight new skills, skills that combine)
- **Tools run in parallel**: when the model asks for several reads in one reply (file reads, listings and searches, web searches, page fetches, recall, skills, job logs), Blaze runs them at the same time, up to 6 at once, in the main chat and in background agents and specialists. Anything that changes things still runs one by one, in the model's order, and results go back in the order they were asked, so history and the steps list read the same. The prompt now tells the model to batch independent calls. Code: `internal/llm/batch.go`, `internal/tools/parallel.go`.
- **Paper sketches**: the new **paper-sketch** skill draws doodles and hand-drawn illustrations that look like pencil, pen or marker on real paper (notebook, sketchbook, sticky note, graph paper, kraft). It builds a self-contained SVG with paper grain, pencil wobble and hatching, one group per part so Design can pick and edit each, and exports to PNG. The starter is `skills/paper-sketch/assets/sketch.svg`.
- **Seven more skills for the requests Blaze used to fumble**: **diagrams** (flowcharts, mind maps, org, sequence, architecture, ER, timelines via Mermaid or SVG), **pixel-art** (sprites, tiles and sheets on an exact grid, exported with nearest-neighbour scaling), **motion-graphics** (CSS, SVG or canvas animation exported to MP4, GIF or WebM), **floor-plan** (to-scale 2D plans with dimensions and clearances, PDF at a stated scale), **print-design** (posters, cards, invitations and stickers with bleed and safe areas, print-ready PDF), **browser-game** (a playable one-file game with touch controls, sound and high scores) and **music-audio** (melodies, beats, jingles and SFX as MIDI, WAV or MP3, plus ffmpeg audio edits).
- **Skills combine**: each skill names the skills it pairs with, and Blaze loads every skill a task touches. When a combination keeps coming up, it saves that as a new skill holding only the glue between them. skill-creator explains how.
- 28 bundled skills (was 20).

## v0.9.4 (3D mockups in Design, Blaze does the work itself)
- **3D starts in Design**: any request for a 3D model or 3D design (a product, furniture, a printable part, a building, a character) starts as a Three.js mockup built with the new bundled **three-d-design** skill. Each part lives in one `blaze-parts` JSON block in the page, so the scene and its numbers never drift apart. The page lists the parts with their sizes, every code block is annotated, and **Download .glb** exports the model.
- **Pick and edit parts**: in Design, click a part in the 3D view (orbit drags are left alone; Shift-click adds more parts). **Adjust** changes colour, scale and turn live, and **Apply** writes them to that part's JSON entry as a one-line diff with Undo. Arrow keys nudge the part by the mockup's step (Shift ×10), and Page Up/Down raise or lower it, saved the same way. Change, Ask and Hold to talk send the part's JSON to the model, which edits only that part.
- **Mockups open themselves**: a turn that produces a 3D mockup opens Design unless *Design tab* is set to Never.
- **Then the real program**: when the request names one (Blender, FreeCAD, OpenSCAD, Fusion 360, SketchUp, Unity, Godot…), Blaze makes the mockup first and builds there next, from the same parts: a `bpy` or FreeCAD script, a `.scad` file, STEP, STL or .glb.
- **Lead mode removed**: Blaze does your tasks itself again. Background agents and specialists start only when you ask for them, or for Design's Directions. The *Blaze does tasks itself* setting is gone.
- Tests: Go tests for the part write-back (`internal/studio/parts_test.go`); the jsdom spec checks that Settings no longer offers lead mode.

## v0.9.3 (switch chats while Blaze works)
- **Chats run on their own**: you can open another chat, start a new one or switch projects while Blaze is answering. The answer keeps going in the background, in its own folder and project, and saves into its own chat. The sidebar shows a pulsing dot on chats that are still working. Open one again and you pick up the live answer. When it finishes in a chat you left, you get a notification.
- Each chat has its own turn runner, so a chat never waits on another. Stop and new messages only affect the chat on screen. Background agents' results go to their own chat even while it's running in the background.
- Tests: background turns save into their own chat, Stop in another chat leaves them alone, and the switch, live dot and pick-up work in the UI.

## v0.9.2 (Blaze stays free: specialists do the work, Stop is instant)
- **Lead mode (on by default)**: Blaze does no work itself. Every task, small or large, goes to a specialist that Blaze briefs, monitors (check_specialist), steers (message_specialist) and stops (cancel_specialist), then reports back. Blaze is always free to chat. New `direct` specialists work on the real computer with no copy, for questions, system checks, installs, apps, research and commands; `build` specialists still work in their own copy for code you review. Design edits are still done directly. Turn it off in Settings › Behaviour › "Blaze does tasks itself".
- **No more queue**: a message sent while Blaze is answering interrupts that answer (its text so far is kept) and gets answered at once.
- **Stop is instant**: Stop now kills the running command and everything it started (process group on macOS/Linux, `taskkill /T` on Windows), not only the model call. Before, a long command such as a disk scan kept running until it finished. Specialists' commands stop the same way when they're cancelled.
- CI: a Windows test that failed while removing its temp folder now retries cleanup; tests for killing a running command, the no-queue path and the setting.

## v0.9.1 (Continue)
- **Continue** when a reply is cut short: it hit the model's output limit (`finish_reason: length`), the stream broke off, you pressed Stop, an error ended it part-way, or it ran out of steps. One click and the model carries on from exactly where it stopped, without a recap or repeated text, and resumes mid-sentence or mid-code-block.
- The text a reply streamed before Stop or an error is now kept in the chat, so the model can see where it left off (it used to be lost).
- The Continue message shows as a small "Continue" bubble. Sending anything else clears old Continue buttons.
- CI: Go test for finish reasons, jsdom checks for every Continue path.

## v0.9.0 (Design anything: artboards and export)
- **New design** in the Design toolbar (and on the empty stage): a social post (1080×1350), story (1080×1920), slide (1920×1080), poster (A4 or A3), phone screen (390×844), email (600 wide), icon or logo (1024×1024) or banner (1500×500). No project folder needed: each design is its own folder under `~/.blaze/designs/`, with versions, Directions, brand kit and the chat working as they do for web pages. Your brand kit carries over.
- **Artboards**: a page marked `<meta name="blaze-artboard" content="…">` is drawn at its real size, scaled to fit the stage (the zoom shows under it), and the model gets that format's rules: one idea per slide, print bleed and 300-dpi safe type for posters, table-free-safe 600px email, a mark that still reads at 32px for icons.
- **Export** (shows on artboards): PNG at 2× or PDF, rendered with the Chrome, Edge, Chromium or Brave already on the computer, saved in the design's `exports/` folder named by version, and shown in Explorer or Finder.
- CI: Go tests for formats, templates and export (export runs where a browser exists), jsdom checks for New design, the scaled artboard and Export.

## v0.8.3 (Private vision)
- **Settings › Notifications › Screen vision › Private vision**: pictures never leave the computer. Attachments, see_screen screenshots and Copy window images are read on this computer, with the OCR built into the OS (Windows.Media.Ocr, macOS Vision, Tesseract on Linux when installed) plus a size, brightness and main-colours readout, and models get only that text. A model running on this computer (Ollama, or a custom service on localhost) may still get the picture.
- No model download and no cloud fallback: with no OCR available, the text says so instead of sending the image.
- Shared with Blaze CLI as `private_vision` in `~/.blaze/config.json`. Doctor shows the engine in use.
- New `internal/ocr` package, tested on every OS in CI (Linux CI installs tesseract).

## v0.8.2 (Specialists strip tucks away when done)
- When every specialist in a round has finished and nothing awaits review, the strip shows the outcome for 6 seconds, then hides; an open row keeps it up while you read. **Clear** in its header hides a finished round at once.
- A new round shows only its own specialists, not the finished ones from before; a reopened chat shows only specialists that still need something.
- CI: jsdom test for the strip (`ci/specs.test.js`).

## v0.8.1 (MCP servers shared with Blaze CLI)

- **One MCP list for both apps**: servers live in `~/.blaze/mcp.json` in the standard shape (`{"mcpServers": {name: {command, args, env} | {url, headers}}}`, plus `"type": "sse"` and `"disabled": true`). Blaze CLI reads and writes the same file; Desktop picks up edits made there (or by hand) on the next look, and keeps keys and fields it doesn't own. Servers Desktop had before move into the file once.
- **Transports**: local commands over stdio (with `env`), Streamable HTTP and the older HTTP+SSE (with `headers`). `${VAR}` in env values and headers reads your environment, so the file can name a secret without holding it; a token typed into Desktop stays in the system keychain.
- **Tools as `mcp__<server>__<tool>`**, the name Blaze CLI and other MCP hosts use.
- **Approvals**: every MCP tool asks first until you set it to Always allow (a server's own read-only hint no longer skips the card); Off hides it.
- **Settings › MCP servers**: add (remote URL with token and headers, or a local command with environment), turn on or off, check all, open a server for its tools and per-tool approval.

## v0.8.0 (the Design studio)
- **Edits that write back.** Double-click text to retype it, drag the MOVE tab or the corner to move or resize, use the
  style controls or **Swap colour › Everywhere**: Blaze changes the real file (HTML, Vue, Svelte, Astro templates and
  JSX/TSX `style={{…}}`) as a small diff, saves a version, and shows the diff with **Undo**. Anything too involved to
  edit by hand (a style built from variables, text spread across tags) goes to the model as before.
- **Directions as live branches.** Directions sends 3 specialists out on their own git worktrees (or copies), carrying
  your uncommitted page with them, and shows the three side by side as they work. **Pick** one, or **Mix**: "header
  from B, colours from C".
- **Brand kit and taste memory.** Pull colours, fonts, radius and logo from a URL or a logo image; every design prompt
  follows it. Blaze also learns the fonts, colours, radii and spacing you keep and the ones you undo
  (`~/.blaze/desktop/taste.json`, Brand › Forget).
- **Copy any window.** Rebuild the window behind Blaze, or a dropped or pasted screenshot, as code in your stack.
- **The critic.** Each new version is audited at 390, 820 and 1280 px (contrast, alt text, labels, tap targets,
  sideways scroll, DOM size, image weight, load time) and scored; below 85 Blaze fixes it once on its own (turn off in
  the score card).
- **Talk while you point.** Select an element, hold **Hold to talk**, say what to change.
- **States board.** Default, hover, focus, pressed, empty, loading, error and dark mode in one grid.
- **Visual history.** A branching timeline (restoring an old version starts a new branch), a before/after slider in
  Compare, and **Restore this element from vNN**.
- **Phone preview.** A QR code opens the live preview on your phone over Wi-Fi behind a secret link, with hot reload and
  long-press comments that show up in Blaze.
- **One-click ship.** Cloudflare Pages (wrangler via npx) or GitHub Pages (a `gh-pages` push), free tiers, with an
  optional comment widget so friends can pin comments on the shared page (blaze-auth Worker 0.7.0:
  `/v1/comments.js`, `/v1/comments/<site>`).
- Fixes the flaky first-run SQLite lock (`TestConcurrentFirstRunInit`) that stopped the v0.7.7 Linux build.

## v0.7.7 (keys shared with Blaze CLI and your phone)
- **One key store with Blaze CLI** (`internal/keystore`, the same file in both repos): OS keychain service `blaze-cli`
  (`<provider>`, `<provider>#extra` for extra keys) or, with no keychain, `~/.blaze/keys.json` (0600) read fresh on
  every use, so the two apps never overwrite each other's keys. Keys older versions kept under `"keys"` in
  `config.json` are copied over once and kept for older Blaze versions; `config.json` saves keep every field they
  don't own.
- **Keys follow your Blaze account** to Blaze on your phone and other computers (Settings › Account › Model keys,
  **Sync now**): after sign-in, whenever you add or remove a key, and at start (at most hourly). The apps encrypt the
  keys with AES-256-GCM before upload; the service stores only the blob. Newest change per provider wins; keys saved
  before syncing existed are merged, not replaced. Connector tokens and your session never sync.
- **blaze-auth Worker 0.6.0**: `GET/POST /v1/keys` (one encrypted blob per account, revision check, deleted with the
  account). Until it's deployed, keys just stay shared on this computer.

## v0.7.6 (a tidier Preview picker)
- **Current, Other, All.** The Design tab's Preview picker opens on **Current**: the pages this task changed, plus the one you're previewing (or, before anything changed, the pages next to it). **Other** holds the rest of the folder and **All** shows everything, each with a count; search works inside the tab.
- **Collapses when you click away.** Clicking anywhere outside the picker closes it, including inside the live preview itself (the preview tells Blaze about the press, and focus moving into the frame counts too).

## v0.7.0 (skill library and proactive memory)
- **Bundled skill library**: 19 skills ship inside Blaze (`skills/` in this repo) and are seeded into `~/.blaze/skills` on start: web design in the Team of Keys and classic Blaze styles, landing pages, slide decks, documents, deep research with sources, code review, debugging, git/PR, tests, refactoring, data analysis, shell/sysadmin (Windows, macOS, Linux, Arch), email drafting, planning, images/assets, web audits, API integrations and a skill-creator guide. Upgrades are version-aware and never overwrite a skill you edited; turn any skill off, delete it or restore it in **Settings › Skills**.
- **Learns as you go**: Blaze now saves lasting facts and preferences on its own, replaces memories that changed instead of piling up duplicates, forgets on request, and writes or improves a skill when a workflow repeats or you teach it a method. A cheap end-of-turn check (skipped for small talk, capped by your spend limit) catches what the turn missed. A small mark under the answer shows what it learned. **Settings › Behaviour › Learning** turns it off.

## v0.6.0 (a lead with specialists)
1. **Big tasks split across specialists.** For work with independent parts (a frontend and a backend change, tests for several packages, research on separate questions) the main chat acts as the lead: it briefs specialists with `dispatch_specialist`, they run in parallel (3 at a time by default, Settings › Behaviour › *Background agents & specialists*, extras queue), and each result comes back to the lead automatically. Small or step-by-step tasks it still does itself.
2. **Each specialist gets its own copy.** A Build specialist works in its own **git worktree** on a `blaze/<name>` branch (or a copy of the folder when it isn't a repo, skipping `node_modules` and other caches), so specialists never collide and nothing reaches your folder until the lead has reviewed the diff (`check_specialist` with `diff: true`) and called `apply_specialist`. Applied work lands as uncommitted changes for you to commit; a copy never overwrites a file you changed meanwhile.
3. **Never more than the lead.** Specialists inherit Plan/Build (in Plan they only scout and plan), Auto/Autopilot approvals, the step budget and the daily spend cap. The lead can check, steer (`message_specialist`) or stop (`cancel_specialist`) any of them.
4. **A compact Specialists strip** above the composer: one row per specialist with a mini particle orb, its task, mode, state and time, Stop, and on expand its plan, log, report and the files it changed (with Apply / Discard if you'd rather decide yourself). See `docs/plans/plan-v0.6.0.md`.

## v0.5.0 (Blaze accounts and one-click connectors)
1. **Connect in one click.** Google (Gmail, Calendar, Drive), GitHub, Spotify, Notion and Slack connect with a browser sign-in through `blaze-auth`, a small Cloudflare Worker on the free tier (`cloud/`), so you no longer need your own developer app. "Use your own key/developer app instead" stays under *Advanced* on each connector page; providers the Worker isn't set up for yet say "One-click Connect is coming soon" and keep manual setup open.
2. **Blaze account (optional).** Settings › Account or `/account`: sign in with Google, GitHub or email and password, sign out, or delete the account. A card at the top of Accounts shows who's signed in.
3. **Private by design.** Client secrets live only on the Worker; provider tokens are never stored there beyond a 5-minute one-time code. The desktop side uses a loopback callback with PKCE and opens your system browser. Override the server with `BLAZE_CLOUD_URL` or `cloud_url` in `~/.blaze/desktop/cloud.json`. See `cloud/README.md` and `docs/plans/plan-v0.5.0.md`.

## v0.4.9 (notifications, screen vision and composer modes, with background agents)
1. **Native notifications.** Blaze works out once per computer how to notify you (Windows toast, macOS Notification Center, or D-Bus on Linux, which covers GNOME, KDE, and mako/dunst/swaync on Hyprland or sway), caches it in `~/.blaze/notify.json`, and falls back to `notify-send`, `osascript` or a PowerShell toast. It tells you when a long answer or background job finishes while Blaze is hidden, and when a scheduled job runs (failures always; successes wait out quiet hours). The model gets a `notify` tool with just a title and a body. Settings › Notifications shows the method, a test button and recent notifications; clicking one opens Blaze on its chat.
2. **Proactive suggestions (opt-in).** A frugal watcher glances at the front window at most every 10 minutes, only while you're at the computer, never in full screen or quiet hours (22:00–08:00 by default), at most 3 an hour and 8 a day, and backs off when you ignore it. It sleeps between checks instead of polling and speaks only when it can genuinely help (an error, a failing build, a confusing form).
3. **Screen vision.** `read_screen` reads the front window's accessibility tree (UI Automation, AX, AT-SPI) as compact text; `see_screen` falls back to a downscaled screenshot (GDI, screencapture, grim on Hyprland/sway, the desktop portal on GNOME/KDE, X11) for a vision model. Reads are cached by window and picture hash, password fields are never read and are blurred in screenshots, and a bar at the top shows whenever the screen is being read, with a one-click off switch. Doctor checks notifications, screen reading, screenshots and suggestions, with Fix buttons for the macOS permissions.
4. **Attach anything.** The composer's **+** attaches images, files and folders; you can also drag them onto the window or paste a screenshot. Each shows as a chip you can remove. Images go to models that can see (Gemini first); with no vision model Blaze says so and sends the path instead. Text files travel in the message, folders as a short tree.
5. **Talk instead of typing.** The mic (or **Ctrl+Shift+M**) turns speech into text with Whisper on your own PC: free, offline after a one-time download you agree to (whisper.cpp ~10 MB plus a 75–466 MB model, base by default, with progress). Windows and Linux download the official whisper.cpp build; macOS uses Homebrew. If it can't run, Blaze says why.
6. **Two modes instead of AUTO-APPROVE**, shown in the composer and remembered:
   - **Work mode** (**Ctrl+Tab**): **Plan** reads and scouts only and ends with a plan. It's enforced in Go: tools that can change things are not offered and are refused if called. **Build** has every tool.
   - **Approval** (**Shift+Tab**): **Auto** asks before risky commands, deletes and installs. **Autopilot** works until done: when Blaze asks you something, it's told you're away and decides by best practice. Disk-wiping commands stay blocked, and a run stops at 150 steps, 45 minutes or your daily spend cap.

## v0.4.1
**Answer cards.** Blaze answers in a sentence or four, and long detail (tables, step lists, reports, long code) folds into a card above it: clipped with a fade, one click (or the chevron) to read it all, a copy button for its Markdown. Short cards just show in full. Works while streaming, in saved chats and in the orb. Turn it off in Settings › Behaviour › *Fold long answers into cards*.

## v0.4.0
1. **Lives in the background.** A tray icon (menu-bar icon on macOS) with Quick ask, Open Blaze, what Blaze is doing, and Quit. Closing the window hides Blaze instead of quitting (Settings › Behaviour), and it can start hidden when you sign in.
2. **Quick ask from anywhere.** Press **Ctrl+Shift+Space** (Option+Space on a Mac) for the orb: a small always-on-top Blaze in the corner. Expand opens the full window, Esc hides it. macOS asks for Accessibility permission once; on Wayland, bind a system shortcut to `blaze-desktop --orb`. On Hyprland (e.g. Omarchy) and sway Blaze floats the orb itself via `hyprctl` / `swaymsg`; on other tiling window managers add a rule that floats the app id `blaze`.
3. **One Blaze at a time.** Opening it again brings the running one forward; `--orb` opens quick ask, `--background` starts hidden.
4. **Calmer answers.** Instead of raw commands, each answer shows one plain-English status line ("Reading index.html", "Searching the web for …") that becomes "Worked for 12s"; click it to see every step. Power users can turn the full list back on.
5. Windows gets Blaze's own title bar. Doctor checks the tray, shortcut and start-at-login.

## v0.3.0
1. **Fixed: Design turns breaking on Gemini.** Tool calls streamed by Gemini (whole calls per chunk, repeated index 0, arguments as an object) were glued together or lost their name, then saved into the chat, so every later request got `HTTP 400: Request contains an invalid argument`. The stream reader now splits calls correctly, keeps Gemini's thought signatures, a gatekeeper repairs or drops malformed calls before they are run or saved, and the history is cleaned before every request (with one flattened retry if a provider still refuses it).
2. **Pick any provider and model.** The model chip and `/model` switch provider *and* model, or Auto. Settings › Models has a clear default (Auto or one model), the Auto chain with live health (speed, resting, failing) that you can reorder, a "use my free stack" preset, fallback, fastest-first and a daily spend cap.
3. **Models from the mobile app**: a built-in catalog per provider (tools, vision, context size, free tier), the provider's live model list, a "check my plan" probe, OpenRouter's best free tool models picked daily, several keys per provider (when one runs out the next carries on), a token meter that skips a model before it hits its rate limit, plan & quota per provider, usage and cost per model, and plain-English errors. New providers: Cerebras and any OpenAI-compatible service. One-click OpenRouter sign-in.
4. **Keyless is never used for actions.** Pollinations can't call tools, so it is never the default, never in the chain, and never used for Work actions or Design; at most it answers plain chat when nothing else is set up, and only if you allow it.
5. **Redesigned app**: calmer sidebar with icons, segmented Work/Design tabs, a model switcher popover, compact one-line tool steps (repeated errors fold into ×N, older steps fold away), an error card with Retry / Switch model / Details, a proper Design toolbar (labelled preview picker, icon toggles for device and theme, versions as a slim collapsible strip, resizable preview/chat split), and Settings with a left sub-nav. Light and dark both tuned.

## v0.2.1
1. **In-app updates**: Blaze checks GitHub for a newer release when it opens (Settings › Updates, or `/update`) and installs it in place: the setup.exe or portable exe on Windows, Blaze.app from the .dmg on macOS, and the .deb, Arch package or tar.gz on Linux, then reopens. 
2. **Fix**: no more blank window on launch. The UI waits for the app to finish starting instead of failing on its first call.

## v0.2.0
1. **Command menu**: type `/` (or press `+`) for a floating list above the chat box: new, model, project, folder, design, retry, stop, skills, theme and more. `@` attaches project files.
2. **Projects**: a folder with its own chats and standing instructions; project page with git status and recent files.
3. **Work / Design tabs**: visual work (HTML/CSS/SVG or a running dev server) opens in Design, by asking first, always or never.
4. **Live preview** at desktop, tablet and phone widths; every change is a numbered version you can view, compare and restore; "Directions" asks for 3 options.
5. **Point to edit**: Select mode, click (Shift-click for more) and Change, Remove, Restyle, Replace text or Ask; live colour/size/spacing tweaks, then Apply writes them into the code.
6. Blaze names the real model it runs on.

Earlier (v0.1.0): first-run PC detection and setup, streaming chat with tool steps, confirm cards, jobs, schedule, doctor, settings.

Earlier (v0.1.0): first-run PC detection and setup, streaming chat with tool steps, confirm cards, jobs, schedule, doctor, settings.
