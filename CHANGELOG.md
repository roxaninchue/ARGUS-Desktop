# Changelog

Release notes for Blaze Desktop, newest first. Design notes per release are in `docs/plans/`.

## v0.19.16 (see what Blaze is doing; know your message arrived; privacy)
- **A small working line under Blaze's reply** (`frontend/dist/pills.js`, `runEvents.js`, `internal/runevents`): when an answer takes more than a couple of seconds, one quiet pill under it says what Blaze is doing right now, in plain words ("Reading sales.csv", "Searching the web"), and changes in place as the work moves. The line always names what is really happening: "Reading cafe-sales-sept.csv", "Searching the web for Lusaka weather", "Charting revenue by store", "Writing your summary". When Blaze gets help from a specialty or a specialist, it says what the help is for in plain words ("Getting help to fix the build error"), never which one; a general line shows only when there is nothing more to say. It never shows commands, raw tool data or what powers the answer, it never sends a notification, and it disappears when the reply arrives. Tasks working in the background keep their pill under their "got it" message until the result lands.
- **Sending, Sent, Read under your latest message**: small dots show "Sending" until Blaze has your message, "Sent" once Blaze Cloud has stored it, and "Read" when Blaze really picks it up. If you're offline it stays at "Sending" and keeps trying by itself with the same message, so it is never sent twice.
- **Blaze is one assistant**: the footer, the model button, error cards, "What happened" and status lines no longer name the services or models behind an answer. Your replies are untouched.
- **About** in Settings: who makes Blaze (Blazeinchue) and which version you have. Blaze answers "who made you?" with Blazeinchue, and saying you own or made Blaze in a chat doesn't change anything; only a verified Blaze account can.
- Tests: `TestScrubNames`, `TestEventDropsProviderModel`, `TestSanitize`, `TestTrackerStubAndSpecialties`, `TestHelpLineFromDetail`, `TestFriendlyTool`, `TestCreateOfflineRetrySameID`, `TestCreateLostAnswerNoDuplicate`, `TestDeskOfflineRetrySameMessageNoDuplicate`, `TestEmitScrubsProviderAndModel`, `TestMakerIsBlazeinchueOnly`; node `ci/runevents.test.js`, `ci/workpills.test.js`; contrast checks for the pill and the status in light and dark.

