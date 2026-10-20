# Desktop v0.4.1 plan: Answer cards
The short answer is what you read; long detail folds into an expandable card above it, like chat cards elsewhere, in Blaze's look.

01 · The model shapes its reply

The system prompt (app.go, `answerShape`) asks for a short direct answer first (1-4 sentences) and puts long material (tables, step lists, long explanations, code over ~15 lines, reports, comparisons) in card blocks:

    :::card Title
    ...Markdown...
    :::

Several cards are fine; short replies need none. The block is only asked for while the setting is on. Scheduled jobs (main.go) keep their plain prompt, since their output goes to a log.

02 · The app splits it (md.js `MD.answer`)

- `:::card Title` … `:::` blocks come out of the reply, ignoring markers inside code fences; a missing title is "Details", stray closers are dropped.
- Fallback when the model ignores the format: on `done`, a reply with no cards that is long (over 900 characters or 14 lines) or has a table keeps its opening (the text before the first table/heading/code block, or just the first paragraph) as the short answer and moves the rest into one card titled from its first heading, else "Details" (a lone code block is "Code · lang"). A closing question ("Want me to commit it?") stays with the short answer.
- Cards off: the markers turn into a bold title line and the content shows inline, so older chats still read well.

03 · The card

Cards render above the short answer. Square edges, 1px border, the raised surface, a blue mark and the title in Inter Tight 600. Collapsed at 260px (220px in the orb) with an 84px fade into the card colour; the corner expand/collapse icon top-right and a small square chevron centred on the bottom edge. Title, icon and chevron all toggle with a smooth height animation; they are buttons with aria-expanded. Cards that fit show in full with no fade or buttons (a ResizeObserver re-checks as content streams). Wide tables scroll sideways (every Markdown table now sits in `.tbl`). A copy button copies the card's Markdown.

04 · Streaming, saved chats, orb

While text streams, an open `:::card` block fills its card live (its mark pulses) and half-typed markers are hidden; the split is finalised on `done` (also on stop and error). Saved chats (loadConv) render the same way, and the orb shows cards full-width. Expanded/collapsed state survives re-renders during streaming.

05 · Setting

Settings › Behaviour › "Fold long answers into cards" (desktop.json `fold_cards`, default on, always explicit in State()).

06 · CI

ci/mock.js has the roadmap reply and a long untagged reply; ci/shots.py shoots card-collapsed, card-expanded (+ -end), card-streaming, card-auto and card-orb in light and dark, and fails if the fallback reply doesn't fold.

Test on a real PC: ask for the roadmap table, a long how-to, and a one-line question; expand/collapse with mouse and keyboard; turn the setting off and reopen a saved chat.
