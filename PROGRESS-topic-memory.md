# PROGRESS — v0.19 workstream B: topic-aware infinite-chat memory

Branch `feat/v019-topic-memory` (clone at /workspace/work/ws-memory), based on local
`fix/glass-v0.18.1` @ 9d9a246. The brief names 7a5d3fc as the base, but that commit doesn't
exist in bd-glass; 9d9a246 is that branch's head.

## What the user asked
"Infinite chat and Blaze still remembers everything … hi → pizza → make a rocket: ignore the
pizza chat; back to pizza: ignore the rocket as if it never existed, unless something important
about the rocket needs addressing."

## Done
- **internal/topics** (pure Go, no DB, depends only on llm.Message)
  - `segment.go`: tokenizer, small stemmer, stoplist; `IsSmallTalk`; explicit cues
    (`back to the X`, `about that X`, `what about X`, `regarding X` → return; `new topic`,
    `different question`, `switching gears` … → new); a lexical score per topic (keyword bag:
    user mention = 1, assistant mention = 0.4; idf-like weights; a word every topic uses (3+
    topics) weighs 0.3). `Decide()`, in order:
    - small talk → no topic;
    - no content words → same topic;
    - an older topic scores ≥0.4 and beats the active one by 0.15 → return to it;
    - overlap with the active topic ≥0.15 → same;
    - an older topic scores ≥0.25 → return to it;
    - no overlap anywhere: an anaphoric short message stays on the active topic; anything else
      goes to the optional `Judge` (one small model call); without a judge it opens a new topic.

    Heuristic titles: "make a pizza" → Pizza, "build a model rocket" → Model rocket.
  - `state.go`: `State` holds the topics, a turn→topic map, the active topic, a high-water mark
    and the carry list. `Segment()` is incremental: each agent step reads only new messages, and
    the judge is used only from `JudgeFrom` (the latest user message), so old history is re-read
    without model calls. `Truncate()` handles retries and edits. When a topic is left,
    `ExtractCarry` pulls out carry-over lines: deadlines, decisions, preferences, "important /
    don't forget / remember that", and the assistant's promises.
  - `assemble.go`: `Assemble()` sends the active topic's turns that the note doesn't cover yet
    (whole turns, newest first within the budget). It always sends the current turn, keeps small
    talk inside the active topic's flow, and drops small talk at the start of the chat. It adds
    this topic text:
    - the `TOPICS` rule;
    - the ACTIVE TOPIC header and its STATE NOTE;
    - CARRIED OVER FROM OTHER TOPICS (8 lines at most);
    - OTHER TOPICS IN THIS CHAT (12 one-line entries at most);
    - CHAT OPENING, when the chat starts with an assistant hand-over (FreshChat).

    The topic text is capped at half the budget. `FoldChunk`, `SetNote` and `HeuristicNote` keep
    a rolling note per topic (Goal, Decisions, Facts, Open threads). The note is updated in
    batches (when there are 12 tail messages + 6 slack, or the topic passes 60% of the budget)
    and is never rewritten from scratch. `View()` feeds the frontend.
  - `parse.go`: builds the judge input and parses the judge and carry-pass output. Prompts:
    `Rule`, `NotePrompt`, `JudgePrompt`, `CarryPrompt`.
- **internal/longchat/topics.go**: new tables `chat_topics(chat_id, state JSON, updated)` and
  `longchat_meta(key,value)` in longchat.db, created with the existing schema
  (`CREATE … IF NOT EXISTS`). Methods: `Topics`, `SetTopics`, `ForgetTopics`, `MetaValue`,
  `SetMetaValue`. chat_fts, chat_state and project_brief are untouched, so every turn stays in
  the FTS index (search_chat / read_turn).
- **topics_app.go** (main): `topicContext()` does, per step:
  - segments new messages (judge = `llmQuick` "quick" model, 15 s cap);
  - folds the active topic's note with the chat model (heuristic note if that fails);
  - saves carry-over: the heuristic pass runs right away; a model pass runs in the background
    when a topic is left;
  - saves carry lines of kind deadline/decision/preference/promise to the memory table via
    `mindSave`, so they show up in every chat and reach CLI through the blaze.db mirror;
  - emits the `topic` event.

  Bindings: `ChatTopics() topics.View`, `TopicMemoryOn()`, `SetTopicMemory(on)`. The setting
  lives in longchat_meta and defaults to on; off brings back the v0.18 rolling-summary path.
  Any failure also falls back to the v0.18 path.
