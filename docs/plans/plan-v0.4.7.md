# v0.4.7: Background agents

Blaze works like this: the main chat hands long work to background agents and stays
free for chatting while they run.

## How it works

- **Dispatch.** The main agent has `dispatch_agent` (handle, self-contained brief,
  optional tool scope). It returns at once with `started` (or `queued`) and the turn
  ends straight away with one short line ("On it, I'll report back."). The composer
  is never locked.
- **Sub-agents.** Each runs in its own goroutine with its own history and its own
  shell (working folder), using the same tools, approval gate (Auto-approve),
  model router and daily budget. No nesting: sub-agents can't dispatch.
- **Cap and queue.** At most N run at once (default 3, 1 on a 2-thread machine;
  Settings › Behaviour › Background agents: 1, 2, 3, 4 or 6). Extra agents queue
  and start in order when a slot frees.
- **Results wake the main agent.** A finished agent's report goes into the
  dispatching chat as a platform message (`[Background agent result · handle · …]`)
  and starts a main turn that tells the user. Main turns run one at a time through
  a queue (`internal/agents.Mainline`), so each result wakes the main agent on its
  own, in arrival order, never bundled behind the slowest agent. A user message sent
  mid-answer queues the same way instead of being refused.
- **Steering.** `message_agent` delivers an instruction at the agent's next step;
  `cancel_agent` stops one; `list_agents` shows status. The prompt teaches: steer,
  don't duplicate; one agent per independent target.
- **Cards.** Each agent has a card in the chat: handle, task, the current plain step
  (steps.js wording, never a raw command), elapsed time and Stop. Finished cards
  collapse to "Done · N steps" and expand to the step log (same rows as the main
  chat) and the agent's report. Approvals and `ask_user` questions show on the card
  and block only that agent.
- **Restart.** Agents persist to `~/.blaze/desktop/agents/<handle>.json` after every
  step. Anything running when the app closed comes back as "Interrupted" with Resume
  (continues from the saved history) and Discard. Finished agents are pruned after
  7 days.
- **Orb.** The orb bar shows "2 agents" (+queued) while agents run.

## Code

- `internal/agents/` — the engine (no Wails): `Manager` (dispatch, cap/queue,
  steering, cancel, persistence, resume), `Mainline` (main turn queue), the main
  agent's tools and prompt blocks. Tests use a fake LLM.
- `bgagents.go` — wires it to the app: router, tools, per-agent env and approval
  cards, UI events (`agent`, `agent_step`, `agent_question`, `agent_removed`, `turn`).
- `frontend/dist/agents.js|css` — cards, approval/question on cards, orb indicator.

## Not yet

- Connector (Gmail, GitHub, MCP…) approvals from a sub-agent still show as a normal
  confirm card in the chat rather than on the agent's card.
- Autopilot's away-reply for sub-agent questions is a hook (`agentAway`) that lights
  up once composer-modes (#13) lands.
- A result for a chat that isn't open is filed into that chat (with a toast); the
  main agent replies to it only when it's open.
