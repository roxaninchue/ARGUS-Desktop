// Fake backend so CI can screenshot the UI in a normal browser.
// Init scripts run in every frame: inside BlazeRouter's dashboard iframe, only stub its /api.
if (/\/internal\/router\/web\//.test(location.href)) {
  (() => {
    const RS = window.__routerState || {};
    window.fetch = async (u, o) => {
      const path = String(u).replace(/^.*\/api\//, "").split("?")[0];
      const body = path in RS ? RS[path] : { ok: true };
      return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
    };
  })();
} else
(() => {
  const os = window.__os || "linux";
  const info = { os, os_name: "Arch Linux (Omarchy)", arch: "amd64", cpu: "AMD Ryzen 7 7840U", cores: 16, ram_gb: 32, gpu: ["AMD Radeon 780M"], shell: "bash", shell_path: "/usr/bin/bash", pkg_managers: ["pacman"], tools: { git: "2.51", python: "3.13", node: "v24.1", go: "1.25", docker: "28.1" }, home: "/home/luke", disk_free_gb: 312 };
  const state = { version: "v0.7.2", setup_done: true, info, missing: [], installs: { node: "sudo pacman -S --noconfirm nodejs npm" }, providers: [
    { id: "gemini", name: "Google Gemini", short: "Google Gemini", key_hint: "AIza…", key_page: "https://aistudio.google.com/apikey", quota_page: "https://aistudio.google.com/", free: true, has_key: true, keys: 2, ready: true, in_chain: true, model: "gemini-3.8-flash" },
    { id: "groq", name: "Groq", short: "Groq", key_hint: "gsk_…", key_page: "https://console.groq.com/keys", free: true, has_key: true, keys: 1, ready: true, in_chain: true, model: "openai/gpt-oss-120b" },
    { id: "cerebras", name: "Cerebras", short: "Cerebras", key_hint: "csk-…", free: true, model: "gpt-oss-120b" },
    { id: "openrouter", name: "OpenRouter", short: "OpenRouter", key_hint: "sk-or-…", free: true },
    { id: "deepseek", name: "DeepSeek", short: "DeepSeek", key_hint: "sk-…", free: false, model: "deepseek-chat" },
    { id: "mistral", name: "Mistral", short: "Mistral", key_hint: "…", key_page: "https://console.mistral.ai/api-keys", free: true },
    { id: "nvidia", name: "NVIDIA NIM", short: "NVIDIA NIM", key_hint: "nvapi-…", free: true },
    { id: "moonshot", name: "Kimi (Moonshot)", short: "Kimi", key_hint: "sk-…", free: false },
    { id: "openai", name: "ChatGPT (OpenAI)", short: "ChatGPT", key_hint: "sk-…", free: false },
    { id: "anthropic", name: "Claude (Anthropic)", short: "Claude", key_hint: "sk-ant-…", free: false },
    { id: "xai", name: "Grok (xAI)", short: "Grok", key_hint: "xai-…", free: false },
    { id: "ollama", name: "Ollama (local)", short: "Ollama", free: true, keyless: true },
    { id: "custom", name: "Custom (OpenAI-compatible)", short: "Custom", key_hint: "Your key" },
    { id: "pollinations", name: "Pollinations (keyless)", short: "Pollinations", free: true, keyless: true, model: "openai-fast" }],
    chain: [{ provider: "gemini", model: "gemini-3.8-flash" }, { provider: "groq", model: "openai/gpt-oss-120b" }],
    selection: { auto: true, provider: "auto", label: "Auto (free routing)" },
    routing: { fallback: true, fastest_first: true, keyless: false, daily_cap: 0, today_cost: 0 },
    prefs: { theme: window.__theme || "light", folder: "/home/luke/code/homeglow", design_mode: "ask", project: "p1", show_steps: false, fold_cards: true, autostart: false, hotkey: "" },
    modes: { work_mode: "build", approval: "auto" }, vision_model: window.__novision ? "" : "Google Gemini · gemini-3.8-flash",
    desktop: { os, tray: true, tray_note: "In the system tray", background: true, autostart: false, autostart_ok: false, hotkey: "ctrl+shift+space", hotkey_label: "Ctrl+Shift+Space", hotkey_ok: true,
      hotkey_note: "Press Ctrl+Shift+Space anywhere for quick ask", needs_access: false, wayland: false, frameless: os === "windows", orb: false,
      presets: [{ id: "ctrl+shift+space", label: "Ctrl+Shift+Space" }, { id: "alt+space", label: "Alt+Space" }, { id: "ctrl+alt+space", label: "Ctrl+Alt+Space" }, { id: "ctrl+shift+b", label: "Ctrl+Shift+B" }] }, model: "Auto · Google Gemini · gemini-3.8-flash",
    project: { id: "p1", name: "HomeGlow", folder: "/home/luke/code/homeglow", instructions: "Shopify theme. Use pnpm. Keep it fast on phones." }, yolo: false, persona: "", keyless: true, skills: ["deploy-site: build and push the HomeGlow theme"], memory: "- Uses pnpm, not npm (2026-10-09)" };
  // Accounts / connectors (PR #6 data, v0.4.x)
  const T = (name, group, risk, desc, mode, grant) => ({ name, group, risk, desc, mode: mode || (risk === "read" ? "always" : "ask"), default: risk === "read" ? "always" : "ask", needs_grant: !!grant });
  const gfields = (set) => [{ key: "client_id", label: "Client ID", hint: "…apps.googleusercontent.com", value: set ? "1234-abcd.apps.googleusercontent.com" : "", set }, { key: "client_secret", label: "Client secret", hint: "GOCSPX-…", secret: true, set }];
  const connView = { loopback: "http://127.0.0.1:43117/callback", scheme: "blaze://connector/oauth_callback", connectors: [
    { id: "google", name: "Google", blurb: "Gmail, Calendar, Drive and Contacts", auth: "oauth", status: "connected", account: "kapapaluckson75@gmail.com", enabled: true, since: "10 Oct 2026", one_click: true, cloud: "ready", via: "broker",
      granted: ["read your email", "send email as you", "see your calendar events", "see and download your Drive files", "see your contacts"], groups: ["gmail", "calendar", "drive", "contacts"],
      fields: gfields(true), steps: ["Open Google Cloud Console and create a project (any name).", "APIs & Services › Library: enable Gmail API, Google Calendar API, Google Drive API and People API.", "OAuth consent screen: External, then add your own Google address as a test user.", "Credentials › Create credentials › OAuth client ID › Application type: Desktop app.", "Paste the client ID and client secret here, then Connect."],
      console: "https://console.cloud.google.com/apis/credentials", console_name: "Google Cloud Console", redirect_uri: "http://127.0.0.1:43117/callback", custom_scheme: false,
      tools: [T("gmail_search", "gmail", "read", "Search the user's Gmail with Gmail search syntax."), T("gmail_read", "gmail", "read", "Read one Gmail message by id."), T("gmail_draft", "gmail", "action", "Save an email as a draft in Gmail (not sent).", "ask", true), T("gmail_send", "gmail", "write", "Send an email from the user's Gmail. The user approves it first."),
        T("gcal_events", "calendar", "read", "List events on the primary Google Calendar."), T("gcal_create", "calendar", "write", "Create an event on the primary Google Calendar.", "always", true), T("drive_search", "drive", "read", "Search Google Drive by name or contents."), T("drive_read", "drive", "read", "Read a Drive file's text by id."), T("gcontacts_search", "contacts", "read", "Search Google Contacts.")] },
    { id: "github", name: "GitHub", blurb: "Repos, issues, pull requests and notifications", auth: "device", status: "expired", account: "@lacksonkapapahue", enabled: true, since: "2 Oct 2026", one_click: true, cloud: "ready", via: "", groups: ["github"],
      fields: [{ key: "client_id", label: "Client ID", hint: "Ov23li…", value: "Ov23liAbCdEf", set: true }], steps: ["github.com › Settings › Developer settings › OAuth Apps › New OAuth App.", "Tick Enable Device Flow and save.", "Paste the Client ID here, then Connect and type the code Blaze shows."], console: "https://github.com/settings/developers", console_name: "GitHub Developer settings",
      tools: [T("github_my_repos", "github", "read", "List the user's own repositories."), T("github_issues", "github", "read", "List issues in a repository."), T("github_read", "github", "read", "Read an issue or pull request."), T("github_create_issue", "github", "write", "Open an issue. The user approves it first."), T("github_comment", "github", "write", "Comment on an issue or PR.")] },
    { id: "spotify", name: "Spotify", blurb: "Now playing, play, pause, skip and search", auth: "oauth", status: "needs_setup", enabled: true, groups: ["spotify"], one_click: false, cloud: "soon", custom_scheme: true, redirect: "", redirect_uri: "http://127.0.0.1:43117/callback",
      fields: [{ key: "client_id", label: "Client ID", hint: "32 hex characters", value: "", set: false }], steps: ["developer.spotify.com › Dashboard › Create app. Tick Web API.", "Redirect URIs: add http://127.0.0.1:43117/callback.", "Save, then paste the Client ID here and Connect."], console: "https://developer.spotify.com/dashboard", console_name: "Spotify Developer Dashboard",
      tools: [T("spotify_now_playing", "spotify", "read", "What's playing right now."), T("spotify_play", "spotify", "action", "Play the top result or resume."), T("spotify_control", "spotify", "action", "Pause, next or previous.")] },
    { id: "notion", name: "Notion", blurb: "Search, read and add to your pages", auth: "token", status: "connected", account: "Blaze · Luke's workspace", enabled: false, one_click: true, cloud: "ready", via: "broker", since: "11 Oct 2026", groups: ["notion"], fields: [{ key: "token", label: "Internal integration secret", hint: "ntn_…", secret: true, set: true }], steps: ["notion.so/my-integrations › New integration."], console: "https://www.notion.so/my-integrations", console_name: "Notion integrations",
      tools: [T("notion_search", "notion", "read", "Search shared pages."), T("notion_read", "notion", "read", "Read a page."), T("notion_append", "notion", "write", "Add text to a page.")] },
    { id: "slack", name: "Slack", blurb: "Read channels and post messages", auth: "token", status: "not_connected", enabled: true, groups: ["slack"], one_click: true, cloud: "ready", fields: [{ key: "token", label: "User OAuth token", hint: "xoxp-…", secret: true, set: false }], steps: ["api.slack.com/apps › Create New App › From scratch.", "Add user token scopes and install.", "Paste the User OAuth Token here."], console: "https://api.slack.com/apps", console_name: "Slack apps",
      tools: [T("slack_channels", "slack", "read", "List channels."), T("slack_read", "slack", "read", "Read a channel."), T("slack_post", "slack", "write", "Post a message.")] }],
    mcp: [{ id: "linear", name: "Linear", transport: "http", url: "https://mcp.linear.app/mcp", has_token: true, enabled: true, status: "ready", checked: "10 Oct 13:02",
        tools: [T("list_issues", "mcp:linear", "read", "List issues assigned to you."), T("create_issue", "mcp:linear", "action", "Create an issue."), T("delete_comment", "mcp:linear", "write", "Delete a comment.")] },
      { id: "files", name: "Files", transport: "stdio", command: "npx", args: ["-y", "@modelcontextprotocol/server-filesystem", "~/Documents"], enabled: true, status: "error", error: "couldn't start npx: executable file not found in $PATH", tools: [] }] };
  state.connected = 2;
  const conv = { id: "c1", title: "Fix the failing test", folder: "/home/luke/code/homeglow", messages: [
    { role: "user", content: "Run the tests and fix whatever fails." },
    { role: "assistant", content: "", tool_calls: [{ id: "t1", function: { name: "run_shell", arguments: "{\"command\":\"pnpm test\"}" } }] },
    { role: "tool", tool_call_id: "t1", content: "exit 1 · cwd /home/luke/code/homeglow\nFAIL src/price.test.ts\n  expected 19.99, got 19.989999" },
    { role: "assistant", content: "", tool_calls: [{ id: "t2", function: { name: "file_edit", arguments: "{\"path\":\"src/price.ts\"}" } }, { id: "t3", function: { name: "run_shell", arguments: "{\"command\":\"pnpm test\"}" } }] },
    { role: "tool", tool_call_id: "t2", content: "Edited /home/luke/code/homeglow/src/price.ts (1 replacement)." },
    { role: "tool", tool_call_id: "t3", content: "exit 0 · cwd /home/luke/code/homeglow\n42 passed" },
    { role: "assistant", content: "All **42 tests pass** now.\n\n1. `price.ts` added floats directly, so `19.99` came out as `19.989999`.\n2. I round to cents with `Math.round(x * 100) / 100` before returning.\n\n```ts\nexport const total = (items: Item[]) =>\n  Math.round(items.reduce((s, i) => s + i.price * i.qty, 0) * 100) / 100;\n```\n\nWant me to commit it?" }] };
  // v0.4.1: a long answer the model put in a card (the 1.0 roadmap table).
  const roadmap = ["| Phase | Done | Already built | Still missing |", "| --- | --- | --- | --- |",
    "| 0 Foundation | ~70% | Wails app, Blaze look, setup wizard, model router, installers for every OS, in-app updates | Tray/background mode, global hotkey, orb overlay |",
    "| 1 Hands | ~30% | Files, shell, run_code, git, web search, fetch, HTTP, jobs | Screen reading, mouse/keyboard, browser driving, verify loop |",
    "| 2 Memory | ~25% | remember/forget, write_skill/use_skill, projects | Structured memory, Show me once |",
    "| 3 Blaze Link | ~50% | QR pairing, pinned HTTPS, ask_pc and phone_ask (phone side shipped) | Merging PR #3, an always-on phone inbox, a relay beyond Wi-Fi/Tailscale, OTP/WhatsApp tools |",
    "| 4 Autonomy | ~20% | OS-level schedules, background jobs | Event triggers, multi-hour missions, Ember report |",
    "| 5 Trust | ~30% | Confirm cards, auto-approve toggle, daily spend cap | Per-app tiers, undo timeline, kill switch |",
    "| 6 Ship | ~50% | CI for win/mac/linux/arch, releases, screenshots | Code signing (SmartScreen warning), crash reports, real-PC testing, benchmark |"].join("\n");
  window.__roadmap = roadmap;
  window.__cardReply = "Blaze Desktop is roughly **a third of the way to 1.0**. The biggest gap is Phase 1: without screen reading and mouse control, Blaze can't drive your other apps yet.\n\n:::card Blaze Desktop v0.4.0 vs the 1.0 roadmap\n" + roadmap + "\n:::\n\nWant me to start on Phase 1?";
  window.__cardConv = { id: "c5", title: "Where are we on the roadmap?", folder: "/home/luke/code/blaze-desktop", messages: [
    { role: "user", content: "How far is Blaze Desktop from 1.0? Give me the roadmap table." },
    { role: "assistant", content: window.__cardReply }] };
  // The fallback: a long reply without card markers still folds (the steps into "Details").
  window.__autoReply = "Docker is installed and running. Postgres 17 is up on **localhost:5432**.\n\n## How I set it up\n\n" +
    ["Pulled `postgres:17-alpine` (82 MB).", "Created the volume `pgdata` so data survives restarts.", "Started the container `blaze-pg` with `POSTGRES_PASSWORD` from your `.env`.", "Mapped port 5432 to localhost only, not your whole network.", "Waited for `pg_isready` to answer (2.1s).", "Created the database `homeglow` and the user `app`.", "Ran your migrations in `db/migrations` (14 files).", "Seeded 40 test products from `db/seed.sql`.", "Added `DATABASE_URL` to `.env.local`.", "Checked a query: `select count(*) from products` → 40."].map((x, i) => `${i + 1}. ${x}`).join("\n") +
    "\n\n```bash\ndocker run -d --name blaze-pg -p 127.0.0.1:5432:5432 \\\n  -v pgdata:/var/lib/postgresql/data \\\n  -e POSTGRES_PASSWORD=$POSTGRES_PASSWORD postgres:17-alpine\n```\n\nWant me to add it to `docker-compose.yml` too?";

  // v0.4.6: plain-words steps. A turn whose raw commands must never reach the live line.
  window.__stepsTurn = [
    { name: "run_shell", args: { command: "cat ~/Projects/blaze/package.json; echo ===; ls ~/Projects/blaze" }, result: "exit 0 · cwd /home/luke/Projects/blaze\n{\n  \"name\": \"blaze\",\n  \"scripts\": { \"build\": \"vite build\", \"test\": \"vitest run\" }\n}\n===\nREADME.md  index.html  package.json  src  vite.config.ts" },
    { name: "file_search", args: { pattern: "sourcemap" }, result: "vite.config.ts:12:    sourcemap: true," },
    { name: "web_search", args: { query: "vite production sourcemap slow build" }, result: "5 results\n1. Build Options | Vite: build.sourcemap…" },
    { name: "file_edit", args: { path: "/home/luke/Projects/blaze/vite.config.ts" }, result: "Edited /home/luke/Projects/blaze/vite.config.ts (1 replacement)." },
    { name: "run_shell", args: { command: "cd ~/Projects/blaze && pnpm test 2>&1 | tail -20" }, result: "exit 1 · cwd /home/luke/Projects/blaze\nFAIL src/price.test.ts > total rounds to cents\n  expected 19.99, got 19.989999" },
    { name: "run_shell", args: { command: "pnpm build 2>&1 | tail -5" }, result: "exit 0 · cwd /home/luke/Projects/blaze\nvite v6.2.0 building for production...\n✓ 412 modules transformed.\n✓ built in 9.8s" }];

  // v0.5: Memory graph. ~60 memories in 6 categories, plus skills and projects; links are
  // inferred here the same cheap way Go does it (shared rare words, same chat, project mentions).
  const memCats = {
    "Preferences": ["Prefers short, direct answers with the code first", "Likes dark mode in every app and editor", "Wants British spelling in docs and UI copy", "Prefers square edges and no pill buttons in designs", "Hates emoji in commit messages", "Likes one blue accent, ink and paper for Blaze designs", "Wants confirm cards before anything deletes files", "Prefers pnpm scripts over long shell one-liners", "Likes answers that end with one clear next step", "Prefers Inter and Inter Tight fonts for Blaze UI"],
    "Setup & tools": ["PC runs Arch Linux (Omarchy) with Hyprland", "Uses pnpm, not npm", "Default shell is bash at /usr/bin/bash", "Ryzen 7 7840U laptop with 32 GB RAM", "Go 1.25 installed at /usr/bin/go", "Docker 28 runs Postgres for HomeGlow", "Neovim is the main editor, VS Code for Shopify liquid", "Ollama runs gemma locally for keyless tests", "Tailscale links the phone and the PC for Blaze Link", "Hyprland ignores window size, so the orb needs a float rule", "Node v24 via pacman", "Git signs commits with an SSH key"],
    "Projects & code": ["HomeGlow theme lives in ~/code/homeglow", "Blaze Desktop repo is lacksonkapapahue/Blaze-dekstop", "HomeGlow tests run with pnpm test (vitest)", "Blaze Desktop builds with Wails v2 and vanilla JS", "Blaze CI builds windows, macos, linux and arch", "HomeGlow deploys with shopify theme push", "Blaze.ai web app is in ~/code/blaze-ai", "Blaze Desktop shots run in CI with Playwright", "Round prices to cents in HomeGlow price.ts", "Blaze releases bump VERSION and add a docs plan", "HomeGlow hero section uses a warm palette"],
    "Work & business": ["HomeGlow is a Shopify dropshipping store for home lighting", "Sources HomeGlow products from Zendrop", "Targets 35% margin on HomeGlow lamps", "Runs HomeGlow ads on TikTok first, then Meta", "Freelances on Upwork and Fiverr for web work", "Budget for ads is about 50 USD a week", "Blaze.ai should launch with a free tier", "Wants HomeGlow orders to ship in under 10 days", "Tracks HomeGlow sales in a Google Sheet", "Prices in USD for HomeGlow customers"],
    "Personal": ["Lives in Lusaka, Zambia (CAT, UTC+2)", "Goes by Luke; full name Lackson Kapapa", "Born in 2008, still studying", "Works on projects late at night", "Supports football on weekends", "Studies for exams in November", "Listens to music while coding", "Wakes around 7 on school days"],
    "People": ["Brother helps test Blaze on his Windows PC", "Cousin designs logos for HomeGlow", "Friend Chanda runs a phone shop and wants a website", "Mentor reviews the Blaze.ai pitch", "Teacher asked for a class project site", "Co-founder idea: a friend who does TikTok marketing", "Mum's laptop needs Blaze set up for school work", "Brother's PC has an Nvidia GPU for local models", "Chanda prefers WhatsApp for updates"],
  };
  const memList = [];
  Object.entries(memCats).forEach(([cat, items], ci) => items.forEach((t, k) => memList.push({ t, cat })));
  const day0 = Date.parse("2026-09-01T09:00:00");
  const memNodes = memList.map((m, i) => {
    const span = 40 * 86400e3, frac = ((i * 37) % memList.length) / memList.length;
    return { id: "m" + i, kind: "memory", label: m.t.replace(/\s*\(.*?\)/g, "").split(/[,;]/)[0].split(" ").slice(0, 5).join(" ") + (m.t.split(" ").length > 5 ? "…" : ""), text: m.t, cat: m.cat, created: Math.round(day0 + frac * span + (i % 5) * 3600e3), ai: true,
      chat: i % 9 === 0 ? "c1" : i % 11 === 0 ? "c2" : "", chat_title: i % 9 === 0 ? "Fix the failing test" : i % 11 === 0 ? "Set up a Postgres container" : "",
      project: /homeglow/i.test(m.t) ? "p1" : /blaze/i.test(m.t) ? "p2" : "", project_name: /homeglow/i.test(m.t) ? "HomeGlow" : /blaze/i.test(m.t) ? "Blaze.ai" : "" };
  });
  memNodes[memNodes.length - 1].created = Date.parse("2026-10-10T19:40:00");
  // v0.5.1: memories live in blaze.db; a few are pinned (sent with every chat) and each counts its uses.
  memNodes.forEach((n, i) => { n.uses = (i * 7) % 23; if (i % 13 === 4) n.pinned = true; });
  const skillNodes = [["deploy-site", "Build and push the HomeGlow theme with shopify theme push"], ["pg-docker", "Start Postgres in Docker for local apps"], ["blaze-release", "Bump VERSION, write the docs plan, merge when CI is green"], ["ad-report", "Summarise the week's TikTok and Meta ads spend for HomeGlow"], ["hyprland-float", "Make a window float on Hyprland with a windowrule"]]
    .map(([n, d], i) => ({ id: "skill:" + n, kind: "skill", label: n, text: d, cat: "Skills", created: Date.parse("2026-09-12T10:00:00") + i * 6 * 86400e3, path: "/home/luke/.blaze/skills/" + n + "/SKILL.md" }));
  const projNodes = [{ id: "project:p1", kind: "project", label: "HomeGlow", text: "Shopify theme. Use pnpm. Keep it fast on phones.", cat: "Projects", created: Date.parse("2026-08-30T12:00:00"), project: "p1", project_name: "HomeGlow", path: "/home/luke/code/homeglow" },
    { id: "project:p2", kind: "project", label: "Blaze.ai", text: "Next.js app. Free tier first.", cat: "Projects", created: Date.parse("2026-08-31T12:00:00"), project: "p2", project_name: "Blaze.ai", path: "/home/luke/code/blaze-ai" }];
  const allMem = memNodes.concat(skillNodes, projNodes);
  const catNames = [...new Set(allMem.map(n => n.cat))];
  const stopW = new Set("the and for are but not you all any can had was one our out has how its may new now see two way who did get got with that this from they them then than when what will your into over also just only very more most some such like want wants uses use used every first runs needs about usr bin home code luke".split(" "));
  const tk = s => [...new Set(s.toLowerCase().split(/[^a-z0-9.+#-]+/).map(w => w.replace(/^[.-]+|[.-]+$/g, "")).filter(w => w.length > 2 && !stopW.has(w)).map(w => w.length > 4 && w.endsWith("s") && !w.endsWith("ss") ? w.slice(0, -1) : w))];
  const toks = allMem.map(n => n.kind === "project" ? [] : tk(n.label + " " + n.text));
  const df = {}; toks.forEach((ts, i) => ts.forEach(t => (df[t] = df[t] || []).push(i)));
  const pairs = {}, N = allMem.length;
  Object.entries(df).forEach(([t, ids]) => { if (ids.length < 2 || ids.length > 9) return; const w = Math.log((N + 1) / ids.length); for (let x = 0; x < ids.length; x++) for (let y = x + 1; y < ids.length; y++) { const k = ids[x] + "," + ids[y]; const p = pairs[k] = pairs[k] || { w: 0, why: t, bw: 0 }; p.w += w; if (w > p.bw) { p.bw = w; p.why = t; } } });
  const deg = new Array(N).fill(0), memEdges = [];
  Object.entries(pairs).filter(([, p]) => p.w >= 2).sort((a, b) => b[1].w - a[1].w).forEach(([k, p]) => { const [a, b] = k.split(",").map(Number); if (deg[a] >= 5 && deg[b] >= 5) return; deg[a]++; deg[b]++; memEdges.push({ a, b, w: +p.w.toFixed(2), why: "both mention “" + p.why + "”" }); });
  allMem.forEach((n, i) => projNodes.forEach((p, pi) => { if (n.kind !== "project" && n.project === p.project) memEdges.push({ a: i, b: memNodes.length + skillNodes.length + pi, w: 1.5, why: "saved in " + p.label }); }));
  window.__memGraph = { nodes: allMem.map(n => ({ ...n, cat: catNames.indexOf(n.cat) })), edges: memEdges, cats: catNames.map(c => ({ name: c, count: allMem.filter(n => n.cat === c).length })), ai_sort: true, can_sort: true, sorting: false, unsorted: 0 };
  // v0.5 composer: attachments, voice and modes.
  const thumb = (a, b) => "data:image/svg+xml;base64," + btoa(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48"><rect width="48" height="48" fill="${a}"/><rect x="8" y="22" width="32" height="18" fill="${b}"/><rect x="30" y="8" width="8" height="8" fill="#fff"/></svg>`);
  const att = {
    "/home/luke/Pictures/hero-mobile.png": { kind: "image", path: "/home/luke/Pictures/hero-mobile.png", name: "hero-mobile.png", size: 412000, mime: "image/png", thumb: thumb("#1F4BD8", "#F4C95D") },
    "/home/luke/Downloads/error-trace.jpg": { kind: "image", path: "/home/luke/Downloads/error-trace.jpg", name: "error-trace.jpg", size: 2310000, mime: "image/jpeg", thumb: thumb("#26262A", "#E05A47") },
    "/home/luke/code/homeglow/package.json": { kind: "file", path: "/home/luke/code/homeglow/package.json", name: "package.json", size: 1840 },
    "/home/luke/Documents/brief-v2.pdf": { kind: "file", path: "/home/luke/Documents/brief-v2.pdf", name: "brief-v2.pdf", size: 884000 },
    "/home/luke/code/homeglow/sections": { kind: "folder", path: "/home/luke/code/homeglow/sections", name: "sections", count: 23 },
  };
  window.__att = att;
  const models = [{ name: "tiny", file: "ggml-tiny.bin", mb: 75, note: "fastest, rougher" }, { name: "base", file: "ggml-base.bin", mb: 142, note: "good on any PC (default)" }, { name: "small", file: "ggml-small.bin", mb: 466, note: "most accurate, slower on older PCs" }];
  window.__stt = { ready: false, engine: "", engine_how: "download", engine_mb: 10, model: "base", model_mb: 142, have_model: false, models, lang: "auto", native: "" };
  window.__rv = { running: true, level: "green", note: "Running · 3 providers ready", url: "http://127.0.0.1:20128", port: 20128, ready: 3, models: 41, version: "v0.10.0", key: "br-local-5f2c9a77e01d4b6a", dashboard: "http://127.0.0.1:20128/dashboard", enabled: true, autostart: true, in_use: false };
  const P = (g, m, d, cur) => ({ group: g, provider: "blazerouter", model: m, name: "BlazeRouter · " + m, desc: d, current: !!cur, tools: true });
  window.__rpicks = [P("blazerouter", "auto/best-free", "Best free model for the job, ranked by quality, speed and quota left"), P("blazerouter", "auto", "Your chain, then every provider with a key"),
    P("blazerouter", "auto/best-coding", "Strongest free coder that can call tools"), P("blazerouter", "auto/best-fast", "Lowest latency first"), P("blazerouter", "auto/best-reasoning", "Thinking models first"),
    P("blazerouter", "auto/best-vision", "Models that understand images"), P("blazerouter", "auto/chaos", "Asks five providers at once; the first good answer wins"),
    P("preset", "auto/lkgp", "Last known good provider first"), P("preset", "auto/coding:pro", "Coding, pro tier"), P("preset", "auto/writing:fast", "Writing, fast tier"),
    { ...P("combo", "my-chain", "Your combo · priority"), name: "BlazeRouter · My chain" }];
  const A = {
    SendWith: async () => "ok", Modes: async () => state.modes,
    SetWorkMode: async m => (state.modes = { ...state.modes, work_mode: m === "plan" ? "plan" : "build" }),
    SetApprovalMode: async m => (state.modes = { ...state.modes, approval: m === "autopilot" ? "autopilot" : "auto" }),
    AttachPaths: async ps => ps.map(p => att[p]).filter(Boolean),
    AttachPick: async k => Object.values(att).filter(a => a.kind === k),
    AttachPasted: async () => ({ kind: "image", path: "/home/luke/.blaze/desktop/pasted/20261010-pasted.png", name: "Pasted image.png", size: 96000, thumb: thumb("#111110", "#6B8CFF") }),
    STTStatus: async () => window.__stt, SetSTT: async m => (window.__stt = { ...window.__stt, model: m || window.__stt.model }), STTSetup: async () => "ok",
    Transcribe: async () => ({ text: "Make the hero image smaller on phones and keep the headline on one line.", secs: 1.8 }),
    RecStart: async () => "", RecStop: async () => ({ text: "" }),
    State: async () => state, Conversations: async () => [{ id: "c1", title: "Float Rounding In Price Tests", updated: "Oct 9", source: "auto" }, { id: "c2", title: "Set up a Postgres container", updated: "Oct 8" }, { id: "c3", title: "Why is my fan so loud?", updated: "Oct 7" }, { id: "c6", title: "HomeGlow launch checklist", updated: "Oct 6", source: "manual" }],
    DefaultPersonality: async () => "PERSONALITY\nYou are Blaze. Be warm, sharp, and one step ahead: an expert who happens to be easy to talk to.\n- Answer first. Lead with the result.\n- Take a side.\n- Be specific.",
    SetPersonality: async () => "",
    RenameConversation: async (id, t) => t || "Set up a Postgres container",
    Connectors: async () => connView, Onboarding: async () => ({ step: 0, skipped: false, done: true }), OnboardNotifyTest: async () => "",
    AddProvider: async (id) => ({ ok: true, model: id === "groq" ? "llama-3.3-70b-versatile" : "gemini-3.8-flash", where: "your system keychain" }),
    OpenConversation: async () => conv, Jobs: async () => "Job 1 [running 4m12s] pnpm dev\n   ready on http://localhost:5173", JobLog: async () => "", Schedules: async () => [{ name: "backup-notes", when: "daily 07:00", command: "rsync -a ~/notes /mnt/backup/" }],
    Projects: async () => [{ id: "p1", name: "HomeGlow", folder: "/home/luke/code/homeglow" }, { id: "p2", name: "Blaze.ai", folder: "/home/luke/code/blaze-ai" }],
    ProjectBrief: async () => window.__brief || "Purpose: HomeGlow Shopify theme, fast on phones.\nRules: pnpm only; never push to main.\nKey files: sections/hero.liquid, assets/theme.css.\nIn progress: blue hero, new product cards.",
    SetProjectBrief: async (p, b) => { window.__brief = b; return ""; },
    FreshChat: async () => "c1",
    ProjectDetails: async () => ({ project: state.project, branch: "main", status: " M sections/hero.liquid\n?? design/option-b.html", recent: [{ path: "design/option-b.html", when: "Oct 9 21:20" }, { path: "sections/hero.liquid", when: "Oct 9 21:02" }, { path: "assets/theme.css", when: "Oct 9 20:40" }], chats: [{ id: "c1", title: "Fix the failing test", updated: "Oct 9" }, { id: "c4", title: "New hero section", updated: "Oct 9" }] }),
    Models: async () => [{ provider: "blazerouter", name: "BlazeRouter", model: "auto/best-free and every auto/* preset", tools: true, ready: true }, { provider: "auto", name: "Auto (free routing)", model: "fastest healthy model in your chain", tools: true, current: true, ready: true }, { provider: "gemini", name: "Google Gemini", model: "gemini-3.8-flash", tools: true, ready: true }, { provider: "groq", name: "Groq", model: "openai/gpt-oss-120b", tools: true, ready: true }],
    ModelOptions: async p => ({ provider: p, fetched: "Oct 10 11:20", models: p === "groq"
      ? [{ id: "llama-3.3-70b-versatile", name: "Llama 3.3 70B", tools: "yes", context: 131072, free: true, known: true }, { id: "openai/gpt-oss-120b", name: "GPT-OSS 120B", tools: "yes", context: 131072, free: true, known: true, live: true }, { id: "qwen/qwen3-32b", name: "Qwen3 32B", tools: "yes", context: 131072, free: true, known: true }]
      : [{ id: "gemini-3.8-flash", name: "Gemini 3.8 Flash", tools: "yes", vision: true, context: 1048576, free: true, known: true, live: true, note: "Blaze's default" }, { id: "gemini-2.5-pro", name: "Gemini 2.5 Pro", tools: "yes", vision: true, context: 1048576, free: true, known: true, live: true }, { id: "gemini-2.5-flash-lite", name: "Gemini 2.5 Flash-Lite", tools: "yes", vision: true, context: 1048576, free: true, known: true }, { id: "gemma-3-27b-it", name: "Gemma 3 27B", tools: "no", context: 131072, free: true, live: true }] }),
    DetectModels: async p => ({ provider: p, plan: "Free tier", models: [{ id: "gemini-3.8-flash", tools: "yes", vision: true, context: 1048576, free: true }] }),
    SelectModel: async (p, m) => { if (p === "blazerouter") { window.__rv.in_use = true; return `BlazeRouter · ${m}`; } return p === "auto" ? "Auto · Google Gemini · gemini-3.8-flash" : `${p} · ${m}`; },
    // BlazeRouter (v0.10.0): the dashboard iframe is the real dashboard page with its /api stubbed.
    // Student pack (v0.15.0): flashcards in the Learn view, low-data mode.
    // v0.16 one mind: chats from before one mind, read-only in the Archive view.
    ArchivedConversations: async () => [{ id: "c1", title: "Float Rounding In Price Tests", updated: "Oct 9" }, { id: "c2", title: "Set up a Postgres container", updated: "Oct 8" }, { id: "c3", title: "Why is my fan so loud?", updated: "Oct 7" }, { id: "c6", title: "HomeGlow launch checklist", updated: "Oct 6" }],
    ReadArchived: async id => ({ id, title: "Set up a Postgres container", messages: [{ role: "user", content: "Set up a Postgres 16 container for HomeGlow, data in ~/pg" }, { role: "assistant", content: "Done. **postgres:16** runs as `homeglow-db` on port 5432 with its data in `~/pg`. The password is in your keyring under *homeglow-db*." }, { role: "user", content: "Make it start with the PC" }, { role: "assistant", content: "Set `--restart unless-stopped`, so Docker brings it back after a reboot." }] }),
    SetMindReview: async () => "",
    // v0.19 topic-aware chat: the chip above the composer.
    ChatTopics: async () => ({ current: "Pizza", id: 1, topics: [{ id: 1, title: "Pizza", turns: 4, active: true }, { id: 2, title: "Model rocket", turns: 3, active: false }, { id: 3, title: "Tax return", turns: 6, active: false }] }),
    TopicMemoryOn: async () => true, SetTopicMemory: async () => {},
    FlashDecks: async () => [{ name: "NET103", cards: 24, due: 7, new: 4 }, { name: "CIT102", cards: 18, due: 3, new: 0 }, { name: "COM102", cards: 9, due: 0, new: 0 }],
    FlashDue: async d => [{ id: "f1", deck: d, front: "Which subnet mask gives 30 usable hosts?", back: "255.255.255.224 (/27): 32 addresses minus network and broadcast." }, { id: "f2", deck: d, front: "Default port for HTTPS?", back: "443" }],
    FlashReview: async () => "",
    SetLowData: async () => "",
    // Blaze Link (v0.14.0): the Phone view.
    PairStatus: async () => ({ devices: [{ id: "d1", name: "Galaxy A12", paired_at: "12 Oct 2026", connected: true }], addresses: ["192.168.1.24", "100.88.12.5"], port: 7377, error: "" }),
    StartPairing: async () => ({ qr: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAcgAAAHICAAAAADvyiU2AAAJJElEQVR4nOydwY4kKA4Fu1bz/788e9m8IFn2M+S2FBVxGWUnCUyFfLAM5p9//wiB//ztDcgbFAlBkRAUCUGREBQJQZEQFAlBkRAUCUGREBQJQZEQFAlBkRAUCUGREBQJQZEQFAlBkRAUCUGREBQJQZEQFAlBkRAUCUGREBQJQZEQ/ukG/Cwnru5dnvP9G67T3ef8aebt9jXd93Rf3X6mdP/fRiQERUJQJARFQlAkBEVCUCSENo/8MO3Hc+ZJad50rlP9fpofdnT7S9d5lQ+n8xmREBQJQZEQFAlBkRAUCUGREMZ55Icqr6nyorR+WK1zfj/NN8/x53rVPNPvu3U60r9nhREJQZEQFAlBkRAUCUGREBQJIc4jX9HlY3+O76vzoWm+tT232v3+1fgtRiQERUJQJARFQlAkBEVCUCSEr+eR6b3Dqg7Y1fnO76fna8/1qt93+WtXx/w2RiQERUJQJARFQlAkBEVCUCSEOI+8rcd1+Vl6bjQ9Fzo9R1ut15HWSV/lm0YkBEVCUCQERUJQJARFQlAkhHEeue0zWuVV3XnV6vfTz93635p/uv7271lhREJQJARFQlAkBEVCUCQERUJo88jbelman23zwW79kzQf7Jje3/zWeVcjEoIiISgSgiIhKBKCIiEoEsL4/chtPvhnOP5c7/xdtZ/pPLfcvl/SzZv2hT0xIiEoEoIiISgSgiIhKBKCIiHE9cg0X6zmqf79ti447cdzrjedb5oPn6Tna9O+tEYkBEVCUCQERUJQJARFQlAkhGf3I9Pvt+dU03W373yk/WHT+Su2fxcjEoIiISgSgiIhKBKCIiEoEsL4XGvHq3Oc3fxdn9bpemm9L/0+PW9724fHiISgSAiKhKBICIqEoEgIioQwrkdWeU56vnOaL92eI62Y5rFpPXD6DmY1vsL7kb8MRUJQJARFQlAkBEVCUCSEny5Puc13tudC07x0mmem9yf/FPO//r4a3+3rgxEJQZEQFAlBkRAUCUGREBQJYVyP/DDNO9P+O9s88Pb9jlfvfFTfn+tMv08xIiEoEoIiISgSgiIhKBKCIiGM+7VO++Rs7xlOf7fNQ9N9pu9xfOu87RQjEoIiISgSgiIhKBKCIiEoEkJ8PzLtT5rma7f9T7d1xel+0npplzffnuv9YERCUCQERUJQJARFQlAkBEVCWPdrTfud3vZFTe9pdn16uvHTfd3WIW/7tH4wIiEoEoIiISgSgiIhKBKCIiFc1yOndbXufOht39dz3urzdPy2b1C3zvTcbYoRCUGREBQJQZEQFAlBkRAUCWGcR3b5WfouyPTe5f/rnuLrd0te3/fsMCIhKBKCIiEoEoIiISgSgiIhxH120vf1K7bviWzPt56/3/b7uSU93zsdb0RCUCQERUJQJARFQlAkBEVCiN/9uD3XWfH6XOi0Drqt/237q27vZ3YYkRAUCUGREBQJQZEQFAlBkRDiPPJD945j+vuTM6/a1kG78du+NtXvu/rpdL9pPmlEQlAkBEVCUCQERUJQJARFQvhJ628VXV+a7X3HtB/sq/1M5+32Pd3POS7tw2NEQlAkBEVCUCQERUJQJARFQojf/UjriNP5p+dST6b1yld5aDU+rZ9u+/BUGJEQFAlBkRAUCUGREBQJQZEQ4j470/HpfGleerufad64va+Y3iO9fUfSiISgSAiKhKBICIqEoEgIioQQ1yO7fK+bZ9pnZvvOyHQ/0/z4tp9Oen80fUfzgxEJQZEQFAlBkRAUCUGREBQJYXw/spzgf/+9vWf5qu73eh/duNu+sem4CiMSgiIhKBKCIiEoEoIiISgSwvX9yPQ9/dt+O+f81eftfcqK6T3N7nfTvNB+rb8URUJQJARFQlAkBEVCUCSEth65Pdd5W8ebrr/Nc7t1O9Jztq/fxzwxIiEoEoIiISgSgiIhKBKCIiFc99nZvtMxrQd29by0f+yrfrIdr94BmWJEQlAkBEVCUCQERUJQJARFQmjzyA9VXjQ9t5rmR+l7HWm/1dt3H6ffT7nNK41ICIqEoEgIioSgSAiKhKBICOP7kdv7iNNzpyfT/G9bZ6xI+6xu7z2m53U7jEgIioSgSAiKhKBICIqEoEgI43rkh2m/m6oeuX3/omNat+zW2ea90/1N9+G7H78URUJQJARFQlAkBEVCUCSEOI+c5lu387xiev+y+ve0r+yUbV+hCiMSgiIhKBKCIiEoEoIiISgSQtyvdftexzZv3PZ13d7P3N7nnO6vG3eO9/3IX4YiISgSgiIhKBKCIiEoEsL1ux+3/VCr+f4s5/1WX9RX74fcvsdZYURCUCQERUJQJARFQlAkBEVCWN+PTPvcpPnmts9q+p7IdH/bz9W623pnhREJQZEQFAlBkRAUCUGREBQJIX4/ctvPZnoutppvWlfsPk+Z5nu3+Wo1LsWIhKBICIqEoEgIioSgSAiKhPCsX+v2fGx6jvX1edRzXLev6e+373ts80sjEoIiISgSgiIhKBKCIiEoEsL4fuTJbT+Zab/XtE73+rxotY/p3yXt73MyrQMbkRAUCUGREBQJQZEQFAlBkRDG51qnpHW92z6u37pfma7X0c17O78RCUGREBQJQZEQFAlBkRAUCaHNI2/vF37rXcn0/uKrvq0Vr+57bjEiISgSgiIhKBKCIiEoEoIiIYzvR27ftTh/f9tnZnoeNp13ul61btfHtttPWh89MSIhKBKCIiEoEoIiISgSgiIhXPfZ+bC9B/kqz6vyuimv8uRq3Lf288GIhKBICIqEoEgIioSgSAiKhBDnkSld3W767uL2/clpntnVSbt7lel9zW5965G/FEVCUCQERUJQJARFQlAkhK/nkV1e2PGtOt+re5Wv5kvHnRiREBQJQZEQFAlBkRAUCUGREOI8ctu/9Vv9UNP+Oek7lufntJ76rb/XiREJQZEQFAlBkRAUCUGREBQJYZxHbvuadvXDrr9M2ldnWyesPqd053jPcR2+H/nLUCQERUJQJARFQlAkBEVC+Hn9/oT8HYxICIqEoEgIioSgSAiKhKBICIqEoEgIioSgSAiKhKBICIqEoEgIioSgSAiKhKBICIqEoEgIioSgSAiKhKBICIqEoEgIioSgSAiKhKBICP8NAAD//+jOFvQQYmB4AAAAAElFTkSuQmCC", link: "blaze://pair?v=1&h=192.168.1.24,100.88.12.5&p=7377&fp=3f9a&c=demo&n=LUKE-PC", addresses: ["192.168.1.24", "100.88.12.5"], expires: Math.floor(Date.now() / 1000) + 287, error: "" }),
    StopPairing: async () => "",
    Unpair: async () => "",
    RouterInfo: async () => window.__rv,
    RouterDashboardURL: async pg => window.__rv.enabled ? window.__routerDash + "#t=demo" + (pg ? "&p=" + pg : "") : "",
    OpenRouterDashboard: async () => "",
    SetRouterOn: async on => { Object.assign(window.__rv, on ? { enabled: true, running: true, level: "green", note: "Running · 3 providers ready" } : { enabled: false, running: false, level: "red", note: "Off" }); return ""; },
    RouterPicker: async () => window.__rpicks,
    RouterStatus: async () => [{ provider: "gemini", model: "gemini-3.8-flash", name: "Google Gemini", ok: 14, avg_ms: 1840, can_tools: true, keys: 2 }, { provider: "groq", model: "openai/gpt-oss-120b", name: "Groq", ok: 3, fails: 1, cooling: 42, resting: "rate limit", avg_ms: 920, can_tools: true, keys: 1, last_error: "HTTP 429: rate limit reached" }],
    Keys: async () => [{ masked: "AIza…x9Q2", main: true, source: "saved" }, { masked: "AIza…7LmP", main: false, source: "saved" }],
    Usage: async () => ({ today: { in: 182000, out: 9400, cost: 0, calls: 41, priced: true }, month: { in: 2400000, out: 130000, cost: 0.42, calls: 512, priced: true },
      models: [{ provider: "gemini", model: "gemini-3.8-flash", in: 2100000, out: 110000, cost: 0, calls: 430, priced: true }, { provider: "groq", model: "openai/gpt-oss-120b", in: 300000, out: 20000, cost: 0, calls: 82, priced: true }],
      days: Array.from({ length: 14 }, (_, i) => ({ day: "2026-10-" + String(i + 1).padStart(2, "0"), tokens: [0, 12e3, 40e3, 8e3, 0, 90e3, 120e3, 60e3, 30e3, 0, 150e3, 80e3, 140e3, 191e3][i], calls: i * 3, cost: 0 })) }),
    Quota: async () => ({ rows: [{ label: "Status", value: "Working · gemini-3.8-flash" }], note: "Google shows exact per-model limits only in AI Studio.", page: "https://aistudio.google.com/" }),
    TestModel: async p => ({ name: p, ok: true, detail: "answered in 0.9s" }),
    ConnectOpenRouter: async () => ({ ok: false, error: "Sign-in didn't finish." }),
    UseFreeStack: async () => state.chain, CheckUpdate: async () => window.__upd || ({ current: "v0.4.1", latest: "v0.4.1", available: false }),
    // v0.4.8: /update in the chat. Progress events; never "restarts" so the shot can catch it.
    InstallUpdate: () => new Promise(() => { [8, 42].forEach((p, i) => setTimeout(() => fire("update", { stage: "download", pct: p, text: "" }), 150 + i * 150)); }),
    Files: async () => ["assets/theme.css", "design/option-a.html", "design/option-b.html", "layout/theme.liquid", "package.json", "sections/hero.liquid"],
    Design: async () => ({ base: window.__sample, target: { kind: "file", path: "index.html" }, files: ["index.html", "design/option-a.html"], tracked: ["index.html"], versions: [{ n: 1, summary: "First pass at the landing page", when: "21:02" }, { n: 2, summary: "Warmer palette and rounded cards", when: "21:10" }, { n: 3, summary: "Bigger hero headline", when: "21:18" }] }),
    SetHotkey: async id => ({ ...state.desktop, hotkey: id }), RetryHotkey: async () => state.desktop, SetAutostart: async () => "",
    MemoryGraph: async () => window.__memEmpty ? { nodes: [], edges: [], cats: [] } : window.__memGraph,
    EditMemory: async (id, text) => { const n = window.__memGraph.nodes.find(x => x.id === id); if (n) n.text = text; return { id }; },
    ForgetMemory: async () => "", SortMemories: async () => "", SetMemoryAISort: async () => {},
    PinMemory: async (id, on) => { const n = window.__memGraph.nodes.find(x => x.id === id); if (n) n.pinned = on; return ""; },
    // Like the database's porter-stemmed FTS: "deploying" finds "deploys".
    SearchMemory: async q => { const st = w => w.toLowerCase().replace(/(ing|ed|es|s)$/, ""); const ws = q.split(/\W+/).filter(w => w.length > 2).map(st);
      return window.__memGraph.nodes.filter(n => n.kind === "memory" && ws.some(w => n.text.split(/\W+/).some(t => st(t) === w))).map(n => n.id); },
    Doctor: async () => [{ name: "Internet", ok: true, detail: "Online" }, { name: "Google Gemini", ok: true, detail: "gemini-3.8-flash answered in 0.9s" }, { name: "Groq", ok: false, detail: "openai/gpt-oss-120b: Groq hit its free limit for now. Blaze will use another model." }, { name: "Shell", ok: true, detail: "bash · /usr/bin/bash" }, { name: "Tray", ok: true, detail: "In the system tray" }, { name: "Shortcut", ok: true, detail: "Press Ctrl+Shift+Space anywhere for quick ask" }, { name: "Start at login", ok: true, detail: "Off (Settings › Behaviour)" }],
  };
  // notify-vision: Settings › Notifications, the privacy indicator, Doctor's lines.
  const nprefs = { proactive: true, every: 10, max_hour: 3 };
  const vprefs = {};
  const nhist = [
    { id: "n3", kind: "suggest", title: "pnpm test fails in price.ts", body: "Expected 19.99, got 19.989999. I can fix the rounding.", prompt: "Fix the failing price test", at: "2026-10-10T10:42:00", method: "D-Bus notifications · mako", clicked: true },
    { id: "n2", kind: "task", title: "Blaze finished", body: "All 42 tests pass now.", at: "2026-10-10T10:31:00", method: "D-Bus notifications · mako" },
    { id: "n1", kind: "job", title: "Scheduled job “backup-notes” finished", body: "sent 1,204 files", at: "2026-10-09T07:00:00", method: "notify-send" }];
  const nview = () => ({ prefs: nprefs, quiet: nprefs.no_quiet ? ["", ""] : [nprefs.quiet_from || "22:00", nprefs.quiet_to || "08:00"], method: "D-Bus notifications · mako", kind: "native", clicks: true, fallback: "notify-send", probed: "Oct 10 09:12", history: nhist, watch: "Checked 10:52 · nothing worth interrupting" });
  const caps = os === "darwin"
    ? { text: false, text_via: "AX", text_note: "Blaze needs Accessibility access to read other apps' windows", pixels: false, pixels_via: "screencapture", pixels_note: "Allow Blaze in System Settings › Privacy & Security › Screen Recording", permission: "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture", text_perm: "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility" }
    : { text: true, text_via: "AT-SPI", text_note: "14 apps readable", pixels: true, pixels_via: "grim", pixels_note: "" };
  const vview = () => ({ prefs: vprefs, caps, os, last: os === "darwin" ? "" : "Firefox — Pull request #12 · 4s ago · AT-SPI 140 ms", warm: false });
  state.notify = nview(); state.vision = vprefs;
  Object.assign(A, {
    NotifyStatus: async () => nview(), ProbeNotify: async () => nview(), TestNotify: async () => "",
    SetNotifyPrefs: async p => { Object.keys(nprefs).forEach(k => delete nprefs[k]); Object.assign(nprefs, p); return nview(); },
    NotifyHistory: async () => nhist, ClearNotifyHistory: async () => { nhist.length = 0; }, SuggestionAction: async () => {},
    CheckSuggestionNow: async () => "Checked 10:58 · nothing worth interrupting",
    VisionStatus: async () => vview(), SetVisionPrefs: async p => { Object.keys(vprefs).forEach(k => delete vprefs[k]); Object.assign(vprefs, p); return vview(); },
    Design: (d0 => async () => ({ ...(await d0()), ...(state.board ? { artboard: state.board, scratch: true } : {}) }))(A.Design),
    DesignFormats: async () => [{ id: "post", name: "Social post", w: 1080, h: 1350, hint: "Instagram/LinkedIn portrait post" }, { id: "slide", name: "Slide", w: 1920, h: 1080, hint: "16:9 presentation slide" }, { id: "poster-a4", name: "Poster A4", w: 794, h: 1123, print: "A4", hint: "printable A4 poster or flyer" }],
    NewDesign: async (id, name) => { const f = (await A.DesignFormats()).find(x => x.id === id); state.board = f; return { base: window.__sample, target: { kind: "file", path: "index.html" }, files: ["index.html"], tracked: [], versions: [], artboard: f, scratch: true }; },
    ExportDesign: async kind => ({ path: "/home/luke/.blaze/designs/summer-sale/exports/summer-sale-v01." + kind }),
    RevealFile: async () => {},
    PrivateVision: async () => ({ on: !!state.pv, engine: os === "darwin" ? "macOS Vision" : "Tesseract", note: os === "darwin" ? "Apple's on-device text recognition" : "Tesseract OCR, on this computer" }),
    SetPrivateVision: async on => { state.pv = on; return A.PrivateVision(); },
    TryReadScreen: async () => "Read Firefox — \"Pull request #12\": 142 items in 138 ms via AT-SPI.", GrantScreen: async () => {}, GrantScreenText: async () => {},
  });
  const doctor0 = A.Doctor;
  A.Doctor = async () => (await doctor0()).concat(os === "darwin"
    ? [{ name: "Notifications", ok: true, detail: "macOS Notification Center · cached in ~/.blaze/notify.json" },
       { name: "Screen reading", ok: false, detail: "AX · Blaze needs Accessibility access to read other apps' windows", fix: caps.text_perm },
       { name: "Screenshots", ok: false, detail: "screencapture · Allow Blaze in System Settings › Privacy & Security › Screen Recording", fix: caps.permission },
       { name: "Proactive suggestions", ok: true, detail: "Every 10 min at most, 3 an hour, quiet 22:00–08:00" }]
    : [{ name: "Notifications", ok: true, detail: "D-Bus notifications · mako · cached in ~/.blaze/notify.json" },
       { name: "Screen reading", ok: true, detail: "AT-SPI · 14 apps readable" },
       { name: "Screenshots", ok: false, detail: "grim · Install grim for screenshots (Arch: sudo pacman -S grim)" },
       { name: "Proactive suggestions", ok: true, detail: "Every 10 min at most, 3 an hour, quiet 22:00–08:00 · Checked 10:52 · nothing worth interrupting" }]);
  // v0.4.7: background agents. A saved chat whose agent was interrupted by a restart.
  const now0 = Date.now();
  window.__agentSteps = [
    { id: "s1", name: "file_read", args: '{"path":"package.json"}', plain: "Reading package.json", result: "Read package.json (48 lines)", done: true },
    { id: "s2", name: "run_shell", args: '{"command":"pnpm build"}', plain: "Running a command", result: "exit 1 · build failed\nError: Cannot find module './hero.css'", bad: true, done: true },
    { id: "s3", name: "file_search", args: '{"pattern":"hero.css"}', plain: "Searching your files for \"hero.css\"", result: "sections/hero.liquid:3 · assets/Hero.css", done: true },
    { id: "s4", name: "file_edit", args: '{"path":"sections/hero.liquid"}', plain: "Editing hero.liquid", result: "Edited sections/hero.liquid (1 replacement).", done: true },
    { id: "s5", name: "run_shell", args: '{"command":"pnpm build"}', plain: "Running a command", result: "exit 0 · built in 9.8s", done: true },
    { id: "s6", name: "run_shell", args: '{"command":"pnpm test"}', plain: "Running a command", result: "exit 0 · 42 passed", done: true }];
  window.__agentConv = { id: "c8", title: "Ship the hero fix", folder: "/home/luke/code/homeglow", messages: [
    { role: "user", content: "Fix the build and get the tests green, I'll keep working on copy." },
    { role: "assistant", content: "", tool_calls: [{ id: "d1", function: { name: "dispatch_agent", arguments: '{"handle":"fix-build","task":"Fix the HomeGlow build"}' } }] },
    { role: "tool", tool_call_id: "d1", content: 'started: agent "fix-build" is working in the background.' },
    { role: "assistant", content: "On it. I'll report back when the build is green." }],
    agents: [{ handle: "fix-build", task: "Fix the failing HomeGlow build (pnpm build), then run pnpm test and make sure all tests pass. Report what was wrong.", conv: "c8", status: "interrupted", now: "", n: 3, steps: window.__agentSteps.slice(0, 3), started: now0 - 300000, ended: 0, seconds: 95 }] };
  // v0.6.0: the lead splits a task across three specialists (worktrees on their own branches).
  window.__specs = [
    { handle: "theme-toggle", kind: "specialist", mode: "build", iso: "worktree", branch: "blaze/theme-toggle", path: "/home/luke/.blaze/desktop/worktrees/theme-toggle-3fa1c2", conv: "c10",
      task: "Add a light/dark toggle to the HomeGlow header (sections/header.liquid + assets/theme-toggle.js). Persist the choice in localStorage, respect prefers-color-scheme on first visit. Run pnpm test.",
      plan: "1. Add the toggle button to header.liquid\n2. Write theme-toggle.js (localStorage + prefers-color-scheme)\n3. Run pnpm test and pnpm build" },
    { handle: "css-vars", kind: "specialist", mode: "build", iso: "worktree", branch: "blaze/css-vars", path: "/home/luke/.blaze/desktop/worktrees/css-vars-9b07de", conv: "c10",
      task: "Move every hard-coded colour in assets/*.css to CSS variables with a [data-theme=dark] set. Don't touch header.liquid. Run pnpm build.",
      plan: "1. Find hard-coded colours in assets/*.css\n2. Add :root and [data-theme=dark] variables\n3. Replace the colours, then pnpm build" },
    { handle: "contrast-tests", kind: "specialist", mode: "build", iso: "worktree", branch: "blaze/contrast-tests", path: "/home/luke/.blaze/desktop/worktrees/contrast-tests-51c0aa", conv: "c10",
      task: "Write vitest tests in tests/contrast.test.ts that check text/background contrast is at least 4.5:1 in both themes, reading the variables from assets/base.css.", plan: "" }];
  window.__specChanges = [{ path: "assets/base.css", op: "modified" }, { path: "assets/product.css", op: "modified" }, { path: "assets/cart.css", op: "modified" }, { path: "assets/theme-dark.css", op: "added" }];
  // v0.5.0: Blaze account through blaze-auth (signed out until a shot signs in).
  const acctUser = { id: "u_demo", email: "kapapaluckson75@gmail.com", name: "Lackson Kapapa", avatar_url: "", email_verified: true, has_password: true, providers: ["google"], created_at: 1791720000 };
  window.__acct = { signed_in: false, user: null, login: { email: true, google: true, github: false }, online: true, pending: "", service: "https://blaze-auth.blaze-app.workers.dev" };
  window.__signIn = () => { Object.assign(window.__acct, { signed_in: true, user: acctUser, pending: "" }); };
  Object.assign(A, {
    AccountState: async () => ({ ...window.__acct }),
    AccountEmail: async (mode, name, email, password) => { if ((password || "").length < 8) return { error: "That email and password don't match." }; window.__signIn(); return { state: { ...window.__acct } }; },
    AccountSignIn: async p => { window.__acct.pending = p; return ""; }, AccountCancel: async () => { window.__acct.pending = ""; },
    AccountSignOut: async () => { Object.assign(window.__acct, { signed_in: false, user: null }); return ""; }, AccountDelete: async () => { Object.assign(window.__acct, { signed_in: false, user: null }); return ""; },
    ConnectorConnect: async id => { const c = connView.connectors.find(x => x.id === id); if (c && c.one_click) { c.status = "connecting"; c.pending = { kind: "browser", url: "https://blaze-auth.blaze-app.workers.dev/oauth/" + id + "/start?state=…", one_click: true }; return { status: "waiting", url: c.pending.url }; } return { status: "error", error: "Paste your client ID first." }; },
    ConnectorCancel: async id => { const c = connView.connectors.find(x => x.id === id); if (c) { c.status = "not_connected"; c.pending = null; } },
    ConnectorConnectManual: async () => ({ status: "error", error: "Paste your client ID first (see the steps)." }),
  });
  Object.assign(A, { Agents: async () => [], AgentStop: async () => "", AgentResume: async () => "", AgentDiscard: async () => "", AnswerAgent: async () => {}, SetMaxAgents: async n => n, Live: async () => [],
    SpecialistReview: async (h, action) => { window.fire && window.fire("agent", { handle: h, review: action === "apply" ? "applied" : "discarded" }); return ""; } });
  // v0.7: bundled skill library + learning (learn.js).
  window.__skills = [{"name": "api-integration", "description": "Connect to a web API or webhook - read the docs, authenticate (API key, OAuth, bearer token), call endpoints with curl or code, handle pagination, rate limits, retries and errors, and store keys safely. Use when the user wants to automate a service (Shopify, Stripe, GitHub, Google, Notion, Telegram, any REST/GraphQL API) or build an integration.", "tags": ["api", "rest", "http", "integration"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "code-review", "description": "Review code changes - a diff, a branch, a pull request or a file - for bugs, security, performance, readability and tests, and report findings ranked by severity with concrete fixes. Use when the user asks to \"review\", \"check my code\", \"look over this PR\" or before merging Blaze's own work.", "tags": ["code", "review", "quality", "security"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "data-analysis", "description": "Analyse data from CSV, Excel/Google Sheets exports, JSON or SQLite - clean it, answer questions with numbers, find trends and outliers, make charts and a short written summary. Use for \"analyse this spreadsheet\", sales/expense/traffic numbers, survey results, logs or any table of data.", "tags": ["data", "csv", "excel", "spreadsheet"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "debugging", "description": "Find and fix the root cause of a bug, crash, failing test, wrong output, build error or \"it worked yesterday\" - systematically, with a reproduction, one hypothesis at a time, and a regression test. Use whenever something is broken or behaves unexpectedly.", "tags": ["code", "debugging", "errors", "testing"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "deep-research", "description": "Research a question thoroughly on the web - compare options, check facts, survey a topic, find the best tool/product/price - and answer with sources for every claim. Use when the answer needs more than one search, the stakes are real, or the user asks for \"research\", \"compare\", \"find out\", \"is it true that\".", "tags": ["research", "web", "sources", "fact-check"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "deploy-homeglow", "description": "Deploy the HomeGlow Shopify theme: pnpm build, shopify theme push, purge the CDN and check the live store.", "tags": ["deploy", "shopify", "pnpm"], "version": 3, "state": "own", "enabled": true, "update": false}, {"name": "document-writing", "description": "Write or edit a document - report, proposal, README, guide, essay, cover letter, SOP, meeting notes - that is clear, structured and ready to share as Markdown, DOCX or PDF. Use for any request to draft, rewrite, summarise or polish long-form text.", "tags": ["writing", "docs", "report", "markdown"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "email-drafting", "description": "Draft emails and messages - replies, follow-ups, cold outreach, client updates, complaints, requests, apologies, Slack/WhatsApp messages and freelance proposals (Upwork/Fiverr) - in the user's voice, short and clear, always shown for approval before anything is sent. Use for any \"write/reply to/answer this email or message\" request.", "tags": ["writing", "email", "messages", "communication"], "version": 1, "state": "bundled", "enabled": false, "update": false}, {"name": "git-pr-workflow", "description": "Day-to-day git and GitHub - branch, commit with good messages, rebase on main, resolve conflicts, push, open a pull request, watch CI, merge, tag a release, and undo mistakes safely. Use for any git or PR task, or when finishing a coding change.", "tags": ["git", "github", "pr", "ci"], "version": 2, "state": "edited", "enabled": true, "update": true}, {"name": "image-assets", "description": "Create, convert and optimise images and visual assets - resize, crop, compress, convert (PNG/JPEG/WebP/AVIF/SVG/ICO), make favicons and app icons, social/OG images, product photos, screenshots, simple SVG logos and icons, background removal - with ImageMagick, ffmpeg, Python or SVG by hand. Use for any image or asset file task.", "tags": ["images", "assets", "svg", "imagemagick"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "landing-page-builder", "description": "Build a single-file landing page (one index.html with inline CSS/JS) that is responsive, accessible and fast - for a product, event, portfolio, waitlist or offer. Use whenever the user wants \"a page\", \"a site\", \"a landing page\" or \"a one-pager\" and no framework is required.", "tags": ["web", "html", "css", "landing-page"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "planning-task-breakdown", "description": "Turn a goal or a big messy request into a concrete plan - milestones, ordered tasks with owners and estimates, dependencies, risks and the next action - and keep it updated as work happens. Use for projects, launches, study plans, \"how do I start\", multi-step coding work, or before any task that will take more than ~30 minutes.", "tags": ["planning", "tasks", "project-management", "productivity"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "refactoring", "description": "Restructure existing code without changing its behaviour - split large files or functions, remove duplication, rename, untangle dependencies, modernise APIs - in small, tested, reversible steps. Use when code is hard to change, before adding a feature to messy code, or when the user asks to \"clean up\" or \"refactor\".", "tags": ["code", "refactoring", "quality", "architecture"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "shell-sysadmin", "description": "Run and fix things on the user's computer across Windows (PowerShell), macOS and Linux including Arch - install software, manage services, disk space, processes, ports, networks, permissions, startup apps, scheduled jobs and updates - safely and with the right commands for that OS. Use for any \"my PC\u2026\", \"install\u2026\", \"why is X slow/full/broken\" or system-level task.", "tags": ["shell", "sysadmin", "windows", "macos"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "skill-creator", "description": "How to write or improve a Blaze skill (SKILL.md) - when a workflow is worth saving, how to name and describe it so it's found, how to structure the steps, checklists and templates, and how to update it later. Use before calling write_skill, when the user says \"save this as a skill\" or teaches you a method, or when an existing skill turned out wrong.", "tags": ["skills", "meta", "writing", "workflow"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "slide-deck", "description": "Make a presentation - pitch deck, class talk, project update or report-out - as a single self-contained HTML deck (arrow keys, fullscreen, print to PDF) or as .pptx when the user needs PowerPoint. Use for any \"slides\", \"deck\", \"presentation\" or \"pitch\" request.", "tags": ["slides", "presentation", "writing", "design"], "version": 1, "state": "removed", "enabled": false, "update": false}, {"name": "web-audit", "description": "Audit a website or web app for accessibility, performance, SEO and basic security, then fix the top issues - with Lighthouse, axe, browser devtools and manual keyboard/screen-reader checks. Use when a page is slow, before launch, when the user asks \"is my site good/accessible/fast\", or after building a page.", "tags": ["web", "accessibility", "performance", "seo"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "web-design-blaze-classic", "description": "Design or build pages, apps or components in the classic Blaze look from blazeinchue.netlify.app - paper #fcfcfc, ink #111110, one blue accent #1f4bd8, square edges, Inter + Inter Tight, mono uppercase labels, hairline-ruled grids and particle-dot hovers. Use for anything Blaze-branded or when the user asks for \"the Blaze style\".", "tags": ["design", "web", "css", "brand"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "web-design-teamofkeys", "description": "Design or build a website in the style of teamofkeys.com - near-black and white sections, one hot orange accent, huge tight sans headlines with one italic serif word, mono \"(labels)\", pill CTAs and slow scroll-driven motion. Use when the user asks for that look, a studio/agency feel, or names Team of Keys.", "tags": ["design", "web", "css", "landing-page"], "version": 1, "state": "bundled", "enabled": true, "update": false}, {"name": "writing-tests", "description": "Write or improve automated tests - unit, integration, table-driven, regression, snapshot or end-to-end - for Go, Python, JavaScript/TypeScript and others, so they catch real bugs and stay fast and reliable. Use when adding features, fixing bugs, raising coverage or when tests are flaky.", "tags": ["code", "testing", "go", "python"], "version": 1, "state": "bundled", "enabled": true, "update": false}];
  state.prefs.learn = {};
  Object.assign(A, {
    SkillList: async () => window.__skills.map(s => ({ ...s })),
    SkillText: async n => "---\nname: " + n + "\ndescription: " + ((window.__skills.find(s => s.name === n) || {}).description || "") + "\ntags: [web, design]\nversion: 1\n---\n\n# " + n + "\n\n1. Read the brief.\n2. Copy assets/tokens.css.\n3. Build, then screenshot and check.\n",
    SetSkillEnabled: async (n, on) => { const s = window.__skills.find(x => x.name === n); if (s) s.enabled = on; return ""; },
    DeleteSkill: async n => { const s = window.__skills.find(x => x.name === n); if (s) { if (s.state === "own") window.__skills.splice(window.__skills.indexOf(s), 1); else Object.assign(s, { state: "removed", enabled: false }); } return ""; },
    RestoreSkill: async n => { const s = window.__skills.find(x => x.name === n); if (s) Object.assign(s, { state: "bundled", enabled: true, update: false }); return ""; },
    SetLearn: async on => { state.prefs.learn = { ...state.prefs.learn, off: !on }; }, SetLearnMarks: async on => { state.prefs.learn = { ...state.prefs.learn, no_marks: !on }; },
  });
  state.skills = window.__skills.filter(s => s.state !== "removed").map(s => s.name + ": " + s.description);
  // v0.8.0: the Design studio (studio.js).
  const brand = { name: "HomeGlow", source: "https://homeglow.example", paper: "#fbf7f1", ink: "#1b1b1b", accents: ["#e8743b", "#2f6f5e"], fonts: ["Fraunces", "Inter"], radius: "10px" };
  window.__dirs = [];
  Object.assign(A, {
    Studio: async () => ({ brand, prefs: {}, stack: "plain HTML/CSS", head: 3, phone: false, comments: 1 }),
    StudioPrefs: async () => ({}), SetStudioPrefs: async p => p,
    BrandKit: async () => brand, SetBrand: async b => b, ClearBrand: async () => ({}),
    BrandFromURL: async () => ({ brand }), BrandFromImageData: async () => ({ brand }),
    Taste: async () => ({ prompt: "TASTE (learned)\n- Likes: fonts Fraunces, Inter; colours #e8743b; corner radius 10px\n- Avoid: colours #1f4bd8\n", prefs: [] }),
    SaveCritique: async n => ({ card: { version: n, score: 78, a11y: 70, layout: 75, speed: 100, issues: ["2 text elements with low contrast (worst 2.9:1 on p.muted, needs 4.5:1)", "Scrolls sideways on phone (36px too wide)", "1 image without alt text"] } }),
    // v0.20 studio (design_studio.go)
    SetStudioMode: async m => (["page", "3d", "motion", "audio"].includes(m) ? m : "page"),
    DesignPalette: async () => ({ brand: ["#1b1b1b", "#fbf7f1", "#e8743b", "#2f6f5e"], page: ["#ffffff", "#f3eee6", "#c9b8a4"] }),
    ApplyEdit: async req => ((window.__edits = window.__edits || []).push(req), { ok: true, file: "index.html", diff: "--- a/index.html\n+++ b/index.html\n@@ -12,3 +12,3 @@\n-<h1>Warm light</h1>\n+<h1>Warm light for every corner</h1>\n", add: 1, del: 1, version: 4 }),
    SwapColor: async () => ({ ok: true, file: "index.html, theme.css", diff: "", add: 6, del: 6, version: 4 }),
    Directions: async () => window.__dirs, StartDirections: async () => "ok", MixDirections: async () => "ok", PickDirection: async () => "ok", DiscardDirections: async () => "ok",
    PhonePreview: async on => on ? { on: true, url: "http://192.168.1.24:51733/index.html?k=3f9a1c", qr: "data:image/svg+xml;utf8," + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 21 21" shape-rendering="crispEdges"><rect width="21" height="21" fill="#fff"/><path d="M1 1h7v7H1zM13 1h7v7h-7zM1 13h7v7H1z" fill="#000"/><path d="M2 2h5v5H2zM14 2h5v5h-5zM2 14h5v5H2z" fill="#fff"/><path d="M3 3h3v3H3zM15 3h3v3h-3zM3 15h3v3H3zM10 2h1v3h-1zM9 9h3v3H9zM14 10h2v2h-2zM12 14h3v1h-3zM17 16h2v3h-2zM10 17h2v2h-2z" fill="#000"/></svg>') } : {},
    Comments: async () => [{ id: "c1", sel: "section.hero > a.btn", text: "Button feels small on my phone", when: "21:14", from: "phone" }],
    ShipInfo: async () => ({ github: true, repo: "luke/homeglow", node: true, stack: "plain HTML/CSS", comments: true, shipped: [{ provider: "cloudflare", url: "https://homeglow.pages.dev", when: "10 Oct 18:02", version: 2 }] }),
    Ship: async () => "ok",
  });
  // v0.19 one infinite chat: the main conversation (30 turns) pages in from the end; the last turn is the
  // price-test fix. Pages start at a user message, like internal/chatpage.Window.
  const proj1 = state.project;
  const topics = [["What's taking space in ~/Downloads?", "**18.4 GB** in `~/Downloads`: three Ubuntu ISOs (11 GB) and old Zoom recordings (5 GB). Want me to move the ISOs to the external drive?"],
    ["Summarise the HomeGlow launch checklist", "Five things left: product photos, the returns page, the **pnpm build** on CI, DNS for homeglow.store and a test order."],
    ["Remind me to call the printer tomorrow at 10", "Done: a reminder **tomorrow at 10:00**, “Call the printer about the flyers”."],
    ["How do I undo the last git commit but keep the changes?", "`git reset --soft HEAD~1` keeps your changes staged. Nothing is lost."],
    ["Is my Postgres container still running?", "Yes, `homeglow-db` (postgres:16) is up 3 days on port 5432."],
    ["Draft a short reply to Ada about Friday", "Here's a draft:\n\n> Hi Ada, Friday at 2 works. I'll bring the mock-ups. Luke"]];
  const mainMsgs = [];
  for (let i = 0; i < 29; i++) { const t = topics[i % topics.length]; mainMsgs.push({ role: "user", content: t[0] }, { role: "assistant", content: t[1] }); }
  mainMsgs.push(...conv.messages);
  const win = (msgs, before, turns) => { let n = 0; for (let i = before - 1; i >= 0; i--) if (msgs[i].role === "user" && ++n === (turns || 12)) return i; return 0; };
  // v0.19 Home board.
  const svg = (a, b, w = 320, hh = 180) => "data:image/svg+xml;utf8," + encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${hh}"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${a}"/><stop offset="1" stop-color="${b}"/></linearGradient></defs><rect width="${w}" height="${hh}" fill="url(#g)"/><circle cx="${w * .72}" cy="${hh * .38}" r="${hh * .22}" fill="rgba(255,255,255,.35)"/><rect x="${w * .1}" y="${hh * .62}" width="${w * .5}" height="${hh * .08}" rx="4" fill="rgba(255,255,255,.5)"/></svg>`);
  const hnow = Date.now(), hm = 60000;
  const W = (id, type, title, category, size, config) => ({ id, type, title, category, size: size || "M", config: config || {}, order: 0 });
  const hw = [W("w1", "weather", "Weather", "Weather", "M", { use_location: "true" }), W("w2", "news", "Top stories", "News", "L", { query: "" }),
    W("w3", "prices", "Bitcoin", "Finance", "M", { kind: "crypto", ids: "bitcoin" }), W("w4", "prices", "USD to ZMW", "Finance", "M", { kind: "fx" }),
    W("w5", "schedule", "Schedule", "Today"), W("w6", "deadlines", "Deadlines", "Today"), W("w7", "sports", "Arsenal", "Sports"),
    W("w8", "recents", "Pick up where you left off", "Pick up"), W("w9", "github", "golang/go", "Tech", "M", { repo: "golang/go" })];
  hw.forEach((w, i) => w.order = i);
  const hd = {
    w1: { headline: "Lusaka", value: "28°C", change: "Partly cloudy · feels 28° · wind 11 km/h", lines: [{ t: "Today", s: "Partly cloudy", n: "31° / 17°" }, { t: "Tue", s: "Rain · rain 40%", n: "29° / 18°" }, { t: "Wed", s: "Mostly clear", n: "30° / 17°" }], link: "https://open-meteo.com/" },
    w2: { headline: "Top stories", lines: [{ t: "Zambia's solar push adds 100 MW to the grid ahead of the rainy season", s: "Lusaka Times · 42 min ago", u: "https://example.com/1", i: svg("#F2A541", "#8C3B1F") },
      { t: "Copper prices hit a three-month high as demand from EV makers grows", s: "Reuters · 1 h ago", u: "https://example.com/2", i: svg("#5A7D9A", "#1E2B3A", 136, 96) },
      { t: "Chipolopolo name squad for the AFCON qualifiers", s: "ZNBC · 2 h ago", u: "https://example.com/3", i: svg("#3E8E5E", "#123524", 136, 96) },
      { t: "Open-source AI models close the gap on coding benchmarks", s: "The Verge · 3 h ago", u: "https://example.com/4" },
      { t: "Kafue Gorge upgrade finishes two weeks early", s: "Lusaka Times · 4 h ago", u: "https://example.com/5" }] },
    w3: { headline: "Bitcoin", value: "64,210 USD", change: "▲ 2.1% today", link: "https://www.coingecko.com/en/coins/bitcoin" },
    w4: { headline: "USD → ZMW", value: "26.48 ZMW", change: "1 USD" },
    w5: { headline: "Next up", value: "Call the printer", change: "Tue 20 Oct · 10:00", lines: [{ t: "backup-notes", s: "daily 07:00 · rsync -a ~/notes /mnt/backup/" }, { t: "weekly-digest", s: "every monday at 8:00 · Brief me on the week" }] },
    w6: { headline: "Deadlines", value: "Call the printer", change: "Tue 20 Oct · 10:00", lines: [{ t: "HomeGlow launch is due 31 October", n: "From memory" }] },
    w7: { headline: "Arsenal", value: "2–1", change: "Arsenal vs Chelsea", lines: [{ t: "Spurs vs Arsenal", s: "Sun 25 Oct · 16:30 · English Premier League", n: "Next" }, { t: "Arsenal 2–1 Chelsea", s: "FT · English Premier League" }] },
    w8: { headline: "Pick up where you left off", lines: [{ t: "Run the tests and fix whatever fails.", s: "Your last message", u: "blaze:chat:main", n: "Chat" }, { t: "HomeGlow", s: "homeglow", u: "blaze:project:p1", n: "Project" }, { t: "blaze-ai", s: "/home/luke/code/blaze-ai", u: "blaze:folder:/home/luke/code/blaze-ai", n: "Folder" }] },
    w9: { headline: "golang/go", lines: [{ t: "cmd/go: cache test results across modules", s: "rsc · 25 min ago", u: "https://github.com/golang/go/commits" }, { t: "Opened PR #75310: runtime: faster maps on arm64", s: "aclements · 1 h ago", u: "https://github.com/golang/go/pull/75310" }, { t: "Released go1.26.3", s: "gopherbot · 6 h ago" }] } };
  Object.keys(hd).forEach((k, i) => hd[k].fetched = hnow - (6 + i * 3) * hm);
  const hstacks = () => { const out = []; hw.slice().sort((a, b) => a.order - b.order).forEach(w => { const k = "s:" + w.category.toLowerCase(); const s = out.find(x => x.key === k); if (s) s.widgets.push(w); else out.push({ key: k, category: w.category, widgets: [w] }); }); return out; };
  Object.assign(A, {
    OpenMainPage: async turns => { state.project = null; state.prefs.project = ""; const start = win(mainMsgs, mainMsgs.length, turns); return { id: "main", title: "Blaze", messages: mainMsgs.slice(start), start, total: mainMsgs.length, agents: [] }; },
    MainHistory: async (before, turns) => { const start = win(mainMsgs, before, turns); return { id: "main", messages: mainMsgs.slice(start, before), start, total: mainMsgs.length }; },
    SearchHistory: async q => { const ws = q.toLowerCase().split(/\s+/).filter(Boolean); const out = [];
      for (let i = mainMsgs.length - 1; i >= 0; i--) { const x = mainMsgs[i]; if ((x.role === "user" || x.role === "assistant") && x.content && ws.every(w => x.content.toLowerCase().includes(w))) out.push({ conv: "main", title: "Chat", index: i, role: x.role, snippet: x.content.slice(0, 140) }); }
      if (ws.some(w => "postgres".includes(w))) out.push({ conv: "c2", title: "Set up a Postgres container", when: "8 Oct 2026", index: 0, role: "user", snippet: "Set up a Postgres 16 container for HomeGlow, data in ~/pg" });
      return out.slice(0, 12); },
    OpenProject: async id => { state.project = id === "p1" ? proj1 : id === "p2" ? { id: "p2", name: "Blaze.ai", folder: "/home/luke/code/blaze-ai", instructions: "" } : null; state.prefs.project = id || ""; return state.project || {}; },
    HomeBoard: async () => ({ widgets: hw, data: Object.fromEntries(Object.entries(hd).map(([k, v]) => [k, { ...v }])), stacks: hstacks(), seeded: true, last_refresh: hnow - 6 * hm, greeting: window.__greeting || "Good afternoon", name: "Luke", date: "Monday 19 October" }),
    HomeRefresh: async id => { hd[id] = { ...(hd[id] || {}), fetched: Date.now() }; return { ...hd[id] }; },
    HomeAdd: async text => { const w = W("w" + (hw.length + 1), "news", text.replace(/^./, c => c.toUpperCase()) + (/news$/i.test(text) ? "" : " news"), "News"); w.order = hw.length; hw.push(w); hd[w.id] = { headline: w.title, lines: [{ t: "A first story about " + text, s: "Example · 5 min ago" }] }; return w; },
    HomeRemove: async id => { const i = hw.findIndex(w => w.id === id); if (i >= 0) hw.splice(i, 1); delete hd[id]; },
    HomeMove: async (id, d) => { const l = hw.slice().sort((a, b) => a.order - b.order); const i = l.findIndex(w => w.id === id), j = i + d; if (i < 0 || j < 0 || j >= l.length) return false; [l[i], l[j]] = [l[j], l[i]]; l.forEach((w, k) => w.order = k); return true; },
    HomeResize: async (id, sz) => { const w = hw.find(x => x.id === id); if (w) w.size = sz; },
    HomeSetCity: async (id, c) => { const w = hw.find(x => x.id === id); if (w) { w.title = "Weather in " + c; hd[id] = { ...hd.w1, headline: c }; } },
    HomeInto: async t => t.split(",").filter(x => x.trim()).length,
  });
  window.__enterProject = async () => { await A.OpenProject("p1"); await refreshState(); convId = ""; hero(); show("work"); };
  // v0.19 Blaze Cloud + Custom models (cloudmodels.js). "custom" here keeps the older BlazeRouter shots as they were;
  // the models-* shots switch with SetModelsMode("cloud").
  window.__cm = { mode: "custom", fallback: false, gateway: "https://ai.blaze-app.workers.dev/v1", custom: "Auto (free routing)",
    overrides: [{ task: "chat" }, { task: "code", provider: "groq", model: "llama-3.3-70b-versatile", label: "Groq · llama-3.3-70b-versatile" }, { task: "vision" }] };
  window.__cmStatus = { ready: true };
  window.__cmModel = state.model;
  const cmView = () => ({ ...window.__cm, overrides: window.__cm.overrides.map(o => ({ ...o })), signed_in: !!window.__acct.signed_in, email: window.__acct.user ? window.__acct.user.email : "", label: state.model });
  Object.assign(A, {
    ModelsChoice: async () => cmView(),
    SetModelsMode: async m => { window.__cm.mode = m; state.model = m === "cloud" ? "Blaze Cloud" : window.__cmModel; return cmView(); },
    SetCloudFallback: async v => { window.__cm.fallback = !!v; return cmView(); },
    SetTaskModel: async (task, p, m) => { const o = window.__cm.overrides.find(x => x.task === task); if (!o) return "Blaze can pin chat, code or vision."; Object.assign(o, p ? { provider: p, model: m, label: p + " · " + m } : { provider: "", model: "", label: "" }); return ""; },
    CloudStatus: async () => window.__acct.signed_in ? { ...window.__cmStatus } : { ready: false, line: "Sign in to your Blaze account and I'm ready to go.", code: 401 },
  });
  window.go = { main: { App: new Proxy(A, { get: (t, k) => t[k] || (async () => "") }) } };
  // Events are captured so the screenshot script can play a turn: fire("tool", {...}).
  window.__ev = {};
  window.runtime = { EventsOn(ev, fn) { (window.__ev[ev] = window.__ev[ev] || []).push(fn); } };
  window.fire = (ev, data) => (window.__ev[ev] || []).forEach(f => f(data));
  window.__conv = conv;
  // v0.18.1 shots: play a streamed reply and hold one frame mid-stream (drop riding the text, shimmer on),
  // and a running agent turn with the edge particles frozen on one frame.
  window.__streamDemo = async (hold = 700) => {
    await newChat(); sendText("Why did the price test fail?", []);
    const parts = ["The test failed because ", "`price.ts` added floats directly, so 19.99 ", "came out as 19.989999. I now round to cents ",
      "before returning, and all 42 tests pass again. The newest words glow blue while "];
    for (const p of parts) { fire("delta", p); await new Promise(r => setTimeout(r, 120)); }
    return StreamOrb.freeze(hold);
  };
  window.__runDemo = async () => {
    await newChat(); sendText("Make the hero section blue", []);
    for (let i = 0; i < 3; i++) { fire("tool", { id: "rd" + i, name: "file_read", args: '{"path":"index.html"}' }); fire("tool_done", { id: "rd" + i, result: "ok" }); }
    fire("tool", { id: "rd9", name: "file_edit", args: '{"path":"styles.css"}' });
    await new Promise(r => setTimeout(r, 500));
    EdgeParticles.frame(3);
    return EdgeParticles.active;
  };
  // Self-knowledge (v0.12): "how do I connect my phone?" answered from about_blaze, not with questions.
  window.__convAbout = { id: "c9", title: "Connect my phone", messages: [
    { role: "user", content: "how do I connect my phone?" },
    { role: "assistant", content: "", tool_calls: [{ id: "a1", function: { name: "about_blaze", arguments: "{\"section\":\"link\"}" } }] },
    { role: "tool", tool_call_id: "a1", content: "BLAZE LINK (PHONE ↔ PC)\n- Status in this build: the PC side is NOT in this Blaze Desktop build…\n- Paired phones: none." },
    { role: "assistant", content: "Blaze on your phone links to Blaze Desktop in two ways. Your account works today; direct pairing (Blaze Link) needs the PC side, which isn't in this build yet.\n\n**Works now**\n1. Open **/account** here and sign in, then sign in to the same Blaze account in Blaze on your phone. Your model keys follow you, encrypted.\n2. To see a design on the phone: **Design › Phone** shows a QR code. Scan it with the phone's camera on the same Wi-Fi.\n\n**Blaze Link pairing** (once the PC side ships)\n1. Blaze Desktop › **Phone** › *Show pairing QR code*.\n2. On the phone: Blaze › **Settings › Blaze Desktop**, then scan it.\n3. Keep both on the same Wi-Fi, or both on Tailscale. The link is direct HTTPS on port 7377, pinned to this PC.\n\nNo phone is paired with this PC right now." },
  ] };
})();
