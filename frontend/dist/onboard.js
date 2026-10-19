// First-run onboarding: a full-screen step flow over the app.
// Welcome → model → apps → permissions & background → quick tour → done.
// Skippable, resumable (the step is saved in desktop.json) and re-openable from
// Settings › Behaviour › "Run setup again". Replaces the old three-step wizard.

const Onboard = (() => {
  IC.bell = IC.bell || '<path d="M4 11.5V7a4 4 0 0 1 8 0v4.5l1 1H3zM6.5 14h3"/>';
  const STEPS = [["welcome", "Welcome"], ["model", "Model"], ["apps", "Apps"], ["perms", "Permissions"], ["tour", "Quick tour"], ["done", "Done"]];
  const FREE = ["gemini", "groq", "cerebras", "openrouter", "mistral", "nvidia"];
  const reduce = window.BlazeMotion.query;
  let step = 0, root = null, pick = "", more = false;

  const hasModel = st => (st.providers || []).some(p => p.ready && p.id !== "pollinations");
  const shouldOpen = st => !st.setup_done || (!hasModel(st) && !(st.prefs && st.prefs.onboard_skipped));

  // The particle orb; with reduced motion it is drawn once and kept as a still image.
  function orb(size, cls) {
    const cv = h("canvas", "oborb " + (cls || "")); const dpr = Math.min(2, window.devicePixelRatio || 1);
    cv.width = cv.height = Math.round(size * dpr); cv.style.width = cv.style.height = size + "px";
    particleOrb(cv, size);
    if (reduce.matches) setTimeout(() => { if (!cv.isConnected) return; const img = h("img", cv.className); img.src = cv.toDataURL(); img.style.cssText = cv.style.cssText; cv.replaceWith(img); }, 120);
    return cv;
  }

  async function open(at) {
    S = await API().State();
    if (at == null) { try { at = (await API().Onboarding()).step || 0; } catch (e) { at = 0; } }
    step = Math.max(0, Math.min(STEPS.length - 1, at | 0));
    if (!root) {
      root = h("div", ""); root.id = "onboard";
      root.setAttribute("role", "dialog"); root.setAttribute("aria-modal", "true"); root.setAttribute("aria-label", "Set up Blaze");
      document.body.append(root);
    }
    document.body.classList.add("onboarding");
    draw();
  }
  function close() { if (root) root.remove(); root = null; document.body.classList.remove("onboarding"); }
  async function go(n) { step = Math.max(0, Math.min(STEPS.length - 1, n)); API().SetOnboardStep(step); S = await API().State(); draw(); }
  async function skip() { await API().SkipOnboarding(); await refreshState(); close(); hero(); show("work"); }
  async function finish(then) { await API().FinishOnboarding(); await refreshState(); close(); hero(); show(then || "work"); }

  function draw() {
    if (!root) return;
    const [id] = STEPS[step];
    root.innerHTML = "";
    const rail = h("aside", "obrail");
    const brand = h("div", "obbrand", `<img class="logo" src="logo.svg" alt=""><span class="bname">Blaze</span><span class="label weak">Setup</span>`);
    const list = h("ol", "obsteps");
    STEPS.forEach(([sid, name], i) => {
      const li = h("li", i === step ? "on" : i < step ? "done" : "", `<span class="sq">${i < step ? icon("check", "i sm") : ""}</span><span class="label">${pad(i + 1)}</span><span class="nm">${esc(name)}</span>`);
      li.onclick = () => go(i);
      list.append(li);
    });
    rail.append(brand, list, h("div", "obfoot label weak", "Change any of this later in Settings."));
    const main = h("section", "obmain");
    const top = h("div", "obtop", `<span class="label acc">${pad(step + 1)} / ${pad(STEPS.length)} · ${esc(STEPS[step][1])}</span><span class="grow"></span>`);
    if (id !== "done") { const sk = h("button", "btn ghost sm", "Skip setup"); sk.onclick = skip; top.append(sk); }
    const body = h("div", "obbody " + id);
    const nav = h("div", "obnav");
    main.append(top, body, nav);
    root.append(rail, main);
    ({ welcome, model, apps, perms, tour, done })[id](body, nav);
  }

  function navBtns(nav, next, opts = {}) {
    if (step > 0) { const b = h("button", "btn line", `<svg class="i sm" viewBox="0 0 16 16"><path d="m10 4-4 4 4 4"/></svg>Back`); b.onclick = () => go(step - 1); nav.append(b); }
    nav.append(h("span", "grow"));
    if (opts.note) nav.append(h("span", "obnote", opts.note));
    const n = h("button", "btn accent", `${esc(next || "Continue")}${icon("chev", "i sm")}`);
    n.onclick = opts.onNext || (() => go(step + 1));
    nav.append(n);
  }

  // 01 · Welcome
  function welcome(body, nav) {
    const i = S.info || {};
    body.append(orb(220, "hero"),
      h("h1", "", "Blaze works on <em>this computer</em> for you."),
      h("p", "lead", "It runs commands, edits files, writes code, uses the web and your apps, and asks before anything risky. Two minutes of setup and it's ready."),
      h("div", "obfacts", [["System", `${i.os_name || i.os || ""} · ${i.arch || ""}`], ["Processor", `${i.cpu || "Unknown"} · ${i.cores || "?"} threads`], ["Memory", `${Math.round(i.ram_gb || 0)} GB RAM`]]
        .map(([k, v]) => `<div><span class="label">${k}</span><span class="v">${esc(v)}</span></div>`).join("")));
    navBtns(nav, "Get started");
  }

  // 02 · Pick & connect a model
  function model(body, nav) {
    const provs = (S.providers || []).filter(p => p.id !== "pollinations");
    const ready = provs.filter(p => p.ready);
    body.append(h("h1", "", "Pick a model"), h("p", "lead", "Blaze needs one AI model that can use tools. These have free tiers; add a key and Blaze tests it straight away. Add more later and Auto switches when one is busy."));
    const grid = h("div", "obtiles");
    const shown = provs.filter(p => more || FREE.includes(p.id) || p.ready);
    if (!pick) pick = (ready[0] || shown[0] || {}).id || "";
    shown.forEach(p => {
      const t = h("button", "obtile" + (p.id === pick ? " on" : "") + (p.ready ? " ready" : ""), `${logo(p.id, "big")}<span class="tn">${esc(p.short)}</span><span class="label ${p.ready ? "acc" : "weak"}">${p.ready ? "Connected" : p.keyless ? "Local" : p.free ? "Free tier" : "Paid"}</span>${p.ready ? `<span class="tick">${icon("check", "i sm")}</span>` : ""}`);
      t.onclick = () => { pick = p.id; draw(); };
      grid.append(t);
    });
    if (!more) { const m = h("button", "obtile moretile", `<span class="blogo line big"><svg class="i" viewBox="0 0 16 16">${IC.plus}</svg></span><span class="tn">More providers</span><span class="label weak">${provs.length - shown.length} more</span>`); m.onclick = () => { more = true; draw(); }; grid.append(m); }
    body.append(grid);
    const p = provs.find(x => x.id === pick);
    if (p) body.append(keyPanel(p));
    navBtns(nav, hasModel(S) ? "Continue" : "Skip for now", { note: hasModel(S) ? "" : "Blaze can't act until it has a model." });
  }

  function keyPanel(p) {
    const box = h("div", "obkey");
    const head = h("div", "obkh", `${logo(p.id)}<div><div class="tn">${esc(p.short)}</div><div class="sub small">${p.ready ? `Connected · using <span class="mono">${esc(p.model || "its default model")}</span>` : p.keyless ? "Runs on this computer. Install it, then Blaze checks it's running." : p.id === "custom" ? "Any OpenAI-compatible service. Set its address in Accounts › API Keys." : `Paste your ${esc(p.short)} key.${p.key_hint && p.key_hint !== "…" ? ` It starts <span class="mono">${esc(p.key_hint)}</span>.` : ""}`}</div></div>`);
    box.append(head);
    const out = h("div", "obout small");
    const result = r => { if (r && r.ok) { out.innerHTML = `<span class="ok">${icon("check", "i sm")} Works. Saved in ${esc(r.where || "your keychain")}; using <span class="mono">${esc(r.model || "")}</span>.</span>`; setTimeout(async () => { S = await API().State(); setModel(S.model, S.selection); draw(); }, 900); } else out.innerHTML = `<span class="bad">${esc((r && r.error) || "That didn't work.")}</span>`; };
    if (p.keyless) {
      const row = h("div", "keyrow");
      if (p.key_page) { const g = h("button", "btn line sm", `Get ${esc(p.short)}${icon("ext", "i sm")}`); g.onclick = () => API().OpenURL(p.key_page); row.append(g); }
      const c = h("button", "btn accent sm", p.ready ? "Check again" : "Check and add");
      c.onclick = async () => { c.disabled = true; c.textContent = "Checking…"; result(await API().AddProvider(p.id, "")); c.disabled = false; c.textContent = "Check and add"; };
      row.append(c); box.append(row);
    } else if (p.id === "custom") {
      const b = h("button", "btn line sm", `Open API Keys${icon("chev", "i sm")}`); b.onclick = () => { close(); show("accounts", "keys"); };
      box.append(b);
    } else {
      const row = h("div", "keyrow", `<input class="field mono" type="password" autocomplete="off" spellcheck="false" placeholder="${esc(p.ready ? "Add another key (optional)" : p.key_hint && p.key_hint !== "…" ? p.key_hint : "Paste your key")}">`);
      if (p.key_page) { const g = h("button", "btn line sm", `Get a key${icon("ext", "i sm")}`); g.onclick = () => API().OpenURL(p.key_page); row.append(g); }
      const sv = h("button", "btn accent sm", "Test & save");
      sv.onclick = async () => { const v = $("input", row).value.trim(); if (!v) { $("input", row).focus(); return; } sv.disabled = true; sv.textContent = "Testing…"; out.textContent = ""; result(await API().AddProvider(p.id, v)); sv.disabled = false; sv.textContent = "Test & save"; };
      $("input", row).onkeydown = e => { if (e.key === "Enter") sv.click(); };
      row.append(sv); box.append(row);
      if (p.id === "openrouter") { const oc = h("button", "btn ghost sm", "Sign in with OpenRouter instead"); oc.onclick = async () => { oc.disabled = true; oc.textContent = "Waiting for you in the browser…"; result(await API().ConnectOpenRouter()); oc.disabled = false; oc.textContent = "Sign in with OpenRouter instead"; }; box.append(oc); }
    }
    box.append(out);
    return box;
  }

  // 03 · Connect apps (a subset of Accounts)
  async function apps(body, nav) {
    body.append(h("h1", "", "Connect your apps"), h("p", "lead", "Blaze can read your mail, calendar, repos and pages, and asks before it sends or changes anything. Sign-ins happen in your browser and stay in your system keychain. Optional; add more any time in Accounts."));
    const box = h("div", "abox obapps"); body.append(box);
    navBtns(nav, "Continue");
    box.append(h("div", "aempty", BrandProgress.skeleton(4)));
    await loadConnectors();
    if (!root || STEPS[step][0] !== "apps") return;
    box.innerHTML = "";
    (CV.connectors || []).forEach(c => box.append(connectorRow(c, () => draw(), { from: "setup" })));
    body.append(h("div", "anote", "Each app uses your own free developer app, so nothing passes through anyone else's server. Blaze shows the steps when you press Connect."));
  }

  // 04 · Permissions & background
  function perms(body, nav) {
    const d = S.desktop || {};
    const mac = d.os === "darwin", where = mac ? "menu bar" : "system tray";
    body.append(h("h1", "", "Always one shortcut away"), h("p", "lead", `Blaze can wait in the ${where} and open a small quick-ask orb from anywhere.`));
    const rows = h("div", "abox obperms");
    const row = (ic, title, desc, right) => { const r = h("div", "arow", `<span class="blogo line">${icon(ic)}</span><div class="am"><div class="an">${title}</div><div class="ac">${desc}</div></div><div class="ar"></div>`); if (right) $(".ar", r).append(right); rows.append(r); return r; };
    const tgl = (on, fn) => { const l = h("label", "toggle", `<input type="checkbox" ${on ? "checked" : ""}>`); $("input", l).onchange = e => fn(e.target.checked, e.target); return l; };
    // Shortcut
    const seg = h("div", "seg");
    (d.presets || []).concat([{ id: "off", label: "Off" }]).forEach(p => { const b = h("button", d.hotkey === p.id ? "on" : "", esc(p.label)); b.onclick = async () => { S.desktop = { ...d, ...(await API().SetHotkey(p.id)) }; draw(); }; seg.append(b); });
    const hk = d.wayland ? `Wayland doesn't let apps grab keys. Bind a system shortcut to <span class="mono">blaze-desktop --orb</span>.` : d.hotkey === "off" ? `Off. The ${where} icon still opens quick ask.` : esc(d.hotkey_note || "");
    const r1 = row("sparks", "Quick-ask shortcut", hk); r1.classList.add("wide"); r1.append(h("div", "arx", "")); $(".arx", r1).append(seg);
    // Tray / background
    row("layers", `Keep running in the ${where}`, d.tray ? `Closing the window hides Blaze. Quit from the ${where} menu.` : `The ${where} isn't available here${d.os === "linux" ? " (no StatusNotifier host, e.g. waybar's tray module)" : ""}, so closing still quits.`,
      tgl(d.background, async v => { await API().SetBackground(v); d.background = v; }));
    // Start at login
    const note = h("span", "obnote");
    row("clock", "Start Blaze when you sign in", "It starts hidden, ready for the shortcut.", tgl(d.autostart, async (v, el) => { const err = await API().SetAutostart(v); if (err) { el.checked = !v; toast(err, "error"); } else d.autostart = v; }));
    // Notifications
    const nb = h("button", "btn line sm", "Send a test");
    const nr = row("bell", "Notifications", "Blaze only notifies you about things that matter, like a finished job or something that needs your OK.", nb);
    nb.onclick = async () => { nb.disabled = true; nb.textContent = "Sending…"; const err = await API().OnboardNotifyTest(); nb.disabled = false; nb.textContent = err ? "Try again" : "Sent"; $(".ac", nr).innerHTML = err ? `<span class="bad">${esc(err)}</span>` : `Sent. ${mac ? "If macOS asks, choose Allow." : "Didn't see it? Check your system's notification settings."}`; };
    // macOS Accessibility
    if (mac) {
      const right = h("div", "keyrow");
      if (d.needs_access) {
        const ga = h("button", "btn accent sm", "Grant access"); ga.onclick = () => API().GrantAccess();
        const tr = h("button", "btn line sm", "Try again"); tr.onclick = async () => { S.desktop = { ...d, ...(await API().RetryHotkey()) }; draw(); };
        right.append(ga, tr);
      } else right.append(h("span", "astat", `<span class="dot"></span><span>Granted</span>`));
      row("cursor", "Accessibility", "macOS needs it for the global shortcut (System Settings › Privacy & Security › Accessibility).", right);
    }
    body.append(rows, note);
    navBtns(nav, "Continue");
  }

  // 05 · Quick tour
  function tour(body, nav) {
    const d = S.desktop || {};
    body.append(h("h1", "", "Three things to know"));
    const g = h("div", "obtour");
    const a = h("div", "obcard", `<div class="obvis orbvis"></div><div class="label acc">01 · The orb</div><div class="tt">Quick ask, from anywhere</div><p>${d.hotkey_ok && d.hotkey_label ? `Press <span class="kbd">${esc(d.hotkey_label)}</span>` : "Click the tray icon"} and a small orb opens over whatever you're doing. Esc hides it.</p>`);
    $(".orbvis", a).append(orb(96));
    const b = h("div", "obcard", `<div class="obvis cardvis"><div class="mini-card"><div class="mh"><span class="dot"></span><span>Roadmap</span><span class="grow"></span>${icon("grow", "i sm")}</div><div class="ml"></div><div class="ml s"></div><div class="ml"></div><div class="ml s2"></div></div><div class="mini-ans"><div class="ml"></div><div class="ml s"></div></div></div><div class="label acc">02 · Answer cards</div><div class="tt">Short answer first</div><p>Long tables, steps and code fold into a card above the answer. Click to expand.</p>`);
    const c = h("div", "obcard", `<div class="obvis docvis">${["Internet", "Model", "Shell", "Shortcut"].map((x, i) => `<div class="drow"><span class="dot ${i === 3 && !d.hotkey_ok ? "ink" : ""}"></span><span>${x}</span><span class="grow"></span><span class="label ${i === 3 && !d.hotkey_ok ? "ink" : "acc"}">${i === 3 && !d.hotkey_ok ? "Fix" : "OK"}</span></div>`).join("")}</div><div class="label acc">03 · Doctor</div><div class="tt">When something's off</div><p>Doctor in the sidebar checks your models, shell, tools, tray and shortcut, and says what to fix.</p>`);
    g.append(a, b, c); body.append(g);
    navBtns(nav, "Continue");
  }

  // 06 · Done
  function done(body, nav) {
    const d = S.desktop || {};
    const m = (S.providers || []).filter(p => p.ready && p.id !== "pollinations");
    const n = S.connected || 0;
    body.append(orb(160, "hero"), h("h1", "", "You're set."), h("p", "lead", "Ask Blaze to do something on this computer. It shows what it's doing and asks before anything risky."));
    const ck = h("div", "abox obsum");
    const line = (ok, t, s) => ck.append(h("div", "arow", `<span class="sq2 ${ok ? "ok" : ""}">${ok ? icon("check", "i sm") : ""}</span><div class="am"><div class="an">${t}</div><div class="ac">${s}</div></div>`));
    line(m.length, "Model", m.length ? esc(m.map(p => p.short).join(", ")) : "None yet. Add one in Accounts › API Keys.");
    line(n, "Apps", n ? `${n} connected` : "None connected. Add them any time in Accounts.");
    line(d.hotkey_ok, "Shortcut", d.hotkey_ok && d.hotkey_label ? esc(d.hotkey_label) : d.wayland ? "Bind a system shortcut to blaze-desktop --orb" : "Off");
    body.append(ck);
    const doc = h("button", "btn line", "Run Doctor"); doc.onclick = () => finish("doctor");
    if (step > 0) { const b = h("button", "btn line", `<svg class="i sm" viewBox="0 0 16 16"><path d="m10 4-4 4 4 4"/></svg>Back`); b.onclick = () => go(step - 1); nav.append(b); }
    nav.append(h("span", "grow"), doc);
    const st = h("button", "btn accent", `Start using Blaze${icon("chev", "i sm")}`); st.onclick = () => finish("work");
    nav.append(st);
  }

  function refresh() { if (root && STEPS[step][0] === "apps") draw(); }

  document.addEventListener("keydown", e => {
    if (!root) return;
    if (e.key === "Enter" && !e.target.closest("input, textarea, button")) { const n = $(".obnav .btn.accent", root); if (n) { e.preventDefault(); n.click(); } }
  });

  return { open, close, shouldOpen, refresh, go: n => go(n), get step() { return step; } };
})();
window.Onboard = Onboard;

// show("setup") opens the flow over the chat.
window.VIEWS = window.VIEWS || {};
window.VIEWS.setup = (el, at) => { workView(el); Onboard.open(at); };

// Settings › Behaviour › Setup: run it again.
const _setBehaviourBase = setBehaviour;
setBehaviour = async function (col, redraw) {
  await _setBehaviourBase(col, redraw);
  const g = group("Setup", "The first-run walkthrough: model, apps, shortcut and permissions.");
  const b = h("button", "btn line sm", `${icon("retry", "i sm")}Run setup again`);
  b.onclick = () => show("setup", 0);
  g.append(b); col.append(g);
};

// Going anywhere else closes the flow (its step is saved, so show("setup") resumes).
const _showBase = show;
show = function (v, arg) {
  if (v !== "setup" && document.body.classList.contains("onboarding")) Onboard.close();
  return _showBase(v, arg);
};
