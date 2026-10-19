// v0.4.2 polish: AI chat titles (animated, rename on double-click) and mini particle-orb marks.
// Loaded after app.js; it only uses app.js globals (API, on, $, $$, h, esc, current) and small hooks.
(() => {
  // ---------------- chat titles ----------------
  const latest = {};   // id -> newest title we were told about (beats a stale list fetch)
  let editing = null;  // id of the chat being renamed
  const downs = {};    // id -> last mousedown time (double-click survives a list rebuild)

  // Typewriter: the old title fades out, the new one types in with a blinking square caret.
  function typeTitle(el, text) {
    if (!el || el.textContent === text) return;
    if (matchMedia("(prefers-reduced-motion: reduce)").matches) { el.textContent = text; return; }
    clearInterval(el._tw);
    el.classList.add("tfade");
    setTimeout(() => {
      el.classList.remove("tfade");
      el.classList.add("typing");
      el.textContent = "";
      let i = 0;
      const chars = [...text];
      el._tw = setInterval(() => {
        el.textContent = chars.slice(0, ++i).join("");
        if (i >= chars.length) { clearInterval(el._tw); setTimeout(() => el.classList.remove("typing"), 700); }
      }, Math.max(18, Math.min(42, 900 / chars.length)));
    }, 220);
  }

  function startRename(b, c) {
    const t = $(".t", b);
    if (!t || $("input", b)) return;
    editing = c.id;
    const inp = h("input", "rename");
    inp.value = latest[c.id] || c.title || "";
    inp.setAttribute("aria-label", "Rename chat");
    inp.maxLength = 80;
    t.replaceWith(inp);
    b.classList.add("editing");
    inp.focus(); inp.select();
    let done = false;
    const finish = async save => {
      if (done) return; done = true; editing = null;
      const v = inp.value.trim();
      const span = h("span", "t", esc(save ? (v || c.title) : (latest[c.id] || c.title)));
      inp.replaceWith(span); b.classList.remove("editing");
      if (!save || v === (latest[c.id] || c.title)) return;
      const final = await API().RenameConversation(c.id, v);
      latest[c.id] = final || v; c.source = v ? "manual" : "";
      b.dataset.src = c.source;
      span.textContent = latest[c.id];
      if (!v) span.title = "Back to automatic titles";
    };
    inp.onkeydown = e => { e.stopPropagation(); if (e.key === "Enter") finish(true); if (e.key === "Escape") finish(false); };
    inp.onblur = () => finish(true);
    inp.onclick = e => e.stopPropagation();
  }

  // Called by refreshSide for every chat row.
  function decorate(b, c) {
    b.dataset.id = c.id;
    b.dataset.src = c.source || "";
    const t = $(".t", b);
    if (latest[c.id] && latest[c.id] !== c.title && t) { t.textContent = latest[c.id]; c.title = latest[c.id]; }
    if (t) t.title = c.title + (c.source === "manual" ? " · renamed" : c.source === "auto" ? " · named by Blaze · double-click to rename" : " · double-click to rename");
    b.addEventListener("mousedown", e => {
      if (e.button !== 0 || e.target.closest(".x, input")) return;
      const now = performance.now(), prev = downs[c.id] || 0;
      downs[c.id] = now;
      if (now - prev < 420) { e.preventDefault(); downs[c.id] = 0; startRename(b, c); }
    });
    b.addEventListener("dblclick", e => { if (!e.target.closest(".x, input")) startRename(b, c); });
    if (editing === c.id) setTimeout(() => startRename(b, c), 0);
  }

  on("title", d => {
    if (!d || !d.id || !d.title) return;
    latest[d.id] = d.title;
    if (editing === d.id) return;
    const b = $(`#chats .chat[data-id="${CSS.escape(d.id)}"]`);
    if (b) { typeTitle($(".t", b), d.title); b.dataset.src = d.source || "auto"; }
  });

  // ---------------- mini particle orbs ----------------
  // One sprite sheet of a turning dot sphere per (size, dpr, theme); every mark blits its
  // frame. Only marks on screen that belong to live work animate (≈15 fps); the rest keep a
  // frozen frame. prefers-reduced-motion: always frozen.
  const SEL = ".msg.blaze .who .mk, .msg.blaze .work .sq, .step .st, .thinking .sq, .acard .amk, .sprow .sp-mk";
  const FRAMES = 48, FPS = 15;
  const reduced = matchMedia("(prefers-reduced-motion: reduce)");
  const sheets = {};
  const marks = new Set();
  const visible = new Set();
  let frame = 7, raf = 0, last = 0;

  function sheet(css) {
    const dpr = Math.max(1, Math.round((window.devicePixelRatio || 1) * 4) / 4);
    const theme = document.documentElement.dataset.theme || "light";
    const key = css + "|" + dpr + "|" + theme;
    if (sheets[key]) return sheets[key];
    const px = Math.round(css * dpr);
    const cv = document.createElement("canvas");
    cv.width = px * FRAMES; cv.height = px;
    const ctx = cv.getContext("2d");
    const st = getComputedStyle(document.documentElement);
    const acc = st.getPropertyValue("--accent").trim() || "#E6E6EA", ink = st.getPropertyValue("--ink").trim() || "#111110";
    const N = css >= 10 ? 34 : 24;
    const pts = Array.from({ length: N }, (_, i) => { const y = 1 - (i + .5) * 2 / N, r = Math.sqrt(1 - y * y), t = i * 2.39996; return [Math.cos(t) * r, y, Math.sin(t) * r]; });
    const R = px * .42, c = px / 2;
    for (let f = 0; f < FRAMES; f++) {
      const a = f / FRAMES * Math.PI * 2, ox = f * px;
      pts.forEach(([x, y, z], i) => {
        const xr = x * Math.cos(a) - z * Math.sin(a), zr = x * Math.sin(a) + z * Math.cos(a);
        const yr = y * Math.cos(.4) - zr * Math.sin(.4), zz = y * Math.sin(.4) + zr * Math.cos(.4);
        const d = (zz + 1) / 2, s = Math.max(1, Math.round(dpr * (css >= 10 ? 1.5 : 1.2) * (.5 + d * .7)));
        ctx.globalAlpha = .4 + d * .6;
        ctx.fillStyle = i % 7 === 0 ? ink : acc;
        ctx.fillRect(Math.round(ox + c + xr * R - s / 2), Math.round(c + yr * R - s / 2), s, s);
      });
    }
    return (sheets[key] = { cv, px });
  }

  function draw(m) {
    const css = m._css, sh = sheet(css);
    if (m.width !== sh.px) { m.width = sh.px; m.height = sh.px; }
    const ctx = m.getContext("2d");
    ctx.clearRect(0, 0, sh.px, sh.px);
    ctx.drawImage(sh.cv, (m._f ?? frame) * sh.px, 0, sh.px, sh.px, 0, 0, sh.px, sh.px);
  }

  const isLive = m => !!(m.closest(".work.live, .step.run, .thinking, .acard.live, .sprow.live") ||
    (typeof current !== "undefined" && current && current.el && m.closest(".who") && m.closest(".msg.blaze") === current.el));

  const io = typeof IntersectionObserver === "function" ? new IntersectionObserver(es => {
    es.forEach(e => e.isIntersecting ? visible.add(e.target) : visible.delete(e.target));
    kick();
  }) : null;

  function attach(host) {
    if (host._orb) return;
    const css = host.matches(".acard .amk") ? 8 : 10;
    const m = document.createElement("canvas");
    m.className = "morb"; m._css = css; m._f = frame;
    m.style.width = m.style.height = css + "px";
    m.setAttribute("aria-hidden", "true");
    host._orb = m; host.classList.add("has-orb");
    host.append(m);
    marks.add(m);
    draw(m);
    if (io) io.observe(m); else visible.add(m);
  }

  function tick(now) {
    raf = 0;
    if (reduced.matches || document.hidden) return;
    let any = false;
    const step = now - last >= 1000 / FPS;
    if (step) { last = now; frame = (frame + 1) % FRAMES; }
    visible.forEach(m => {
      if (!m.isConnected) { visible.delete(m); marks.delete(m); return; }
      if (!isLive(m)) { if (m._f == null) m._f = frame; return; } // freeze where it stopped
      any = true; m._f = null;
      if (step) draw(m);
    });
    if (any) raf = requestAnimationFrame(tick);
  }
  function kick() { if (!raf && !reduced.matches) raf = requestAnimationFrame(tick); }

  function scan(root) {
    if (root.nodeType !== 1) return;
    if (root.matches(SEL)) attach(root);
    root.querySelectorAll(SEL).forEach(attach);
  }
  new MutationObserver(ms => {
    let added = false;
    ms.forEach(r => r.addedNodes.forEach(n => { if (n.nodeType === 1) { scan(n); added = true; } }));
    // Class changes (work.live → done, step.run → ok) start or stop the loop.
    if (added || ms.some(r => r.type === "attributes")) kick();
    marks.forEach(m => { if (!m.isConnected) { marks.delete(m); visible.delete(m); if (io) io.unobserve(m); } });
  }).observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ["class"] });
  scan(document.body);

  // Theme or display changes: rebuild the sheets and redraw every mark's frozen frame.
  const redrawAll = () => { for (const k in sheets) delete sheets[k]; marks.forEach(draw); };
  new MutationObserver(redrawAll).observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
  matchMedia(`(resolution: ${window.devicePixelRatio || 1}dppx)`).addEventListener?.("change", redrawAll);
  reduced.addEventListener?.("change", kick);
  document.addEventListener("visibilitychange", kick);
  ["delta", "tool", "tool_done", "status", "thinking", "done", "error", "stopped"].forEach(ev => on(ev, kick));

  // ---------------- personality (Settings › Behaviour) ----------------
  function personalityGroup(redraw) {
    const g = group("Personality", "How Blaze sounds in chat. Scheduled jobs always run plain.");
    const mode = (S.prefs.personality || "default");
    const seg = h("div", "seg");
    const box = h("div", "pbox");
    [["default", "Default"], ["plain", "Plain"], ["custom", "Custom"]].forEach(([k, t]) => {
      const b = h("button", mode === k ? "on" : "", t);
      b.onclick = async () => {
        let custom = S.prefs.personality_custom || "";
        if (k === "custom" && !custom.trim()) custom = (await API().DefaultPersonality() || "").replace(/^PERSONALITY\n/, "");
        await API().SetPersonality(k, custom);
        S.prefs.personality = k; S.prefs.personality_custom = custom; redraw();
      };
      seg.append(b);
    });
    g.append(seg, box);
    if (mode === "default") {
      box.append(h("div", "td pnote", "Warm, sharp and one step ahead: answers first, takes a side, specific, honest about being an assistant."));
    } else if (mode === "plain") {
      box.append(h("div", "td pnote", "No personality block: just Blaze's rules and your style note below."));
    } else {
      const ta = h("textarea", "field pcustom"); ta.value = S.prefs.personality_custom || ""; ta.rows = 9;
      ta.placeholder = "Describe how Blaze should talk and behave.";
      const sv = h("button", "btn line sm", "Save personality"); sv.style.marginTop = "8px";
      sv.onclick = async () => { await API().SetPersonality("custom", ta.value); S.prefs.personality_custom = ta.value.trim(); sv.textContent = "Saved"; setTimeout(() => sv.textContent = "Save personality", 1200); };
      box.append(ta, sv);
    }
    return g;
  }

  window.Polish = { decorate, typeTitle, startRename, kick, personalityGroup };
})();
