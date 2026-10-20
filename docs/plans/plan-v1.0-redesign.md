# Blaze 1.0 plan: calm UI, long chats, students and marketers (Oct 12 2026)

Owner: Luke. Standing rules:
- Merge feature PRs once CI is green.
- Ship preview screenshots with every build (`CHROME=/usr/bin/chromium python3 ci/shots.py`).
- Free tier only.
- Keep the classic Blaze look: paper #fcfcfc, ink #111110, accent #1f4bd8 (dark #26262a / #6b8cff), square hairlines, Inter + Inter Tight, mono uppercase labels, particle dots.
- **Never delete an existing page or feature.** Pages move off the main screen; they don't disappear.

Order:
1. BlazeRouter (branch `blazerouter`, in progress)
2. Redesign (this doc, part A)
3. Long-chat engine (part B)
4. Student pack (part C)
5. Marketer pack (part D)

Each one is its own PR and version bump.

---

## A. Redesign: a minimal shell like a messaging app

### A1. Layout
- **Top bar:**
  - Left: the B logo. Clicking it returns to the basic chat.
  - Centre: a pill with `Chat | Projects`.
  - Right: a `Search` chip (Ctrl+K), the BlazeRouter status dot, and the account avatar.
- **Left rail (Projects mode only):** the project list with status chips (`Working`, `Needs you`). In Chat mode the rail is the recent chats list, and it collapses below 1100px.
- **Centre column (max 760px):** bubbles. User messages are right-aligned on an ink bubble (paper text in dark mode). Blaze messages are left-aligned with no bubble (plain text), and cards stay square hairline boxes.
- **Work in progress is ONE live card** per turn: `● Working · 4 steps · 1m 12s ›`. Clicking it expands to today's steps view (reuse steps.js, specialists.js). When finished it collapses to `Worked · 13 steps · 3m 11s ›` and the answer follows as normal text.
- **Composer:** one rounded bar with `+` on the left and Send on the right.
  - The `+` menu holds Attach, Voice, @files, Plan/Build, Auto/Autopilot and Model (opens BlazeRouter).
  - The current mode shows as a small chip inside the bar, such as `BUILD · AUTOPILOT`, only when it isn't the default.
  - The shortcuts strip moves into a `?` tooltip.
- **Account avatar menu:** Account, Settings, BlazeRouter dashboard, Theme (sun/moon), Check for updates, Sign out.

### A2. Nothing is deleted: `/views`
Memory, Jobs, Schedule, Accounts, Doctor, Settings, Design, BlazeRouter, Learn and Agents all stay. They come off the sidebar, and one command opens a picker like `/skills`.
- `/views` opens a picker with a filter box, arrow-key navigation and Enter. Each row shows an icon, the name, a one-line description and a badge (job count, update available).
- The direct shortcuts still work: `/memory /jobs /schedule /accounts /doctor /settings /design /router`.
- Ctrl+K Search also lists views.

