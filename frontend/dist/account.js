// Blaze account: sign in / create an account (Google, GitHub, email + password)
// through Blaze's sign-in service, and Settings › Account. Optional: connectors,
// chats stay on this computer either way; model keys sync with the account while
// signed in (keysync.go). Loaded after accounts.js.

IC.user = '<circle cx="8" cy="5.5" r="2.75"/><path d="M2.5 14c.6-2.9 2.8-4.5 5.5-4.5s4.9 1.6 5.5 4.5"/>';
IC.mail = '<rect x="1.5" y="3.5" width="13" height="9"/><path d="m1.5 4 6.5 5 6.5-5"/>';

let ACCT = null; // last AccountState()
let signMode = "signin"; // signin | signup

async function loadAccount() { ACCT = await API().AccountState() || { signed_in: false, login: {} }; return ACCT; }

function initials(u) {
  const s = (u.name || u.email || "?").trim();
  const p = s.split(/[\s@._-]+/).filter(Boolean);
  return ((p[0] || "?")[0] + (p.length > 1 && u.name ? p[1][0] : "")).toUpperCase();
}

function avatar(u, big) {
  const a = h("span", "uav" + (big ? " big" : ""), esc(initials(u)));
  if (u.avatar_url && /^https:\/\//.test(u.avatar_url)) {
    const img = new Image(); img.alt = ""; img.referrerPolicy = "no-referrer";
    img.onload = () => { a.textContent = ""; a.append(img); };
    img.src = u.avatar_url;
  }
  return a;
}

// The two-factor step (blaze-auth 0.9.1): an authenticator app code, or a code emailed to
// the account. Without an app the email has already gone out (Go sends it).
let MFA = null;
function mfaForm(onDone) {
  const m = MFA, app = (m.methods || []).includes("app"), mail = (m.methods || []).includes("email"), rec = (m.methods || []).includes("recovery");
  let byEmail = !app || !!m.sent_to;
  const box = h("div", "signbox mfabox");
  const draw = () => {
    box.innerHTML = `<div class="label acc">Two-factor sign-in</div><h2>${byEmail ? "Check your email" : "Enter your code"}</h2>
      <p class="sub small">${byEmail ? (m.sent_to ? `Blaze sent a 6-digit code to <span class="mono">${esc(m.sent_to)}</span>. It works for 10 minutes.` : `Blaze can send a 6-digit code to <span class="mono">${esc(m.email_hint || "your email")}</span>.`) : "Open your authenticator app and type the 6-digit code for Blaze."}${rec ? " You can also use a recovery code." : ""}</p>
      <form class="formgrid signform" autocomplete="off">
        <div class="fg"><label class="label">${byEmail ? "6-digit code from the email" : "Code"}</label><input class="field mono" name="code" inputmode="numeric" autocomplete="one-time-code" placeholder="123456" required></div>
        <div class="out small" role="alert"></div>
        <button class="btn accent go" type="submit">Verify</button>
      </form>
      <div class="switch small sub mfalinks"></div>`;
    const out = $(".out", box), links = $(".mfalinks", box);
    if (m.send_error) { out.className = "out small bad"; out.textContent = m.send_error; m.send_error = ""; }
    const link = (t, f) => { const b = h("button", "btn ghost xs", t); b.type = "button"; b.onclick = f; links.append(b); };
    const send = async () => {
      out.className = "out small sub"; out.textContent = "Sending a code…";
      const r = await API().AccountMFAEmail(m.token);
      if (!r || r.error) { out.className = "out small bad"; out.textContent = (r && r.error) || "Couldn't send the code."; return; }
      m.sent_to = r.sent; byEmail = true; draw();
      const o = $(".out", box); o.className = "out small ok"; o.textContent = "Code sent.";
    };
    if (mail && !byEmail) link("Email me a code instead", send);
    if (byEmail && mail) link(m.sent_to ? "Send a new code" : "Send the code", send);
    if (byEmail && app) link("Use my authenticator app", () => { byEmail = false; draw(); });
    link("Start over", () => { MFA = null; box.replaceWith(signinForm(onDone)); });
    const form = $("form", box);
    form.onsubmit = async e => {
      e.preventDefault();
      const go = $(".go", box);
      go.disabled = true; out.className = "out small sub"; out.textContent = "Checking…";
      const r = await API().AccountMFAVerify(m.token, new FormData(form).get("code") || "");
      go.disabled = false;
      if (!r || r.error) { out.className = "out small bad"; out.textContent = (r && r.error) || "That code didn't work."; if (r && /expired/.test(r.error || "")) { MFA = null; setTimeout(() => box.replaceWith(signinForm(onDone)), 1500); } return; }
      MFA = null;
      ACCT = r.state || await loadAccount();
      toast("Signed in as " + ACCT.user.email, "ok");
      onDone && onDone();
    };
    setTimeout(() => { const i = $("input", box); if (i) i.focus(); }, 30);
  };
  draw();
  return box;
}

