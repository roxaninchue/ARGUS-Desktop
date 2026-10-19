# Desktop v0.19.0 plan

Built as four workstreams off fix/glass-v0.18.1, merged into feat/v0.19.0. Each has a PROGRESS-*.md at the repo root with details, decisions and what's left.

| Area | New code | Mobile source | Notes |
|---|---|---|---|
| One chat + Home | internal/chatpage, internal/home, onechat_app.go, home_app.go, onechat.js, home.js | Shell.kt, ChatStore.kt, home/*.kt | Sidebar removed; old chats paged in, never deleted; ~/.blaze/desktop/home.json |
| Topic memory | internal/topics, internal/longchat/topics.go, topics_app.go, topics.js | LongChat.kt, AutoCompact.kt, Mind*.kt | Additive tables chat_topics, longchat_meta; on by default, falls back to v0.18 path |
| Blaze Cloud + Custom | internal/cloudai, cloudmodels_app.go, cloudmodels.js | Cloud.kt, ThinkBudget.kt, EmptyReply.kt, Providers.kt | Gateway https://ai.blaze-app.workers.dev/v1 (override BLAZE_GATEWAY_URL); ~/.blaze/desktop/models.json; CLI unaffected |
| Music + Video | internal/media, media_app.go, music.js, video.js, media-store.js | mediaapps/*.kt | ~/.blaze/media/; ClipStudio, BeatLab, Cover Art later |

Not in this release: Call (waiting on the user's choice), agent tools for Home cards, rename/merge topics, onboarding Blaze sign-in step, Doctor checks for ffmpeg and GStreamer codecs.
