# v0.19 Workstream D: Music and Video (branch `feat/v019-media`)

Ported Blaze Mobile's Music and Video apps (`mediaapps/`, `storeapi/`, mobile v0.34.0) to the desktop.
Calls aren't part of this workstream; they're handled elsewhere.

## Layout

| Part | Where |
|---|---|
| Service (library, scan, tags, covers, prefs, taste, smarts, lyrics, subtitles, Store sources, downloads, file server, agent ops) | `internal/media/*.go` (new package) + `internal/media/media_test.go` |
| Window bindings (`Media*` methods), `media:*` events, agent tools, asset handler | `media_app.go` (new) |
| Player rules (queue, shuffle, repeat, fades, lyric line, resume, skip intro, episodes) | `frontend/dist/media-logic.js` (new; node-tested in `ci/media.test.js`) |
| Music view, the global `<audio>` player, mini player, entry points | `frontend/dist/music.js`, `music.css` (new) |
| Video view and player | `frontend/dist/video.js`, `video.css` (new) |
| Store and Downloads (shared by both) | `frontend/dist/media-store.js` (new) |
| Shots | `ci/media_shots.py` + `ci/media-mock.js` (new), one call from `ci/shots.py` |

## Entry points (for the Home | Chat | Projects shell rebuild)

Everything goes through small hooks, so the new shell can move them:

- **Views:** `window.VIEWS.music` and `window.VIEWS.video`, so `show("music")` and `show("video")` work. Both take an optional arg: `show("music","store")`, `show("music","library:playlists")`, `show("video","downloads")`.
- **Views registry:** `BlazeViews.LIST` gets `music` and `video`, so they appear in `/views`, Ctrl+K and the Exit chip label. Icons are added as `IC.music` and `IC.film`.
- **Home hook:** `window.BlazeApps.list` is a registry of `{id, name, icon (svg path), desc, open()}` holding Music and Video. It fires a `blazeapps` window event on change, so a Home screen can draw tiles from it.
- **Slash commands:** `/music` and `/video` are pushed into `COMMANDS`.
- **Top bar:** `#mediaBtns` is a small round ♪/▶ pair inserted before `#searchBtn` (falls back to `#routerBtn`/`#themeBtn`, then the end of `#top`). The new shell can delete it if Home carries the tiles.
- **Mini player:** `#mzMini` is fixed bottom-left over the sidebar (202 px wide) and hidden inside Music. It watches `#view` with a MutationObserver, so it needs no hook in `show()`. If the new shell moves the sidebar, reposition `.mzmini` in `music.css`.
- **Agent:** `open_view` accepts `music` and `video` (`view_app.go` `openableViews`).
- **Window events for other UI:** `blazemedia:player` (detail = player state), `blazemedia:library` and `blazemedia:downloads`.
- **JS API:** `BlazeMusic.{init, playList, playKeys, toggle, next, prev, state, cur, lib}` and `BlazeVideo.{play, pause, active, cmd}`.

## Go side

- `media.New(~/.blaze/media)` loads everything; `mediaGet()` in `media_app.go` starts it lazily on first use, so `app.go` needs no startup hook.
- **Scan:** `~/Music`, plus `~/Videos` (or `~/Movies` on macOS), plus any folders the user adds, plus Blaze's download folders.
  - Walks up to 8 levels deep and 30 000 files, skipping hidden folders.
  - Tags and embedded covers come from `github.com/dhowden/tag` (now a direct dependency). Falls back to a folder `cover.jpg`/`folder.jpg`, or `<video>.jpg` / `poster.jpg`.
  - Unchanged files (same size and mtime) come from the `scan.json` cache.
  - Durations are learned from the player (`MediaDuration`), because tag readers don't give them.