// The sign-in / sign-up form. onDone runs after a successful sign-in.
function signinForm(onDone) {
  if (MFA) return mfaForm(onDone);
  const st = ACCT || { login: {} };
  const box = h("div", "signbox");
  const pend = st.pending;
  if (pend) {
    box.innerHTML = `<div class="label acc">Blaze account</div><h2>Finish in your browser</h2>
      <div class="waitline"><span class="sq"></span><span>Waiting for ${esc(pend === "github" ? "GitHub" : "Google")} to send you back to Blaze…</span></div>
      <div class="keyrow" style="margin-top:16px"><button class="btn line sm cancel">Cancel</button></div>`;
    $(".cancel", box).onclick = async () => { await API().AccountCancel(); await loadAccount(); box.replaceWith(signinForm(onDone)); };
    return box;
  }
  const up = signMode === "signup";
  box.innerHTML = `<div class="label acc">Blaze account</div><h2>${up ? "Create your Blaze account" : "Sign in to Blaze"}</h2>
    <p class="sub small">Optional. Your chats, connectors and keys stay on this computer either way.</p>
    <div class="oauthbtns"></div>
    <div class="ordiv"><span class="label weak">or with email</span></div>
    <form class="formgrid signform" autocomplete="on">
      <div class="fg nm" ${up ? "" : "hidden"}><label class="label">Name <span class="weak">optional</span></label><input class="field" name="name" autocomplete="name" placeholder="Luke"></div>
      <div class="fg"><label class="label">Email</label><input class="field" name="email" type="email" autocomplete="email" placeholder="you@example.com" required></div>
      <div class="fg"><label class="label">Password</label><input class="field" name="password" type="password" autocomplete="${up ? "new-password" : "current-password"}" placeholder="${up ? "At least 8 characters" : "Your password"}" required></div>
      <div class="out small" role="alert"></div>
      <button class="btn accent go" type="submit">${up ? "Create account" : "Sign in"}</button>
    </form>
    <div class="switch small sub">${up ? "Already have an account?" : "New to Blaze?"} <button class="btn ghost xs sw">${up ? "Sign in" : "Create an account"}</button></div>`;
  const ob = $(".oauthbtns", box);
  [["google", "Google"], ["github", "GitHub"]].forEach(([id, name]) => {
    const on = st.login && st.login[id];
    const b = h("button", "btn line obtn", `${logo(id)}<span>Continue with ${name}</span>${on ? "" : '<span class="soon label weak">Soon</span>'}`);
    b.type = "button";
    b.disabled = !on;
    b.title = on ? `Sign in with your ${name} account in the browser` : `Sign in with ${name} isn't switched on yet`;
    b.onclick = async () => {
      const err = await API().AccountSignIn(id);
      if (err) { toast(err, "error"); return; }
      await loadAccount(); box.replaceWith(signinForm(onDone));
    };
    ob.append(b);
  });
  if (!st.online) $(".ordiv", box).insertAdjacentHTML("afterend", `<div class="callout small">${icon("x")}<div>Blaze can't reach its sign-in service right now. Check your connection.</div></div>`);
  $(".sw", box).onclick = () => { signMode = up ? "signin" : "signup"; box.replaceWith(signinForm(onDone)); };
  const form = $("form", box);
  form.onsubmit = async e => {
    e.preventDefault();
    const out = $(".out", box), go = $(".go", box), f = new FormData(form);
    go.disabled = true; out.className = "out small sub"; out.textContent = up ? "Creating your account…" : "Signing in…";
    const r = await API().AccountEmail(up ? "signup" : "signin", f.get("name") || "", f.get("email") || "", f.get("password") || "");
    go.disabled = false;
    if (r && r.mfa) { MFA = r.mfa; box.replaceWith(mfaForm(onDone)); return; }
    if (!r || r.error) { out.className = "out small bad"; out.textContent = (r && r.error) || "Sign-in failed."; return; }
    ACCT = r.state || await loadAccount();
    toast(up ? "Welcome to Blaze" : "Signed in as " + ACCT.user.email, "ok");
    onDone && onDone();
  };
  return box;
}