```js
// frontend/dist/views.js — registry + /views picker (vanilla, matches app.js style)
window.BlazeViews = (() => {
  const VIEWS = [
    { id: 'memory',   name: 'Memory',      desc: 'What Blaze remembers',          open: () => showView('memory') },
    { id: 'jobs',     name: 'Jobs',        desc: 'Running and past jobs',         open: () => showView('jobs'), badge: () => $('#jobCount')?.textContent },
    { id: 'schedule', name: 'Schedule',    desc: 'Timed tasks and reminders',     open: () => showView('schedule') },
    { id: 'accounts', name: 'Accounts',    desc: 'Connected apps',                open: () => showView('accounts') },
    { id: 'doctor',   name: 'Doctor',      desc: 'Check this PC and Blaze',       open: () => showView('doctor') },
    { id: 'settings', name: 'Settings',    desc: 'Behaviour, voice, privacy',     open: () => showView('settings') },
    { id: 'design',   name: 'Design',      desc: 'Live preview and artboards',    open: () => enterMode('design') },
    { id: 'router',   name: 'BlazeRouter', desc: 'Models, keys, usage dashboard', open: () => window.go.main.App.OpenRouterDashboard() },
  ];
  const $ = s => document.querySelector(s);
  // showView/enterMode come from app.js; expose them there as window.showView / window.enterMode.
  const showView = id => window.showView(id);
  const enterMode = m => window.enterMode(m);

  function picker(filter = '') {
    const box = document.createElement('div');
    box.className = 'vpick';
    box.innerHTML = `<input class="vq" placeholder="Search views" value="${filter}"><div class="vl"></div>`;
    const list = box.querySelector('.vl'), q = box.querySelector('input');
    let sel = 0, rows = [];
    const draw = () => {
      const f = q.value.toLowerCase();
      rows = VIEWS.filter(v => (v.name + v.desc).toLowerCase().includes(f));
      sel = Math.min(sel, Math.max(rows.length - 1, 0));
      list.innerHTML = rows.map((v, i) =>
        `<button class="vr${i === sel ? ' on' : ''}" data-i="${i}"><span class="label">${String(i + 1).padStart(2, '0')}</span>` +
        `<b>${v.name}</b><span class="d">${v.desc}</span><span class="count">${v.badge?.() || ''}</span></button>`).join('');
    };
    const go = i => { box.remove(); rows[i]?.open(); };
    q.oninput = () => { sel = 0; draw(); };
    q.onkeydown = e => {
      if (e.key === 'ArrowDown') { sel = (sel + 1) % rows.length; draw(); e.preventDefault(); }
      else if (e.key === 'ArrowUp') { sel = (sel - 1 + rows.length) % rows.length; draw(); e.preventDefault(); }
      else if (e.key === 'Enter') go(sel);
      else if (e.key === 'Escape') box.remove();
    };
    list.onclick = e => { const b = e.target.closest('.vr'); if (b) go(+b.dataset.i); };
    document.getElementById('composerWrap').appendChild(box);
    draw(); q.focus();
  }

  // Hook into the existing slash-command table in composer.js.
  function slash(cmd) {
    if (cmd === '/views') { picker(); return true; }
    const v = VIEWS.find(v => '/' + v.id === cmd); if (v) { v.open(); return true; }
    return false;
  }
  return { VIEWS, picker, slash };
})();
```

```css
/* frontend/dist/views.css */
.vpick{position:absolute;bottom:calc(100% + 8px);left:0;right:0;background:var(--paper);border:1px solid var(--line);box-shadow:0 8px 24px rgba(0,0,0,.08)}
.vpick .vq{width:100%;border:0;border-bottom:1px solid var(--line);padding:12px 14px;font:500 14px Inter;background:transparent;color:var(--ink)}
.vpick .vr{display:grid;grid-template-columns:32px 120px 1fr auto;gap:10px;align-items:center;width:100%;padding:10px 14px;text-align:left;border:0;background:none;color:var(--ink)}
.vpick .vr.on,.vpick .vr:hover{background:var(--hover)}
.vpick .label{font:11px "JetBrains Mono",monospace;letter-spacing:.12em;color:var(--muted)}
.vpick .d{color:var(--muted);font-size:13px}
```

### A3. Blaze switches views by itself
Design and the other views open automatically when a task needs them, and Blaze asks first for big switches. An **Exit** button always returns to the basic chat.

Go tool:
```go
// open_view tool (register in internal/tools or app.go next to the other UI tools)
// {"view":"design|memory|jobs|schedule|accounts|doctor|settings|router","ask":bool,"reason":"..."}
func (a *App) toolOpenView(args struct{ View, Reason string; Ask bool }) (string, error) {
    if args.Ask {
        ok := a.askUser(fmt.Sprintf("Switch to %s? %s", strings.Title(args.View), args.Reason), []string{"Switch", "Stay in chat"})
        if ok != "Switch" { return "user stayed in chat", nil }
    }
    runtime.EventsEmit(a.ctx, "view:open", args.View)
    return "opened " + args.View, nil
}
```
- System prompt line: "When a task is visual (a website, design, 3D, poster), call open_view design without asking. For other views, ask first unless the user requested it."
- Frontend: `runtime.EventsOn('view:open', v => BlazeViews.VIEWS.find(x => x.id === v)?.open())`.
- While a view is open, the top bar shows a chip `DESIGN  ✕ Exit`. Exit calls `enterMode('chat')`.