- **Serving** (`serve.go`):
  - Routes: `/media/file?k=`, `/media/art?k=`, `/media/sub?v=&id=` (sidecar `.srt` is converted to VTT) and `/media/s/<id>` (stream proxy with Range and the source's user-agent).
  - Only keys the library knows are served, never an arbitrary path.
  - Mounted on two servers:
    - the Wails asset server (`main.go`: `AssetServer.Handler: mediaHandler()`, a one-line change)
    - a loopback server (`127.0.0.1:random` with a random `?t=` token). The player prefers this one, because WebKitGTK/WKWebView stream big `<video>` more reliably over http. CORS `*` lets WebAudio (EQ/fades) hear cross-origin media.
- **Downloads:** a queue saved in `downloads.json`.
  - Fetches 8 MB ranged parts and resumes from `.part` files.
  - Backoff runs 2 s to 5 min over up to 8 tries. Expired or refused URLs (403/410/`expire=`) are refreshed from the source.
  - Supports pause, resume, cancel and dismiss, and checks free space first.
  - Finished files go to `~/Music/Blaze/Music` and `~/Videos/Blaze/Videos` (`~/Movies` on macOS).
  - ffmpeg merges YouTube video+audio and remuxes DASH audio. Without ffmpeg, video offers only YouTube's one-file qualities, and audio is kept as sent (m4a/webm).
- **Store sources** (same as mobile):
  - YouTube: InnerTube WEB search; streams from VISIONOS → ANDROID_VR → ANDROID → IOS, each probed 1 KB at 1.5 MB.
  - Jamendo: user client_id, `jamendo_setup`, or `JAMENDO_CLIENT_ID`; nothing hard-coded.
  - Internet Archive audio (CC/PD only), public-domain feature films, Blender open movies (fixed id list).
  - Download by link: YouTube links, direct files, og:video pages.
- **Agent tools** (`tools.Register` in `media_app.go`; all non-core, so they're reachable through find_tools or by exact name):
  - `music_play` (new on the desktop): play/queue by title, artist, playlist or mood (e.g. "play something chill"), plus pause, resume, next, previous, stop, shuffle, repeat, volume, seek and now.
    - It matches library words first, then mood words against title, genre, album and artist.
    - With no match, it plays a shuffled DJ mix and says so.
    - `now` reads the state the webview reports every 3 s.
  - `music_ops`: dj, mixes, play_list, wrapped, eq, crossfade, data_saver, sleep, import (YouTube/Spotify playlist or album), fix_tags.
  - `video_ops`: play, series, subtitles, translate_subtitles, recap, settings.
  - `media_download`: batch YouTube search with match scoring, up to 25 items. Returns candidates when unsure. Quality is best/high/small/height; format is m4a/opus.
  - `jamendo_setup`.
  - Player commands reach the UI as `media:cmd` events.
- **LLM:** recap and subtitle translation use `llm.CompleteWith(ctx, a.cfg, …)`. The taste line goes to memory through `mindSave` (kind preference, subject "music and video taste") every 5 plays/downloads. Download done/failed goes through `a.send` (desktop notification, low urgency).

## ~/.blaze changes (additive, backward compatible)

New folder `~/.blaze/media/` holds:
- `prefs.json`, `library.json` (Blaze downloads), `scan.json`
- `playlists.json`, `recent.json`, `recent-videos.json`, `liked.json`
- `plays.json` (≤6000, ≥3 s, no streams), `positions.json` (≤300)
- `art.json`, `tags.json`, `taste.json`, `imports.json`, `downloads.json`, `jamendo.json` (0600)
- folders `art/`, `lyrics/`, `subs/<hash>/` and `dl/` (part files)

Nothing existing is changed. **CLI follow-up:** none required; the CLI never reads `media/`. If the CLI later wants `music_play`/`media_download`, it would need its own player, so leave them desktop-only.

## Feature map: mobile → desktop

| Mobile feature | Status | Notes |
|---|---|---|
| Library from MediaStore (songs, videos) | **Ported (adapted)** | Folder scanner instead of MediaStore: tags, embedded covers, folder covers, add/remove folders, rescan |
| Downloads + library.json, liked, recents (50), plays log, positions, art, imports | **Ported** | Same files, limits and rules |
| Queue, shuffle (tapped song first), repeat off→all→one, up next, move/remove in queue | **Ported** | `media-logic.js` (JS) and `logic.go` (Go), tested |
| Mini player across tabs | **Ported** | `#mzMini`, plus a top-bar ♪/▶ pair |
| Sleep timer (presets, end of song, fade out) | **Ported** | |
| Equalizer (11 presets), crossfade-style fades | **Ported (adapted)** | WebAudio: 5 biquad bands fed by Go `BandLevels`; fade in/out per song (mobile's fadeVolume), not a two-track overlap |
| Lyrics (LRCLIB, cache), synced lyrics, karaoke | **Ported** | Tap a line to seek; karaoke fill per line |
| Library sections: For you, Songs, Liked, Artists, Albums, Playlists, search | **Ported** | |
| Playlists (create/rename/delete/add/remove/move, unique names, 60 chars) | **Ported** | |
| Smart lists (Most played, On repeat, Recently added, Forgotten favourites, Never played, Liked), Daily Mixes (≤3) | **Ported** | |
| Wrapped (month / year / all) | **Ported** | Sheet plus `music_ops wrapped` |
| Blaze DJ | **Ported** | Tops up the queue when it runs out |
| Taste → Store ideas + memory line | **Ported** | |
| Tag fixer + iTunes covers | **Ported** | Folder files are renamed in Blaze only (`tags.json`); files are never rewritten |
| Playlist import (YouTube, Spotify playlist/album) | **Ported** | |
| Store: YouTube, Jamendo, Archive audio, PD films, Blender, by link; licences; stream or download; data saver | **Ported** | Data saver is a manual setting (no metered-network detection on desktop) |
| Downloads (ranges, resume, backoff, refresh, pause/resume/cancel/dismiss, mux, fileName) | **Ported** | ffmpeg replaces MediaMuxer |
| `media_download`, `music_ops`, `video_ops`, `jamendo_setup` tools | **Ported** | Plus the new `music_play` |
| Video library, Continue watching, series grouping | **Ported** | |
| Resume position (under 5 s or within 10 s of the end → 0) | **Ported** | |
| Speed 0.5–2× | **Ported** | |
| Subtitles: YouTube captions (json3/srv1/vtt), SRT/VTT files, sidecar files, per-video saved tracks, preferred language | **Ported** | `<track>` served as VTT |
| AI subtitle translation, AI recap | **Ported** | Uses the user's configured model |
| Skip intro (learned per series), up-next countdown, autoplay, smart downloads | **Ported** | Also "Mark the end of the intro here" |
| PiP, full screen | **Ported** | Where the webview supports them (PiP button hidden otherwise) |
| OS media keys / now playing | **Ported (best effort)** | `navigator.mediaSession`. WebKitGTK only exposes MPRIS when built with media-session support, and WKWebView varies, so it isn't guaranteed. A native MPRIS/SMTC bridge is a follow-up if needed |
| Audio track picker in the video player | **Skipped** | Webviews don't expose `audioTracks` reliably; files play their default track |
| Cover Art Forge (AI covers) | **Later** | No image-generation service in the desktop yet; the tag fixer's iTunes covers cover the common case |
| ClipStudio (Media3 Transformer clip editor) | **Later** | Feasible with ffmpeg (trim/concat/export), but it's a separate editor UI; not in this pass |
| BeatLab (YIN pitch / beat DSP) | **Later** | Needs mic capture plus a DSP port; out of scope |
| Android Auto, media notifications, WorkManager, MediaStore, gesture brightness/volume | **Skipped** | Android-only. Desktop uses a desktop notification for download done/failed |
| Data-saver auto-on for metered networks | **Skipped** | No reliable metered signal on desktop |

## Checks

All are run from the repo root.

| Check | Result |
|---|---|
| `go test ./internal/media/` | 15 tests, pass (also passes with `-race`) |
| `go test -tags webkit2_41 .` and `./internal/tools/` | pass |
| `CGO_ENABLED=1 go vet -tags webkit2_41 ./...` | clean |
| `go build -tags webkit2_41 ./...` | ok |
| `GOOS=windows go vet ./internal/media/` | clean |
| `node ci/steps.test.js` | 80/80 |
| `node ci/picker.test.js` | 48/48 (`NODE_PATH` with jsdom) |
| `node ci/specs.test.js` | 34/34 (`NODE_PATH` with jsdom) |
| `node ci/media.test.js` | 6 groups pass |
| `CHROME=/usr/bin/chromium python3 ci/shots.py` | ok (full run, 315 shots, no ERRORS.txt) |

Media shots, each in light and dark, all passing the radius guard: `music`, `music-lyrics`, `music-playlists-queue`, `music-store-ideas`, `music-store`, `music-store-item`, `music-downloads`, `music-mini`, `video`, `video-player`, `video-subs`.

## Notes and decisions for the user

- **Tool exposure:** `music_play` is not a core tool, so it appears under MORE TOOLS and works by exact name. Making it core (one line in `internal/tools/select.go`) would make "play something chill" one call cheaper on every turn, at about 200 tokens per request. **Decision needed.**
- **ffmpeg:** without ffmpeg, YouTube video tops out at the one-file qualities (usually 360p), and the Store sheet says so. Should the installer or Doctor offer ffmpeg?
- **Playback in WebKitGTK:** depends on the GStreamer plugins installed (e.g. `gst-libav` for AAC/H.264). Worth a Doctor check.
- **Deleting files:** deleting a file from the user's own folders asks first in the UI. The agent cannot delete files.
- **Lost work:** a sandbox restart mid-session lost the uncommitted `internal/media` files once. They were rewritten and committed in small steps afterwards; nothing was lost from the final branch.