// Signed-in account card (Settings › Account and the top of Accounts).
function accountCard(compact, redraw) {
  const u = ACCT.user;
  const card = h("div", "ucard" + (compact ? " compact" : ""));
  const via = (u.providers || []).map(p => p === "github" ? "GitHub" : p === "google" ? "Google" : p);
  if (u.has_password) via.push("Email");
  const since = u.created_at ? new Date(u.created_at * 1000).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" }) : "";
  card.append(avatar(u, !compact));
  const main = h("div", "um", `<div class="label acc">Blaze account</div><div class="un">${esc(u.name || u.email.split("@")[0])}</div><div class="ue mono">${esc(u.email)}</div>`);
  card.append(main);
  const acts = h("div", "uacts");
  if (compact) {
    const m = h("button", "btn ghost sm", `Manage${icon("chev", "i sm")}`); m.onclick = () => show("settings", "account"); acts.append(m);
    card.classList.add("click"); card.onclick = e => { if (!e.target.closest("button")) show("settings", "account"); };
  } else {
    main.append(h("div", "upills", via.map(v => `<span class="pill">${esc(v)}</span>`).join("") + (u.email_verified ? '<span class="pill acc">Verified</span>' : "")));
    if (since) main.append(h("div", "note2", `Member since ${esc(since)}`));
    const so = h("button", "btn line sm", "Sign out");
    so.onclick = async () => { so.disabled = true; await API().AccountSignOut(); await loadAccount(); toast("Signed out of Blaze"); redraw && redraw(); };
    acts.append(so);
  }
  card.append(acts);
  return card;
}

function signedOutCard() {
  const card = h("div", "ucard compact out click", `<span class="uav off">${icon("user")}</span><div class="um"><div class="label">Blaze account</div><div class="un">Not signed in</div><div class="ue">Sign in with Google, GitHub or email. Optional.</div></div>`);
  const b = h("button", "btn line sm", "Sign in");
  b.onclick = e => { e.stopPropagation(); show("signin"); };
  card.onclick = () => show("signin");
  const a = h("div", "uacts"); a.append(b); card.append(a);
  return card;
}

// Full-screen sign-in page.
async function signinView(el, mode) {
  if (mode === "signup" || mode === "signin") signMode = mode;
  await loadAccount();
  const pg = h("div", "page signpage"); const col = h("div", "col"); pg.append(col); el.append(pg);
  if (ACCT.signed_in) { show("settings", "account"); return; }
  const back = h("button", "btn ghost xs back", `<svg class="i sm" viewBox="0 0 16 16"><path d="m10 4-4 4 4 4"/></svg>Back`); back.onclick = () => show("work");
  col.append(back, signinForm(() => show("settings", "account")));
}

window.VIEWS = window.VIEWS || {};
window.VIEWS.signin = signinView;