## v0.19.15 (charts shown whole; symlinked project folders)
- **Charts and other single pictures in a reply are shown whole** (`frontend/dist/chat.css`): a picture on its own in a reply, like the chart from a CSV report, was squeezed into a 4:3 box and cut off at the sides, so the title and the first and last bars could be missing. It now keeps its own shape and is never cropped. Two or three pictures side by side and the stacked deck look the same as before. When a long reply (one with a table) is folded into a Details card, a chart on its own stays in view under the short answer instead of being folded away below the table.
- **Project folders that are shortcuts (symlinks) work** (`internal/tools`): if the folder you picked is a symlink to somewhere else, Blaze treated its own files as "outside this project" and asked before every write or edit. Files inside the folder you picked now count as inside, however the path is written. A link inside your folder that points somewhere outside it still asks first (before, it didn't). Approval cards show paths under the folder the way you picked it, not where the link leads.
- Tests: `TestReplySingleImageNotCropped`, `TestPickedSymlinkRootInside`, `TestSymlinkOutOfPickedFolderStillAsks`, `TestCardShowsPickedPath`; a headless screenshot check that a wide chart keeps its shape with its title and first and last bars visible.

## v0.19.14 (answers at once, works in the background; a new home; sync across devices; CSV reports)
- **Blaze says "got it" at once and works in the background** (`runs_app.go`, `internal/bgrun`): when you ask Blaze to do something, a short line like "Got it, setting that reminder for 9:00 tomorrow. I'll confirm when it's set." appears within a second and the message box is free again. The work keeps going (with Blaze Cloud even if you close the window), and the result arrives as its own message: "Done ✓" with what was done, only when the step on this computer really worked, or "Couldn't finish that" with the reason and what to try. Only requests to do something take this path: greetings, thanks, chat, questions, look-ups (reading or listing files, recalling memory), web searches and summaries get no "got it" line and stream their reply as before. The suggested next steps under a reply never end up in the "Done ✓" line or the notification. A request that mixes reading and doing ("read notes.txt, find the word, and create out.txt") counts as doing. If a task finishes while you're in the middle of another question in the same chat, its result is filed before that question, so the new answer isn't thrown off. You can send another message while a task is still running. Tasks still running when Blaze quit are picked up again the next time it opens. When a task finishes and Blaze isn't in front, you get exactly one notification; nothing while you're using it. Approval cards still appear for risky steps inside background tasks.
- **A new home with one chat box** (`home.js`, `home.css`): the orb, a greeting, one big box to type in (Enter sends), three starters, your recent chats, and your Home cards once you have some. Asking from Home opens Chat with the answer on its way.
- **Two-step setup** (`onboarding.go`, `onboard.js`): sign in, then optionally pick a folder. That's it. Setup from an older version resumes where it was.
- **Permissions only when they're needed** (`permissions.go`, `perms.js`): Blaze no longer asks for everything up front. It asks for a folder the first time a message is about your files, and offers notifications the first time a task takes a while, each with one line on why. Saying no is fine: Blaze tells you plainly what happens instead and keeps going. Approval cards for risky steps now say why in one line, and a declined step says it wasn't done. macOS no longer asks about notifications at launch.
- **Sync across devices, encrypted and opt-in** (`cloudsync_app.go`, `internal/cloudsync`, `cloudsync.js`): turn on "Sync across devices" in Settings > Account and choose a passphrase to keep your memory and chats the same on your computers and phone. Everything is encrypted on your device before it leaves; the passphrase is never stored or sent, only a key derived from it in your system keychain. Private chats and tool output stay on this computer. The status line says honestly when it last synced or what's wrong. Turning it off deletes nothing.
- **CSV report with a chart, PDF and summary** (`csv_app.go`, `internal/csvkit`, `csvkit.js`): give Blaze a CSV file and get a cleaned-up report: totals by group, a bar chart, notes on what was cleaned, a short written summary based only on the numbers, and a PDF and Markdown copy you can share.
- **"Done ✓" only when the work really happened** (`runs_app.go`, `internal/bgrun`): Blaze no longer shows "Done ✓" for a reply that only says it did something without using a tool, or for a reply that says it couldn't finish. Earlier "Done ✓" marks are no longer passed back to the model, so it can't copy them into a new answer.
- **CSV tools ready as soon as you name a CSV** (`internal/tools`): when your message mentions a .csv file, the CSV report and summary tools are switched on before Blaze starts, including for background tasks, so it can make the report instead of trying and failing to find the tool.
- **Faster "got it"** (`runs_app.go`): starting a task no longer waits on a model call to file it under a topic, so the "got it" line shows in about a fifth of a second.
- Tests: `TestNoToolClaimNeverDone`, `TestActivateForCSV`, `TestFailedReplyNeverDone`, `TestRunRequestSkipsTopicJudge`, `internal/bgrun` and `runs_app_test.go` (ack then Done ✓, no ack for greetings, `TestBgActionRouting`, `TestFollowupsNeverInSummaryOrNotification`, `TestDoneNotSaidTwice`, `TestDeskCloudActionWithoutGatewayAckStillAcks`, `TestFinalLandsBeforeRunningTurn`, device steps answered, failures honest, one notification only when not focused, resume after restart, a new message moves a run to the background, local models), `permissions_test.go`, `internal/cloudsync` and `cloudsync_app_test.go`, `internal/csvkit` and `csv_app_test.go`, `TestEveryToolSchemaIsStrictValid`.

## v0.19.13 (strict functionality pass: Blaze Cloud no longer "busy", Post Kit WebP, Blaze Cloud picker, no upstream names)
- **Fixed "Blaze Cloud is busy right now" on most messages** (`internal/agents/tools.go`): the list_agents tool was sent with `"required": null`, which strict upstream models reject with a 400 ("invalid JSON schema"), so a turn failed whenever Blaze Cloud picked one of them. Every tool now sends a valid schema, and a new test checks all of them.
- **Post kit takes WebP photos** (`internal/postkit/make.go`, `postkit_app.go`): .webp can be picked, dropped or pasted like JPEG and PNG (decoded with golang.org/x/image/webp), and exports crops as JPEG as before. The picker filter and the "unsupported photo" message now list WebP.
- **The model menu shows Blaze Cloud as one model** (`models.go`, `app.js`): the first row is always Blaze Cloud (blaze-auto). In Cloud mode nothing else from Blaze is offered (no BlazeRouter, Auto chain, or providers you have no key for); only your own providers (a saved key, Ollama, or your custom server) are listed below it, and picking one switches to Custom. Picking the Blaze Cloud row goes back to Cloud.
- **"What happened" no longer names the service behind Blaze Cloud** (`trace_app.go`): each Blaze Cloud attempt shows as Blaze Cloud · blaze-auto with its request id and timings; the upstream provider and model stay in the gateway's own logs, matched by request id.
- **Web search finds results again** (`internal/tools/search.go`, same backends as Blaze CLI 0.14.15): DuckDuckGo often answers with an empty page (rate limits, bot checks), so web_search said "No results." and Blaze gave up. Now DuckDuckGo with a short timeout, then Bing RSS, then DuckDuckGo Lite, and news questions ("latest", "news", "today"…) try Google News RSS first, with dates. When every source fails, the model is told plainly to read a known page instead.
- **News goes to web search, and gets searched again** (`weather_app.go`, `internal/tools/more.go`): "latest AI news" could call get_weather, and in the one continuous chat a repeated news question was answered by copying the earlier reply. get_weather is now described as weather only, web_search as the tool for news and anything "latest" or "today", and every turn tells the model to search news, prices and scores again rather than reuse an earlier answer.
- **/clear really starts fresh with topic memory on** (`internal/topics/assemble.go`): the active topic's state note was still sent after /clear even when it only covered turns from before it. It is now left out, as the rolling summary already was.
- **The live persona 1.0.2 loads again** (`internal/persona`): 1.0.2 writes some eval contexts as plain text, which 0.19.12 couldn't parse, so it kept using its bundled 1.0.0. Eval fields are now read as raw JSON, and 1.0.2 is bundled as the offline fallback, matching blazeinchue.pages.dev/persona.json.
- **Update flow test seams** (`update.go`): the platform and the running exe path are variables, so the real CheckUpdate + InstallUpdate path is exercised end to end for a Windows portable update on Linux CI.
- Tests: `TestPostKitWebPPhoto`, `TestExportWebP`, `TestModelsMenuCloudModeBlazeAutoOnly`, `TestTraceScrubsCloudUpstream`, `TestParsesPersona102Shapes`, `TestEveryToolSchemaIsStrictValid`, `TestWebSearchFallsBack`, `TestWebSearchAllDown`, `TestParseDDG`, `TestNewsRoutesToWebSearchNotWeather`, `TestClearDropsPreClearNoteAndTurns`, updated `TestModelsMenuNeverOffersKeyless`; the strict checklist (5 fresh runs per item plus a run with prior tool history) passed on this build.

## v0.19.12 (parallel tool calls, like Blaze CLI)
- **Reads run at the same time** (`internal/llm/batch.go`, `internal/tools/parallel.go`): when the model asks for several things in one reply, consecutive read-only calls (file reads, lists and searches, web_search, fetch_page, get_weather, recall, skills, job logs) now run together, up to 6 at once with a sliding limit instead of fixed groups of 6, the same as Blaze CLI's batchEnd/runBatch. Results go back to the model, the history and the screen in the order the model asked for them. Writes, edits, commands, git, installs, schedules, anything that asks for approval, and system_info still run one by one, in order, and end a run of reads. Plan mode blocks are unchanged. Background agents and specialists use the same path.
- **One slow or broken call no longer holds up the rest**: each read has its own 2 minute limit; a read that overruns is answered with a plain timeout error and its request is cancelled (web_search and fetch_page now stop when Stop is pressed or the limit is hit). A call that errors or crashes fails only itself, for writes too. Stop still ends a batch at once, and calls already running report "Stopped."
- Tests: `internal/llm/batch_test.go` (reads overlap and finish in about one call's time, order kept when later calls finish first, the 6-at-once cap, writes/commands/approvals never overlap anything and keep their order, per-call timeout with cancellation, no timeout on writes, error and panic isolation, Stop) and `internal/tools/parallel_test.go` (which tools may run in parallel).

## v0.19.11 (Post kit: a photo or an idea becomes a ready-to-post kit)
- **Post kit** (`postkit.js`, `postkit_app.go`, `internal/postkit`; open it from /views or /postkit): add a photo (pick, drop or paste) or type an idea, choose platforms (X, Instagram, TikTok, LinkedIn, Threads, YouTube Shorts, Reddit) and a language, and Make kit. You get one hook (12 words at most), exactly 3 hashtags, alt text (125 characters at most), a caption per platform within its limit (X counts every link as 23), a title for Reddit and YouTube Shorts, and crops for 1:1, 4:5, 9:16 and 16:9 centred on the subject. An idea without a photo gets a shot guide for a 1080 x 1920 frame. Copy any part, Copy all, Copy JSON, or Export: the kit JSON, a captions file and, with a photo, one cropped JPEG per ratio (the original is never changed). Open a kit checks one made by another Blaze app or the gateway tool.
- **Exactly blaze.postkit/1**, the shared schema Blaze Site publishes at blazeinchue.pages.dev/schemas/postkit-1.json (bundled in `internal/postkit`). The model writes the words through the normal chat path (Blaze Cloud or your own models; Private vision is respected, and the photo is sent only as a chat image). The rules run on this computer: captions are assembled and trimmed at a sentence or word boundary, every link, @handle, email, {placeholder} and `code` from your idea is kept, hashtags are cleaned, crops are computed locally, and the kit is validated before it's shown. A reply that doesn't fit is retried once with the problems spelled out. No provider or model is named anywhere, and Blaze never posts for you. Your own Blaze Cloud daily cap applies.
- Tests: `internal/postkit` (the validator against the schema file, every rule, strict parsing, trimming, tokens, crops, the idea path, the photo path with export, retry and fill, failures, every platform's limit, with a stubbed model), `postkit_app_test.go` (end to end through the app's chat call with a stubbed model: idea, photo attached as a chat image, export, plain errors, no provider or model names) and `ci/postkit.test.js` (the view end to end).

## v0.19.10 (writing tools: rewrite and translate selected text)
- **Select, then Rewrite or Translate** (`writing.js`, `writing_app.go`, `internal/writing`): select text in the message box or in any reply and a small bar offers Rewrite (Clearer, Shorter, Friendlier, More formal, More casual, Fix spelling and grammar, More confident, or your own words) and Translate (11 languages or any other). The result shows next to the original, and nothing changes until you Accept.
- **Accept, Undo, Reject**: in the message box, Accept replaces exactly the selected text, even if you typed elsewhere meanwhile, and Undo puts it back. From a reply, Accept copies the result to paste back into your document. Esc or Reject throws it away, and a late answer to a rejected request is ignored. To work on a document from another app, paste it into the message box, select, accept, and copy it back.
- **Reliable output**: a strict prompt, plus checks before anything is offered. Wrapper text like "Here's the rewritten text:", quotes and code fences are stripped. Your leading and trailing spaces are kept. Every link, email, `code`, {placeholder}, {{placeholder}}, %s and $VARIABLE must survive. A translation must actually change the text. A piece that fails is retried once with the problem spelled out; otherwise nothing changes and you can Try again. Long text is split on paragraph breaks and joined back exactly. Your own Blaze Cloud daily cap applies.
- Tests: `internal/writing` (split, clean, checks, prompts) and `ci/writing.test.js` (end to end: select, rewrite, accept, undo, moved text, errors, a rejected late answer, translate from a reply and copy).

## v0.19.9 (usage meter, no surprises)
- **Usage meter in the top bar** (`usage.js`, `usage_app.go`, `internal/cloudai/usage.go`): signed in with Blaze Cloud answering, a chip shows the messages left today. It turns amber at 20% or less and red when none are left, and gives one heads-up a day at 10%. Click it for messages and tokens used against the plan (or "no cap"), the last reply's tokens, the plan, and when it resets in local time. The numbers are exactly the gateway 0.7.0 contract Blaze CLI shows (`X-Blaze-Requests-Used/-Limit`, `X-RateLimit-Remaining-Requests`, `X-Blaze-Tokens-Used/-Limit/-Remaining`, `X-Blaze-Quota-Reset`, `X-Blaze-Plan`; this reply's tokens from `usage`). They update from every reply, including a 429, and from the free `GET /v1/usage`. Hidden in Custom mode.
- **Your own daily cap**: set a message cap in the meter, and Blaze stops before going over and tells you why, instead of quietly using more.
- **What happened** also notes the fast lane (`X-Blaze-Fast-Lane`) and fallbacks (`X-Blaze-Fallback`).
- Tests: `TestUsageMeter` and `ci/usage.test.js` (chip, levels, details, cap, hidden in Custom).

## v0.19.8 (readable bubbles, shared persona, gateway 0.6.9 speed contract)
- **Your chat bubble is readable in the light theme again** (`chat.css`): the text used `--paper`, which is translucent white in light (1.84:1 on the dark bubble). It now uses `--on-ink`, at 18.9:1 in light and 17.2:1 in dark. New guard `ci/contrast.test.js` checks the bubbles and body text against WCAG AA (4.5:1) in every theme from the real stylesheets.
- **One Blaze voice in every app** (`internal/persona`): the shared persona from blazeinchue.pages.dev/persona.json is sent as the FIRST system message, byte-identical every turn. It's fetched on start, cached and refetched at most daily, with v1.0.0 bundled as the offline fallback. Desktop's own instructions, the user's first name (when signed in) and the local time in natural form ("Tuesday, 20 October 2026, 7:15 PM", no zone IDs) go in a second system message.
- **Gateway 0.6.9 speed contract** (`internal/cloudai`): when signed in, warm-ups POST `/v1/warm` with the account token (on start, focus, typing; at most every 25 s); signed out they still use `/health`. Requests with tools send `"blaze_tools": "auto"`, so the gateway drops tool schemas on plain chat turns. **What happened** now also shows each request's `X-Request-Id`, gateway TTFT (`X-Blaze-TTFT-Ms`), `Server-Timing`, the fast lane, whether tools were dropped, the client's own time to first word, and the session's p50/p95 time to first word.
- Tests: `TestBundledAndRefresh` (persona), `TestPrewarm` (warm with token, /health without), `ci/whathappened.test.js` (new timing fields, p50/p95), `ci/contrast.test.js`.

## v0.19.7 (first word faster, What happened, show password)
- **Show / hide on every password field** (`pwreveal.js`): an eye button inside every password and passphrase field (sign-in, create account, reset, two-factor, keys and tokens, and any field added later). It switches between dots and plain text, and the field goes back to dots when the form is sent. Fields opt out with `data-noreveal`.
- **What happened** under each reply (`trace_app.go`, `whathappened.js`): the request ID, which is the `X-Blaze-Turn` the gateway logs; time to first word; the total time; each step's model, first word, time, tokens and how it ended; every Blaze Cloud request with its provider and model (`X-Blaze-Provider` / `X-Blaze-Model`), status, gateway first byte (`X-Blaze-TTFB-Ms`), hedged or replayed, and errors; and every tool with its time and any error. Closed by default.
- **First word faster** (`internal/cloudai`): HTTP/2 on the Blaze Cloud connection, so a turn's steps and the warm-up share one warm connection; idle connections are kept 5 minutes (was 90 s); streams ask for no compression, so a gzip layer can't hold back the first words; warm-ups can run every 25 s (was 60 s), also when the window becomes visible; and a keep-warm ping every 45 s while the window has focus and was used in the last 5 minutes. The first streamed word was already drawn as soon as it arrived.
- **Signed with the new update key** (`moDRRNAn…d4QI=`) from its GitLab CI/CD variable. v0.19.6 trusts it, so v0.19.6 updates in place. v0.19.5 and older, which only trust the previous key, need one manual download.
- Tests: `ci/whathappened.test.js` covers the eye toggles and the What happened panel.
- **No GitHub left in the release path**: the GitHub Actions workflows (`.github/workflows/build.yml`, `pr-shots.yml`) are removed. Releases are built in the sandbox from GitLab main, uploaded to the GitLab package registry, and published on blazeinchue.pages.dev. The "update Blaze" tool and the README no longer say updates come from GitHub. The updater already read only pages.dev (`latest/SHA256SUMS`, `versions.json`). Sign in with GitHub and the GitHub connector are user features and stay.

- **Weather from one source** (gateway 0.6.8 `/v1/weather`, `weather_app.go`, `internal/cloudai/weather.go`): a `get_weather` tool (lat/lon, a place name, or nothing for this PC's approximate location) and a system line that says weather only ever comes from it, never from web search or a guess. If both sources are down it says weather is unavailable. The Home weather card shows the same Blaze Cloud reading (with its source and time) when you're signed in; the days ahead still come from the forecast. Test: `TestWeather`.

## v0.19.6 (signing key rotation, step 1)
- **New release-signing key** (`internal/updsig`): `moDRRNAnYCRxRjblqUI1oDlGgfRI13NumGA4sLrd4QI=`. Its private half lives only as a masked, protected CI/CD variable on the GitLab project; the release build reads it from there and it's never printed.
- **Smooth hand-over**: v0.19.6 trusts both the new key and the previous one (`h+BLX1pI…MFo=`), and is itself signed with the previous key, so v0.19.2 to v0.19.5 update into it as usual. The next release is signed with the new key only and drops the previous one. Test: `TestRotationTrustsBoth`.

## v0.19.5 (profile badge, like the website)
- **Plan pill on your avatar**: a small Basic, Standard or Pro pill sits centred on the bottom edge of the round top-bar avatar, in the theme's colours (Pro filled with the accent). The plan comes from Blaze Cloud (`GET /v1/billing/me`, `term.plan` else `plan`; `internal/cloudai/plan.go`, `AccountPlan`, cached 10 minutes). It's hidden when signed out or offline.
- **Account menu**: clicking the avatar shows your picture or initial, name, email and plan, then Upgrade (Basic and Standard; opens the plans on the website), Add / Change picture, Remove picture, the usual rows, and Sign out.
- Tests: `TestPlan`, `ci/avatar.test.js` (initial and colour match the website's hash, pill follows the plan, menu rows, picture URL, signed-out reset).

## v0.19.4 (profile pictures, blaze-auth 0.9.3)
- **Your picture in the top bar**: signed in, the avatar button shows your profile picture, or the first letter of your name (else your email) on a colour picked from it, the same letter and colours as the website.
- **Add, change or remove it** in Settings › Account. Blaze centre-crops the picture to a 256 px square and compresses it (WebP, under 256 KB) before uploading it to `POST /v1/me/avatar`. Remove goes back to the letter (`POST /v1/me/avatar/delete`). PNG, JPEG and WebP only.
- `internal/cloud`: `AvatarUpload` (raw image body, type and size checked before sending) and `AvatarDelete`; test `TestAvatarUpload`. Not tried against the live service from a real desktop yet.

## v0.19.3 (tells you when an update is out)
- **"Update available" in the top bar** (`app.js` `updPill`): the signed updater already checked the downloads site 3 seconds after launch, but its only signs were a card in the chat and a hidden badge. Now a pill with an orange dot sits in the top bar on every view until you update or skip that version; clicking it opens the Update card (Update now / Later / Skip this version).
- **Checks again once a day** while Blaze stays open. Offline checks fail quietly. Settings › Updates › "Check on launch" turns both off.
- **No false alarms** (`update.go` `hasOSAsset`): a release with no file for your system (for example a Windows-only patch) isn't offered as an update. Test: `TestHasOSAsset`.

## v0.19.2 (new update signing key)
- **Release-signing key rotated** (`internal/updsig`): the private key behind the old public key `A/yWqdv1…ctUA=` was lost with the old CI, so v0.19.2 trusts a new key, `h+BLX1pI7e4k9Xti6+dRn8tAz+kfZ+GgKqmoiUNkMFo=`. Releases are built and signed outside CI now; the private key sits in the owner's secret vault and never in the repo.
- **One manual reinstall**: v0.15.1 to v0.19.1 can't verify releases signed with the new key, so they say the update isn't signed and install nothing. Download v0.19.2 once from https://blazeinchue.pages.dev; in-app updates work again from there.

## v0.19.1 (faster first word, from Blaze Mobile 0.45.1)
Speed only; nothing else changes. Not yet measured against Blaze Cloud from a real desktop.
- **System prompt ordered for prompt caching** (`speed.go`, `app.go` `systemStable` / `systemLive`): AI services reuse their work for the longest identical start of a request. The stable parts (rules, about Blaze, personality, design rules, project, PC profile, connected accounts, skills for the request, learning rules) now come first, then the constant delegation, orchestration and long-chat blocks (they used to come after the per-step MORE TOOLS line), then the live part (current folder, memory, clock), then MORE TOOLS, the long-chat summary and the note about the last reply. No wording changed; only the order.
- **Tools keep their place across a turn's steps** (`keepToolOrder`): a tool that joins mid-turn goes after the ones already sent instead of in the middle. The same tools are sent, nothing added or dropped.
- **One route per turn** (`internal/cloudai/turn.go`): every Blaze Cloud request of a chat turn carries `X-Blaze-Turn`, and from the second step on `X-Blaze-Prefer: provider/model` with the route that answered the first step. Blaze Gateway 0.6.1 reads it and keeps the turn on that route while it is healthy, which also keeps that service's prompt cache warm. A gateway that doesn't read the headers ignores them.
- **Connection opened early** (`internal/cloudai/prewarm.go`, `CloudPrewarm`): at start-up, on the first key press in the composer and when the window comes back to the front, at most once a minute, Blaze does a GET of the gateway's public `/health` (no account token, no model call, no quota) on the same keep-alive client, so DNS and TLS are done before Send.
- Tests: `TestTurnHeadersAcrossSteps`, `TestHeaderSafe`, `TestPrewarm` (the chat request reuses the warmed connection), `TestKeepToolOrder`, `TestPromptOrderSharesLongerStart`.
- **Updates come from the Blaze downloads site, not GitHub** (`update.go`): Blaze reads `https://blazeinchue.pages.dev/dl/desktop/latest/SHA256SUMS`, takes the version from the asset names and downloads from that version's folder. The signed-SHA256SUMS check is unchanged, and no GitHub token is sent anywhere. Buttons say "Open downloads page". Override the source with `-X main.updateBase=...`. Test: `TestReleaseFromSums`; checked live against the site's v0.18.0 (download + signature OK).
- **No GitHub links in the app**: the request Referer, the media User-Agent, Blaze's notes about itself and the Arch package URL now point at `https://blazeinchue.pages.dev`.
- Removed an outside product name from code comments (accounts.js, agents.go) and old plan docs.

## v0.19.0 (one chat, Home, Blaze Cloud, Music and Video)
- **One endless chat**: the chats sidebar is gone. Chat is one conversation that loads older messages as you scroll up; your earlier chats sit further back under "Earlier chat" dividers, and Ctrl K searches all of it. Nothing in `~/.blaze` was moved or deleted.
- **Topic-aware memory**: Blaze follows which topic you're on. Going back to an earlier topic brings back that topic's notes and messages and leaves unrelated ones out, while deadlines, decisions, preferences and anything you call important carry across. A quiet topic chip above the composer shows where you are. Every message stays searchable.
- **Home tab**: Home | Chat | Projects tabs. Home has a greeting, a quick ask box and cards for weather, news, prices, sports, GitHub activity, your schedule, deadlines and "pick up where you left off". Add cards in plain words.
- **Blaze Cloud**: every model call goes through Blaze Cloud with your Blaze account by default, picking the model per task like Blaze Mobile, with clear messages for signed out, waiting list or busy.
- **Custom models**: Settings > Models lets you use your own providers and keys instead, optionally pin a model for chat, code or vision, and choose whether Blaze Cloud is a backup.
- **Music**: your music folders as a library with covers, a player with queue, shuffle, repeat, EQ, sleep timer and lyrics, a mini player on every tab, playlists, mixes, Wrapped and the Store (YouTube, Jamendo, Internet Archive). Ask Blaze to "play something chill".
- **Video**: library, Continue watching, series, subtitles, resume, speed, skip intro, up next, recap and translate.
- Design notes: `docs/plans/plan-v0.19.md`.

## v0.18.2 (the mobile design on the glass look)
The v0.17.1 mobile design (PR #54), merged onto the glass look (v0.18.0 + v0.18.1). Glass keeps the palette, the frosted surfaces and the orb + halo mark; the mobile design brings its chat, composer, rich replies and the no-square guard. `round.css` keeps v0.17.1's rules first and v0.18.1's radii last, so v0.18.1 wins where both set a radius.
- **Kept from v0.18.0**: the dark-first monochrome glass tokens, the ground gradient, `glass.css` / `motion.css`, and the orb + halo `logo.svg` (it replaces v0.17.1's flame mascot, which is the older mark).
- **Kept from v0.17.1**: messaging bubbles (`bubbles.js`, `chat.css`), follow-up pills, the pill composer and rotating placeholder, the empty state, rich replies (step-flow, pictures, deck, mermaid), `round.css` and `ci/nosquare.test.js` / `ci/design.test.js`.
- **Reconciled tokens**: v0.17.1's surface tokens now map to glass: `--primary` is the monochrome ink fill (no classic blue), `--bubble`, `--inset`, `--field` and `--field-line` are translucent glass fills. Light theme accent is `#18181B` with white text on it, so pills and labels stay readable.

## v0.18.1 (finish the glass look)
- **The real emblem**: the drop in its gold halo is now drawn exactly like Blaze Mobile, breathing and wobbling while Blaze works and still when it's done. New app and tray icons.
- **The drop rides your reply**: while Blaze streams, the drop follows the end of the text, falls onto code blocks, tables and cards with a little splash, and glides home when the reply ends. The newest words glow blue, and a soft sweep crosses the reply when it finishes. The blinking caret is gone.
- **Thinking marks**: every square status mark (thinking, steps, work cards, agents) is now a small live emblem. The old particle sprites are gone.
- **Edge particles only while working**: a faint ring of dots travels round the window edge during a run and disappears when it ends. Fixes the rows of still dots v0.18.0 painted across the window.
- **Rounded everywhere**: composer, buttons, cards, code chips, status dots, checkboxes and menus. CI now fails if a control comes back square.
- **Light theme fixed**: your own messages are readable again, accent text and selected pills have proper contrast, and menus are frosted instead of see-through.
- **Progress shimmer** on the update and voice-model downloads, and skeletons instead of "Loading…".
- **Reduce motion** now also covers the memory graph, onboarding, chat titles and the big orb.
- **Changelog restored**: v0.18.0 had dropped every older entry.

## v0.18.0 (glass)
- **Dark frosted glass shell**: the desktop app now uses the same glass language as Blaze Mobile, with rounded cards, softer edges, and a translucent layer over the main surfaces.
- **Monochrome surfaces and gold halo emblem**: the app keeps a neutral grey palette, leaving semantic colours for warnings and errors while the brand orb keeps its gold halo accent.
- **Streamed reply motion**: new messages and the live drop effect use the same timing and reduced-motion rules as the mobile design.
- **System-aware light/dark theme**: the theme still follows the OS by default, while the appearance settings let you force light or dark and switch off motion.

## v0.17.1 (the mobile design, never released on its own; shipped in v0.18.2)
Request: "Make the desktop version have the mobile version's new design." Blaze Desktop now looks like Blaze for Android v0.24.0–v0.25.2, in light and dark; every desktop feature (chat history, projects, views, Design, shortcuts) stays.
- **No square UI** (Android v0.24.8): radius tokens `--r-xs` 8, `--r-sm` 12, `--r-md` 16, `--r-lg` 24 and `--r-pill` in `style.css`, used by every button, input, card, menu, popover, dialog, sidebar row, the Chat | Projects toggle, tabs, Design / Studio panels and Settings (`round.css`). The old global `* { border-radius: 0 }` reset is gone, status marks are round dots, icons have round caps. **`ci/nosquare.test.js`** (like Android's `NoSquareTest`) fails CI on a zero radius in any stylesheet or inline in a script, a global radius reset, square icon caps, or one of 50 core components losing its radius; it runs in the Linux build and the PR checks.
- **Colours from mobile's `Theme.kt`**: light paper `#FCFCFC` / ink `#111110` / accent `#1F4BD8`, dark paper `#26262A` / ink `#F0F0EE` / accent `#6B8CFF`, plus Blaze's bubble, inset and field fills. Primary buttons are the solid classic Blaze blue `#4C7DFF` (Android v0.25.2), no gradient.
- **Brand mark**: Blaze's mascot in a dark circle (`logo.svg`, ported from Android's `phone/Mascot.kt` vectors: the flame body, orange-to-blue gradient, cyan highlight, eyes and glow), replacing the pixel "B".
- **Messaging-style chat** (Android v0.24.0, `bubbles.js`, `chat.css`): your messages are ink bubbles on the right, Blaze's are soft bubbles on the left with code, tables and answer cards inset; consecutive messages from one side group 3px apart (16px between groups, a 10-minute pause starts a new group) with asymmetric corners; one quiet mono timestamp with **Copy · Quote · Retry** closes each group (the route stays as a quiet chip). The work card is a soft rounded strip (accent wash while live), the step list a rounded inset with round marks, confirm and Long-chat cards read as Blaze's newest message with pill buttons, errors are a Blaze bubble with an accent wash, and a round **jump to newest** button appears when you scroll up.
- **Follow-up pills**: Blaze ends a final answer with `<<follow-ups: a | b | c>>` (new RICH REPLIES / FOLLOW-UPS prompt block, `replyExtras` in `app.go`); the app hides the marker and shows three rounded accent pills under the newest answer. Tap one to send it.
- **Pill composer** (Android v0.24.1): a fully rounded pill one step off the background with a 1px outline, a thin **+** on the left (turns into × with the menu open), the mic on the right that becomes a round **send arrow** when there's text and a **stop square** while Blaze works. Long or multi-line messages keep rounded ends. The mode chip, `?` shortcuts, recording line and Plan / Autopilot outline stay.
- **Rotating placeholder**: every ~3 s the hint fades and slides to the next suggestion, built only from this computer (connected accounts, recent projects, the open folder) plus generic fallbacks; no network or model calls. It stops while the field is focused or has text, while recording, in Plan / Autopilot (their own text) and with reduced motion.
- **Empty state**: mobile's greeting ("Good afternoon, <first name>." from your Blaze account), the mascot, and the four tries as rounded cards.
- **Rich replies** (Android v0.24.9, `md.js`): a numbered list whose items start with a **bold title** is a step-flow (numbered blue dots joined by a line); a picture on its own line before a paragraph sits beside it, 2–3 pictures sit side by side, 4 or more become a stacked, slightly tilted deck with arrows (instant with reduced motion); ```mermaid blocks are drawn by mermaid.ink with the source one click away. Only https pictures load; Low-data mode keeps pictures and diagrams as text. (Kroki diagrams and the card-flip animation were left out to keep it light.)
- Tests: `ci/nosquare.test.js`, `ci/design.test.js` (grouping, corners, timestamps, meta actions, follow-ups, composer states, hints, step-flow, pictures, deck, mermaid), `TestReplyExtras`; shots `design-*` in both themes (`ci/design_shots.py`, also run by `ci/shots.py`). Design notes: `docs/plans/plan-mobile-design.md`.

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
- **ABOUT YOU**: every chat now starts with a short, live block telling Blaze what it is: Blaze Desktop and its version, made by Blazeinchue, the three apps (Desktop, CLI, Android) and how they connect, BlazeRouter on/off, Blaze Link status, the Blaze account and this PC (OS, desktop/Wayland, CPU, RAM). About 300 tokens.
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
