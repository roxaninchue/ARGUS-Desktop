// "What happened" under each reply (v0.19.7): the turn id that matches the gateway's logs, time to
// first word, each step's model and service, every Blaze Cloud attempt (retries, hedges, errors),
// and the tools with their timings. Closed by default; for working out why a reply was odd.
(() => {
  const ms = v => v == null ? "" : v < 1000 ? `${Math.round(v)} ms` : `${(v / 1000).toFixed(2)} s`;
  const sec = v => v == null ? "" : v < 1 ? `${Math.round(v * 1000)} ms` : `${v.toFixed(2)} s`;
  const e = s => (typeof esc === "function" ? esc(String(s == null ? "" : s)) : String(s == null ? "" : s));

  function body(t) {
    const rows = [];
    rows.push(`<div class="wh-kv"><span>Request ID</span><span class="mono">${e(t.turn)}</span><button class="btn ghost xs wh-copy" title="Copy the request ID">Copy</button></div>`);
    rows.push(`<div class="wh-kv"><span>First word</span><span>${t.ttft_ms ? ms(t.ttft_ms) : "none streamed"}</span></div>`);
    rows.push(`<div class="wh-kv"><span>Total</span><span>${sec(t.seconds)}</span></div>`);
    if (t.error) rows.push(`<div class="wh-kv bad"><span>Error</span><span>${e(t.error)}</span></div>`);
    const steps = (t.steps || []).map(s => `<tr><td>${s.n}</td><td>${e(s.route || s.provider || "")}${s.model && s.model !== s.route ? ` <span class="weak">${e(s.model)}</span>` : ""}</td><td>${s.ttft_ms ? ms(s.ttft_ms) : ""}</td><td>${sec(s.seconds)}</td><td>${s.tokens_in || s.tokens_out ? `${s.tokens_in || 0} → ${s.tokens_out || 0}` : ""}</td><td>${e(s.finish || "")}${s.error ? ` <span class="bad">${e(s.error)}</span>` : ""}${s.note ? ` <span class="weak">${e(s.note)}</span>` : ""}</td></tr>`).join("");
    if (steps) rows.push(`<div class="wh-h">Steps</div><table class="wh-t"><tr><th>#</th><th>Model</th><th>First word</th><th>Took</th><th>Tokens</th><th>Ended</th></tr>${steps}</table>`);
    const att = (t.attempts || []).map((a, i) => `<tr><td>${i + 1}</td><td>${e(a.provider || "")}${a.model ? ` · ${e(a.model)}` : ""}</td><td>${a.status || ""}</td><td>${a.ttfb_ms ? ms(a.ttfb_ms) : ""}</td><td>${sec(a.seconds)}</td><td>${[a.hedged ? "hedged" : "", a.replay ? "replayed" : "", a.finish || ""].filter(Boolean).join(", ")}${a.error ? ` <span class="bad">${e(a.error)}</span>` : ""}</td></tr>`).join("");
    if (att) rows.push(`<div class="wh-h">Blaze Cloud requests</div><table class="wh-t"><tr><th>#</th><th>Provider · model</th><th>Status</th><th>Gateway first byte</th><th>Took</th><th>Notes</th></tr>${att}</table>`);
    const tools = (t.tools || []).map(x => `<tr><td>${x.step}</td><td class="mono">${e(x.name)}</td><td>${sec(x.seconds)}</td><td>${x.blocked ? "blocked" : x.error ? `<span class="bad">${e(x.error)}</span>` : "ok"}</td></tr>`).join("");
    rows.push(tools ? `<div class="wh-h">Tools</div><table class="wh-t"><tr><th>Step</th><th>Tool</th><th>Took</th><th>Result</th></tr>${tools}</table>` : `<div class="wh-h">Tools</div><div class="weak small">None</div>`);
    return rows.join("");
  }

  function attach(el, t) {
    if (!el || !t || el.querySelector(".wh")) return;
    const box = h("div", "wh");
    const btn = h("button", "btn ghost xs wh-btn", "What happened");
    const pan = h("div", "wh-pan"); pan.hidden = true;
    btn.onclick = () => {
      if (pan.hidden && !pan.innerHTML) {
        pan.innerHTML = body(t);
        const c = pan.querySelector(".wh-copy");
        if (c) c.onclick = () => { try { navigator.clipboard.writeText(t.turn); c.textContent = "Copied"; } catch (x) {} };
      }
      pan.hidden = !pan.hidden; btn.classList.toggle("on", !pan.hidden);
    };
    box.append(btn, pan);
    const meta = el.querySelector(".meta"); (meta || el).after ? (meta ? meta.after(box) : el.append(box)) : el.append(box);
  }

  on("trace", t => { const el = window.__lastReply; window.__lastReply = null; attach(el, t); });
  window.WhatHappened = { attach, body };
})();