SECTIONS.unshift(["account", "Account", "user"]);
window.SETTINGS_FNS = window.SETTINGS_FNS || {};
window.SETTINGS_FNS.account = async (col, redraw) => {
  await loadAccount();
  if (!ACCT.signed_in) {
    col.append(signinForm(redraw));
    return;
  }
  col.append(accountCard(false, redraw));
  const g = group("What your account does", "");
  g.append(h("div", "box pad note2 acctnote", "Your Blaze account is who you are across Blaze apps. One-click connectors work with or without it, and your chats, memory and connectors never leave this computer. Your model keys follow your account to Blaze on your phone and other computers."));
  col.append(g);
  const ks = group("Model keys", "Shared with Blaze CLI on this computer. While you're signed in, they also sync, encrypted, with Blaze on your phone and your other computers.");
  const kb = h("div", "box pad keyrow");
  const kn = h("div", "note2", "");
  const fmt = st => st && st.lastSync ? "Last synced " + new Date(st.lastSync * 1000).toLocaleString("en-GB", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" }) : "Not synced yet";
  try { kn.textContent = fmt(await API().KeySyncState()); } catch (e) { kn.textContent = ""; }
  const sb = h("button", "btn line sm", "Sync now");
  sb.onclick = async () => {
    sb.disabled = true;
    const st = await API().KeySyncNow();
    sb.disabled = false;
    kn.textContent = fmt(st);
    if (st && st.message) toast(st.message, st.available === false ? undefined : "ok");
  };
  kb.append(kn, sb); ks.append(kb); col.append(ks);
  col.append(securityGroup(redraw));
  const d = group("Delete account");
  const bx = h("div", "box pad keyrow dangerrow");
  bx.append(h("div", "note2", "Deletes your Blaze account and signs out everywhere. Nothing on this computer is removed."));
  const del = h("button", "btn ghost sm danger", "Delete account");
  del.onclick = async () => {
    if (!del.dataset.sure) { del.dataset.sure = 1; del.textContent = "Delete for good? Click again"; return; }
    del.disabled = true;
    const err = await API().AccountDelete();
    if (err) { toast(err, "error"); del.disabled = false; return; }
    await loadAccount(); toast("Your Blaze account is deleted"); redraw();
  };
  bx.append(del); d.append(bx); col.append(d);
};

// Account › Security: emailed sign-in codes (blaze-auth 0.9.1). Switching needs a code from the email.
function securityGroup(redraw) {
  const u = ACCT.user, tf = u.two_factor_methods || {};
  const g = group("Security", tf.app ? "Two-factor sign-in with your authenticator app is on." : "Ask for a second step when you sign in on a new device.");
  const bx = h("div", "box pad");
  const row = h("label", "toggle", `<input type="checkbox" ${tf.email ? "checked" : ""}><span><span class="tt">Email codes</span><span class="td" style="display:block">When you sign in, Blaze emails a 6-digit code to <span class="mono">${esc(u.email)}</span>.</span></span>`);
  const cb = $("input", row), step = h("div", "keyrow mfastep"), out = h("div", "out small", "");
  step.hidden = true;
  cb.onchange = async () => {
    const want = cb.checked; cb.checked = !want; cb.disabled = true;
    out.className = "out small sub"; out.textContent = "Sending a code…";
    const r = await API().AccountEmailCodesSend(want);
    cb.disabled = false;
    if (!r || r.error) { out.className = "out small bad"; out.textContent = (r && r.error) || "Couldn't send the code."; return; }
    out.className = "out small sub"; out.textContent = `Enter the code sent to ${r.sent || u.email}${!want && tf.app ? " (or an app code)" : ""}.`;
    step.innerHTML = `<input class="field mono" inputmode="numeric" autocomplete="one-time-code" placeholder="123456"><button class="btn accent sm">${want ? "Turn on" : "Turn off"}</button><button class="btn ghost sm">Cancel</button>`;
    step.hidden = false;
    const [inp] = $$("input", step), [ok, cancel] = $$("button", step);
    inp.focus();
    cancel.onclick = () => { step.hidden = true; out.textContent = ""; };
    const done = async () => {
      ok.disabled = true;
      const s = await API().AccountEmailCodesSet(want, inp.value);
      ok.disabled = false;
      if (!s || s.error) { out.className = "out small bad"; out.textContent = (s && s.error) || "That code didn't work."; return; }
      ACCT = s.state || await loadAccount();
      toast(want ? "Email codes are on" : "Email codes are off", "ok");
      redraw && redraw();
    };
    ok.onclick = done; inp.onkeydown = e => { if (e.key === "Enter") done(); };
  };
  bx.append(row, step, out); g.append(bx);
  return g;
}

// A Google/GitHub sign-in that needs a second factor comes back here.
on("account_mfa", m => { MFA = m; show("signin"); });

on("account", async st => {
  ACCT = st || await loadAccount();
  if (view === "signin") { if (ACCT.signed_in) show("settings", "account"); else show("signin"); }
  if (view === "settings" && settingsSection === "account") show("settings", "account");
  if (view === "accounts" && connRedraw) connRedraw();
});

COMMANDS.splice(COMMANDS.findIndex(c => c.name === "accounts"), 0, { name: "account", desc: "Your Blaze account: sign in, sign out", run: () => { loadAccount().then(a => show(a.signed_in ? "settings" : "signin", a.signed_in ? "account" : undefined)); } });