### A4. What to change in index.html and app.js
- Remove `<aside id="side">` nav buttons and the `#tabs` Work/Design. Keep their handlers and point them at `window.showView` and `window.enterMode`.
- Move `#themeBtn` and `#route` into the avatar menu. The model picker is replaced by BlazeRouter.
- Add `#pill` (Chat | Projects), `#searchBtn`, `#routerDot` and `#avatar`.
- Update ci/mock.js and ci/shots.py with shots for chat light and dark, the /views picker, Design mode with Exit, the avatar menu and Projects mode.

---

## B. Long-chat engine: Blaze never re-reads everything

Goal: every turn stays under ~6k input tokens however long the chat is.

1. **Rolling summary (compaction).** When prompt tokens are over 60% of the model's context (or over 12k on free models), summarise every turn except the last 12 into a state note.
   - The note has five sections: Goal, Decisions, Facts, Open threads, Files touched. It is at most ~800 tokens.
   - Store it per chat and keep updating it rather than redoing it from scratch.
2. **Project brief.** A pinned brief per project (purpose, rules, key files, in progress), at most ~600 tokens. Blaze edits it with an `update_brief` tool and it is injected every turn.
3. **search_chat tool.** Every message goes into SQLite FTS5 (the memory-db work). `search_chat {query, limit}` returns exact past snippets with turn ids, and `read_turn {id}` returns one full turn. The prompt says: "If you need something older than the summary, search_chat. Never ask the user to repeat."
4. **Big outputs aside.** Any tool result over 4k chars is saved to `~/.blaze/artifacts/<id>.txt`. The model sees the first 40 lines, the last 20 and `artifact:<id> (N lines)`, and `read_artifact {id, from, lines}` reads a range on demand.
5. **Memory ranking.** Pinned memories plus the top 8 by FTS match on the current message (already planned in memory-db).
6. **Specialists** receive a brief and return a summary of 300 words or less. Their full logs stay in the agent card.
7. **Token meter** in the live card: `in 4.2k · out 0.6k`. Past 80% of the limit, Blaze offers "Start a fresh chat with a summary".

```go
// internal/llm/compact.go
package llm

type ChatState struct {
    Summary string // rolling state note
    Upto    int    // index of last turn folded into Summary
}

const keepTail = 12

// BuildContext returns the messages to send: system + brief + memories + summary + last N turns.
func BuildContext(sys, brief string, mems []string, st *ChatState, turns []Msg, limit int, summarise func(prev string, chunk []Msg) (string, error)) ([]Msg, error) {
    est := func(ms []Msg) (n int) { for _, m := range ms { n += len(m.Content) / 4 }; return }
    tailStart := max(0, len(turns)-keepTail)
    if est(turns[st.Upto:tailStart]) > 0 && est(turns) > limit*6/10 {
        s, err := summarise(st.Summary, turns[st.Upto:tailStart])
        if err != nil { return nil, err }
        st.Summary, st.Upto = s, tailStart
    }
    out := []Msg{{Role: "system", Content: sys}}
    if brief != "" { out = append(out, Msg{Role: "system", Content: "PROJECT BRIEF\n" + brief}) }
    if len(mems) > 0 { out = append(out, Msg{Role: "system", Content: "MEMORIES\n- " + strings.Join(mems, "\n- ")}) }
    if st.Summary != "" { out = append(out, Msg{Role: "system", Content: "EARLIER IN THIS CHAT (summary; use search_chat for exact text)\n" + st.Summary}) }
    return append(out, turns[max(st.Upto, tailStart):]...), nil
}

const SummarisePrompt = `Update the state note with the new turns. Keep sections: Goal, Decisions (with why), Facts (names, numbers, paths exactly), Open threads, Files touched. Max 800 tokens. Drop chit-chat. Never invent.`
```