- **Frontend**: `frontend/dist/topics.js|css` adds a chip ("TOPIC Pizza +2") above the composer.
  Clicking it lists the topics; picking one starts the message with "Back to the <topic>: ".
  It mounts itself: it wraps the global `loadConv`, listens to `topic` and `done`, and a
  MutationObserver keeps it in the chat pane when the pane moves. It uses the glass and radius
  tokens.
- **Tests**
  - `internal/topics/topics_test.go`:
    - the user's scenario, exactly as given (asserted in detail below);
    - return to pizza by keywords alone (no cue);
    - while on the rocket, no pizza turns are sent and nothing carries over;
    - small talk and follow-ups;
    - the new-topic cue, and the judge being called exactly once;
    - incremental reading gives the same result as a full read; JSON round trip; truncate;
    - carry extraction;
    - token budgets of 2k, 6k and 12k on an 8-topic chat of over 40k tokens;
    - the topic text stays ≤ half the budget with 60 topics;
    - the fold extends the note instead of redoing it.

    The scenario test: hi → 3 pizza turns → 3 rocket turns (including "Important … by Friday 24
    October … deadline") → "Ok back to the pizza…". It asserts the context has the pizza turns,
    no rocket turns and no "hi"; that the deadline is carried over; and that the rocket topic is
    listed in the index.
  - `internal/longchat/topics_test.go`: a v0.18 DB opens with its data intact and gains the new
    tables; running the v0.18 schema again on the new file is fine.
  - `topics_app_test.go`: the scenario through the real `longContext`, turn by turn, with a
    stubbed quick model. It checks for exactly one judge call, that the rocket turns are still in
    the FTS index, the chip view, and that switching off restores the v0.18 behaviour.
- ci/mock.js: `ChatTopics`, `TopicMemoryOn`, `SetTopicMemory`. ci/shots.py adds
  `topic-chip-{light,dark}` and `topic-list-{light,dark}` with assertions.

## Edits to shared files (small)
- `longchat_app.go`: +5 lines (`all := hist`; call `topicContext` before the v0.18 summary path).
- `app.go`: +1 line (`d.ForgetTopics(id)` in DeleteConversation).
- `internal/longchat/db.go`: 1 line (`schema + topicSchema` in Open).
- `frontend/dist/index.html`: +2 lines (topics.css, topics.js).
- `ci/mock.js`, `ci/shots.py`: additive blocks only.

## How to run
```
export PATH=/workspace/work/toolchain/go/bin:$PATH   # Go 1.26.9 linux/arm64
CGO_ENABLED=1 go vet -tags webkit2_41 ./... && CGO_ENABLED=1 go test -tags webkit2_41 ./...
go test ./internal/topics/ -run Scenario -v
NODE_PATH=/workspace/work/toolchain/node_modules node ci/picker.test.js   # jsdom lives there
CHROME=/usr/bin/chromium python3 ci/shots.py
```

## Known limits / left to do
- Lexical matching can't tell that "toppings on a margherita" is about pizza when no word
  overlaps. That case goes to the judge (one quick call). With no model available, it opens a new
  topic; the model still has the topic index and search_chat.
- Titles are heuristic (the judge names the topics it opens). There is no rename, merge or split
  UI yet.
- There is no Settings toggle for `SetTopicMemory` (another workstream owns Settings); the
  binding exists.

## CLI follow-up (Blaze CLI)
- CLI needs no change to keep working: only new tables were added, nothing was renamed.
- CLI would benefit from the same behaviour. It could port `internal/topics` (pure Go) and
  read/write `chat_topics` for its long sessions. Carry lines already reach CLI through the
  mind → blaze.db mirror.
- Mobile (LongChat.kt) could mirror the same algorithm. The thresholds and prompts are constants
  in segment.go and assemble.go.
