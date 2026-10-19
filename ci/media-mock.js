// v0.19 Music + Video shots: fake media backend on top of ci/mock.js (loaded after it).
(() => {
  const cov = (a, b, c) => "data:image/svg+xml;utf8," + encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${a}"/><stop offset="1" stop-color="${b}"/></linearGradient></defs><rect width="120" height="120" fill="url(#g)"/><circle cx="84" cy="38" r="22" fill="${c}" opacity=".55"/><path d="M0 92 Q40 70 70 92 T120 86 V120 H0z" fill="#fff" opacity=".18"/></svg>`);
  const wide = (a, b) => "data:image/svg+xml;utf8," + encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 90"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${a}"/><stop offset="1" stop-color="${b}"/></linearGradient></defs><rect width="160" height="90" fill="url(#g)"/><circle cx="118" cy="30" r="16" fill="#fff" opacity=".35"/><path d="M0 70 L40 44 L72 66 L104 40 L160 74 V90 H0z" fill="#000" opacity=".22"/></svg>`);
  const now = Date.now(), day = 864e5;
  const S = (k, t, a, al, d, art, g, src = "folder") => ({ k: "file:/home/luke/Music/" + k, u: "/home/luke/Music/" + k, t, a, al, d, art, src, at: now - Math.random() * 40 * day, kind: "audio", g });
  const songs = [
    S("ana/slow.mp3", "Slow Morning", "Ana Mwale", "Lusaka Lights", 201000, cov("#ff7a59", "#7b2ff7", "#ffd166"), "Lo-Fi"),
    S("ana/rain.mp3", "Piano in the Rain", "Ana Mwale", "Lusaka Lights", 184000, cov("#ff7a59", "#7b2ff7", "#ffd166"), "Piano"),
    S("burna/last.m4a", "Last Last", "Burna Boy", "Love, Damini", 172000, cov("#0f9b8e", "#1d3557", "#f1fa8c"), "Afrobeats", "download"),
    S("rema/calm.m4a", "Calm Down", "Rema", "Rave & Roses", 239000, cov("#f72585", "#3a0ca3", "#4cc9f0"), "Afrobeats", "download"),
    S("tems/free.mp3", "Free Mind", "Tems", "For Broken Ears", 247000, cov("#2b2d42", "#8d99ae", "#ef233c"), "R&B"),
    S("kabza/umshove.mp3", "Umshove", "Kabza De Small", "I Am the King of Amapiano", 412000, cov("#06d6a0", "#118ab2", "#ffd166"), "Amapiano"),
    S("macky/mwana.mp3", "Mwana Wandi", "Macky 2", "Zed Classics", 225000, cov("#e76f51", "#264653", "#e9c46a"), "Zambian"),
    S("yo/sunday.mp3", "Sunday Worship", "Yo Maps", "Praise", 268000, cov("#ffb703", "#fb8500", "#023047"), "Gospel"),
  ];
  const V = (k, t, a, d, art, src = "folder") => ({ k: "file:/home/luke/Videos/" + k, u: "/home/luke/Videos/" + k, t, a, d, art, src, at: now - Math.random() * 20 * day, kind: "video" });
  const videos = [
    V("bbb.mp4", "Big Buck Bunny", "Blender Studio", 596000, wide("#7bd389", "#2a9d8f"), "download"),
    V("show1.mp4", "Kitchen Diaries S01E01", "", 1460000, wide("#f4a261", "#e76f51")),
    V("show2.mp4", "Kitchen Diaries S01E02", "", 1500000, wide("#f4a261", "#e76f51")),
    V("show3.mp4", "Kitchen Diaries S01E03", "", 1380000, wide("#f4a261", "#e76f51")),
    V("trip.mp4", "Victoria Falls trip", "", 312000, wide("#48cae4", "#023e8a")),
    V("sintel.mp4", "Sintel", "Blender Studio", 888000, wide("#9d4edd", "#240046"), "download"),
  ];
  const lib = { songs, videos, playlists: [{ id: "p1", name: "Road trip", keys: [songs[2].k, songs[3].k, songs[5].k], u: now }, { id: "p2", name: "Sunday", keys: [songs[7].k, songs[1].k], u: now }],
    liked: [songs[3].k, songs[4].k], recent: [songs[3].k, songs[0].k, songs[5].k, songs[2].k, songs[6].k], recent_videos: [videos[1].k],
    positions: { [videos[1].k]: 640000, [videos[0].k]: 120000 }, folders: ["/home/luke/Music", "/home/luke/Videos", "/home/luke/Music/Blaze/Music"], scanning: false, last_scan: now,
    prefs: { folders: [], eq: "Flat", crossfade_s: 0, intros: {}, finished: {} }, can_merge: true };
  const mixes = [
    { id: "mix:0", name: "Daily Mix 1", about: "Ana Mwale and more", keys: [songs[0].k, songs[1].k, songs[4].k, songs[6].k] },
    { id: "mix:1", name: "Daily Mix 2", about: "Rema and more", keys: [songs[3].k, songs[2].k, songs[5].k, songs[7].k] },
    { id: "smart:top", name: "Most played", about: "Your 50 most played songs", keys: [songs[3].k, songs[0].k, songs[5].k] },
    { id: "smart:liked", name: "Liked, newest first", about: "Every song you liked", keys: [songs[3].k, songs[4].k] },
  ];
  const store = [
    { source: "youtube", id: "a1", title: "Rema, Selena Gomez - Calm Down (Official Music Video)", author: "RemaVEVO", duration_text: "3:59", thumb: wide("#f72585", "#3a0ca3"), licence: "YouTube", views: "1.1B views" },
    { source: "youtube", id: "a2", title: "Burna Boy - Last Last [Official Music Video]", author: "Burna Boy", duration_text: "2:52", thumb: wide("#0f9b8e", "#1d3557"), licence: "YouTube", views: "330M views" },
    { source: "youtube", id: "a3", title: "Amapiano Mix 2026 | Kabza De Small, DJ Maphorisa", author: "Piano Hub", duration_text: "58:12", thumb: wide("#06d6a0", "#118ab2"), licence: "YouTube", views: "2.4M views" },
    { source: "youtube", id: "a4", title: "Tems - Free Mind (Live Session)", author: "Tems", duration_text: "4:12", thumb: wide("#2b2d42", "#8d99ae"), licence: "YouTube", views: "45M views" },
    { source: "youtube", id: "a5", title: "Zambian Hits Throwback Mix", author: "ZedBeats", duration_text: "1:02:40", thumb: wide("#e76f51", "#264653"), licence: "YouTube", views: "800K views" },
    { source: "youtube", id: "a6", title: "Lo-fi beats to study to", author: "Chill Room", duration_text: "3:01:00", thumb: wide("#ff7a59", "#7b2ff7"), licence: "YouTube", views: "12M views" },
  ];
  const lyrics = { synced: [[0, "Morning comes in slow"], [6000, "Light across the floor"], [12000, "Kettle on, the city hums"], [18000, "I don't need much more"], [24000, "Slow morning, stay a while"], [30000, "Let the day wait at the door"]].map(([ms, text]) => ({ ms, text })), plain: "", instrumental: false, source: "LRCLIB" };
  const dl = [
    { id: "d1", kind: "audio", title: "Tems - Me & U", ch: "Tems", q: "High \u00b7 AAC 130 kbps", st: "RUNNING", pct: 62, thumb: wide("#2b2d42", "#ef233c"), parts: [] },
    { id: "d2", kind: "audio", title: "Kabza De Small - Sponono", q: "High \u00b7 AAC 130 kbps", st: "QUEUED", pct: 0, thumb: wide("#06d6a0", "#118ab2"), parts: [] },
    { id: "d3", kind: "audio", title: "Yo Maps - Finally", q: "High \u00b7 AAC 130 kbps", st: "DONE", pct: 100, thumb: wide("#ffb703", "#fb8500"), parts: [], out: "/home/luke/Music/Blaze/Music/Yo Maps - Finally.m4a" },
    { id: "d4", kind: "video", title: "Cosmos Laundromat", q: "720p", st: "RUNNING", pct: 34, thumb: wide("#9d4edd", "#240046"), parts: [] },
  ];
  const M = {
    MediaInit: async () => ({ base: "", token: "", eq: ["Flat", "Bass boost", "Bass reducer", "Vocal", "Treble boost", "Rock", "Pop", "Jazz", "Classical", "Hip-hop", "Small speaker"], bands: [60, 230, 910, 3600, 14000], sleep: [5, 10, 15, 30, 45, 60, 90], speeds: [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2] }),
    MediaLibrary: async () => lib, MediaMixes: async () => mixes, MediaDJ: async () => songs.slice(0, 5),
    MediaWrapped: async () => ({ label: "2026 so far", minutes: 4312, plays: 1180, songs: 214, artists: 61, busiest_hour: 21, top_songs: [{ track: songs[3], plays: 88 }, { track: songs[0], plays: 64 }, { track: songs[5], plays: 51 }], top_artists: [{ name: "Rema", minutes: 410 }, { name: "Ana Mwale", minutes: 388 }] }),
    MediaLyrics: async () => lyrics, MediaEQ: async () => [0, 0, 0, 0, 0],
    MediaSources: async kind => kind === "video" ? [{ id: "youtube", name: "YouTube" }, { id: "archive-films", name: "Public-domain films", browsable: true }, { id: "blender", name: "Blender open movies", browsable: true }]
      : [{ id: "youtube", name: "YouTube" }, { id: "jamendo", name: "Jamendo", setup: "Jamendo needs a free client_id.", browsable: true }, { id: "archive-audio", name: "Internet Archive", browsable: true }],
    MediaStoreSearch: async () => ({ items: store, next: "x" }), MediaIdeas: async () => ["Rema mix", "Afrobeats", "Amapiano music", "Chill lo-fi beats", "Gospel worship", "Zambian music", "Workout mix"],
    MediaStoreOpen: async (kind, it) => ({ detail: { title: it.title, author: it.author, licence: "YouTube", length_sec: 239 }, choices: [{ id: "a140", label: "High \u00b7 AAC 130 kbps", detail: "m4a \u00b7 3.9 MB" }, { id: "a139", label: "Smaller \u00b7 AAC 49 kbps", detail: "m4a \u00b7 1.5 MB" }, { id: "o251", label: "Opus 160 kbps", detail: "opus \u00b7 4.2 MB" }], default: "a140", stream: "/media/s/x", stream_ms: 239000, cost: "\u2248 3.9 MB to listen" }),
    MediaDownloads: async () => dl, MediaVideoSubs: async () => [{ id: "file-en", label: "English", lang: "en", loaded: true }, { id: "ai-french", label: "French (AI)", lang: "fr", auto: true, loaded: true }],
    MediaNextEpisode: async () => videos[2], MediaRecap: async () => "- Mwila opens the kitchen and plans the menu.\n- The supplier is late, so she improvises with nshima and greens.\n\nIt left off as the first guests arrive.",
  };
  const old = window.go.main.App;
  window.go.main.App = new Proxy({}, { get: (t, k) => M[k] || old[k] });
  window.__media = { lib, songs, videos };
})();