```sql
-- in the memory-db schema
CREATE VIRTUAL TABLE IF NOT EXISTS chat_fts USING fts5(chat_id UNINDEXED, turn_id UNINDEXED, role UNINDEXED, content, tokenize='porter');
CREATE TABLE IF NOT EXISTS chat_state(chat_id TEXT PRIMARY KEY, summary TEXT, upto INTEGER);
CREATE TABLE IF NOT EXISTS project_brief(project_id TEXT PRIMARY KEY, brief TEXT, updated INTEGER);
```

Tests:
- A 300-turn chat stays under the budget.
- The summary keeps the facts from a seeded turn.
- search_chat finds turn 5.
- Artifact truncation and read-back work.

---

## C. Student pack (one PR; each item is a skill plus a view or job where needed)

1. **Classroom sync:** the Google connector with the `classroom.coursework.me.readonly` scope. Each assignment becomes a Schedule item with a reminder 24h before. Add the scope in Google Cloud project blaze-511220 (it is in Testing mode, so that's fine).
2. **Study planner:** a `study-planner` skill. It takes an exam date and topics, builds a day-by-day plan in Schedule, and re-plans on a missed day.
3. **Tutor mode:** a `tutor` skill. It explains, then quizzes, and turns a PDF or slides into flashcards. Spaced repetition lives in the Learn view (learn.js already exists, so extend it).
4. **Lecture notes:** audio through Whisper (stt.go), then notes, a summary and key terms.
5. **Essay coach:** outline, feedback and citations (APA or Harvard). It refuses to write full graded essays and offers coaching instead.
6. **Group project:** a project template with a tasks list and a "WhatsApp summary" button.
7. **Low-data mode:** a setting that prefers text models, skips screenshots and shows MB per task.

## D. Marketer and seller pack

1. **Content calendar:** a `content-calendar` skill that writes a week of posts. Visuals come from the Design studio using the brand kit (brand.js), stored as Schedule items.
2. **Shopify listing:** a `product-listing` skill. A supplier URL becomes a title, description, tags and image prompts.
3. **Ad lab:** 5 variants scored on hook, clarity and CTA.
4. **Research brief:** reuse the deep-research skill to produce a one-page competitor and price brief.
5. **WhatsApp replies:** drafted on desktop and sent through the phone link (Android DesktopLink), always with approval.
6. **Weekly digest:** a scheduled job covering orders and top posts, with suggested next posts.

---

## Handoff state (Oct 12)
- Desktop main ≈ v0.9.5+, Worker blaze-auth 0.8.0.
- `blazerouter` branch: a native Go port of OmniRoute (MIT; credit it in THIRD_PARTY_NOTICES).
  - Local OpenAI-compatible server on 127.0.0.1:20128 with `/dashboard` in the classic style.
  - Top-bar button; "BlazeRouter" in the model picker.
  - The in-app custom model/key settings are removed in favour of the dashboard, and existing keys migrate.
  - See that PR's checklist for what's left.
- `memory-db` branch (v0.5.1 SQLite FTS) may be stale against main. Rebase it or re-implement it inside part B.
- Blaze-cli `desktop-parity` branch: a WIP port of agents, cloud, connect and notify.
- Android: handled in the BlazeAssistant project.

## BlazeRouter: keep all of OmniRoute's auto/* combos (Luke, Oct 12)
PR #44 only ported auto, auto/fast, auto/cheap, auto/quota and auto/free. Port every OmniRoute auto/* preset with the same names and behaviour. Luke relies on them; auto/best-free is his daily driver.
auto/best-chaos auto/best-chat auto/best-coding auto/best-coding-fast auto/best-fast auto/best-free auto/best-reasoning auto/best-vision auto/chaos auto/chat auto/cheap auto/claude-opus auto/claude-sonnet auto/coding auto/fast auto/lkgp auto/multimodal auto/offline auto/pro-chat auto/pro-coding auto/pro-fast auto/pro-reasoning auto/pro-vision auto/quality auto/reasoning auto/smart auto/subscription auto/thrifty auto/vision 
Source files in OmniRoute: src/app/api/combos/auto/route.ts,src/app/api/combos/duplicate/route.ts,src/domain/assessment/types.ts
Expose them in /v1/models and the model picker as BlazeRouter · auto/<name>.
