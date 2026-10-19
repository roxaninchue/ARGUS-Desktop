// jsdom test for the Specialists strip (and the Private vision setting): a finished round tucks away instead of staying
// on "3 done" forever; work still running or awaiting review keeps it up.
// Run: npm i jsdom (anywhere on NODE_PATH), then node ci/specs.test.js
const fs = require("fs"), path = require("path");
const { JSDOM, VirtualConsole } = require("jsdom");
const dist = path.join(__dirname, "..", "frontend", "dist");

let fails = 0, n = 0;
const ok = (cond, msg) => { n++; if (!cond) { fails++; console.log("FAIL " + msg); } else console.log("ok   " + msg); };

// ---- the real page with the CI fake backend ----
(async () => {
  const html = fs.readFileSync(path.join(dist, "index.html"), "utf8");
  const mock = fs.readFileSync(path.join(__dirname, "mock.js"), "utf8");
  const vc = new VirtualConsole(); const errs = [];
  vc.on("jsdomError", e => errs.push(String(e && (e.detail || e.message || e))));
  // Inline the page's scripts and styles and give it a real-looking origin, so localStorage
  // works (file:// is opaque) and no resource loader is needed.
  const page = html
    .replace(/<script src="([^"]+)"><\/script>/g, (_, f) => fs.existsSync(path.join(dist, f)) ? "<script>" + fs.readFileSync(path.join(dist, f), "utf8").replace(/<\/script/gi, "<\\/script") + "</script>" : "")
    .replace(/<link rel="stylesheet" href="([^"]+)">/g, (_, f) => fs.existsSync(path.join(dist, f)) ? "<style>" + fs.readFileSync(path.join(dist, f), "utf8") + "</style>" : "");
  const dom = new JSDOM(page, {
    url: "http://blaze.test/index.html", runScripts: "dangerously", pretendToBeVisual: true, virtualConsole: vc,
    beforeParse(w) {
      w.__theme = "dark";
      w.matchMedia = w.matchMedia || (() => ({ matches: false, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {} }));
      w.HTMLCanvasElement.prototype.getContext = () => null;
      w.Element.prototype.scrollIntoView = function () {};
      w.eval(mock);
    },
  });
  const w = dom.window, d = w.document;
  if (d.readyState !== "complete") await new Promise(r => w.addEventListener("load", r));
  const tick = (ms = 30) => new Promise(r => setTimeout(r, ms));
  for (let i = 0; i < 100 && !d.querySelector("#input"); i++) await tick(30);
  if (!d.querySelector("#input")) { console.log(errs.slice(0,5).join("\n")); w.eval("show(\"work\")"); await tick(100); }
  ok(!!d.querySelector("#input"), "page: composer is up");
  // Continue: a reply cut off at the output limit, stopped, or ended by an error part-way offers Continue,
  // which sends the [Continue] message (shown as a small "Continue" bubble).
  {
    const sent = []; const A = w.go.main.App, sw = A.SendWith, s1 = A.Send;
    A.SendWith = async (t, f, a) => { sent.push(t); return "ok"; }; A.Send = async (t, f) => { sent.push(t); return "ok"; };
    if (typeof w.show === "function") { w.show("work"); await tick(80); }
    await w.eval('sendText("write me a long story", [], [])'); await tick(40);
    w.fire("delta", "Once upon a time, the"); w.fire("done", { route: "Groq · x", cut: true }); await tick(40);
    const row = d.querySelector(".cutrow");
    ok(!!row && /cut off/.test(row.textContent) && !!row.querySelector(".contbtn"), "continue: a cut-off reply offers Continue");
    sent.length = 0; row.querySelector(".contbtn").click(); await tick(60);
    ok(sent.length === 1 && sent[0].startsWith("[Continue]"), "continue: the button sends the [Continue] message");
    const users = [...d.querySelectorAll(".msg.user")]; const last = users[users.length - 1];
    ok(last && last.classList.contains("cont") && /Continue/.test(last.textContent) && !/Pick up/.test(last.textContent), "continue: shown as a small Continue bubble");
    ok(!d.querySelector(".cutrow"), "continue: the old Continue goes away");
    w.fire("delta", " end."); w.fire("stopped", { resumable: true }); await tick(40);
    const metas = [...d.querySelectorAll(".meta")]; const sm = metas[metas.length - 1];
    ok(sm && /Stopped/.test(sm.textContent) && !!sm.querySelector(".contbtn"), "continue: Stop leaves a Continue");
    await w.eval('sendText("again", [], [])'); await tick(40);
    ok(!d.querySelector(".contbtn"), "continue: a new message clears old Continue buttons");
    w.fire("delta", "Half an ans"); w.fire("error", { message: "The model went away.", kind: "model", partial: true }); await tick(40);
    const card = [...d.querySelectorAll(".card.err")].pop();
    ok(card && !!card.querySelector(".contbtn") && /Retry/.test(card.textContent), "continue: an error part-way offers Continue beside Retry");
    await w.eval('sendText("steps", [], [])'); await tick(40);
    w.fire("error", { message: "Stopped after 30 steps without finishing.", kind: "steps" }); await tick(40);
    const sc = [...d.querySelectorAll(".card.err")].pop();
    ok(sc && !!sc.querySelector(".contbtn.accent") && !/Retry/.test(sc.textContent), "continue: the step limit offers Continue");
    A.SendWith = sw; A.Send = s1;
  }

  // v0.9.3 chats run on their own: switching chats mid-answer is allowed and the answer keeps going.
  {
    const A = w.go.main.App, oc = A.OpenConversation;
    if (typeof w.show === "function") { w.show("work"); await tick(60); }
    await w.eval('sendText("long task", [], [])'); await tick(30);
    ok(w.eval("busy") === true, "background chats: a turn is running");
    let opened = "";
    A.OpenConversation = async id => { opened = id; return { id, messages: [{ role: "user", content: "older chat" }, { role: "assistant", content: "old answer" }] }; };
    await w.eval('openChat("c2")'); await tick(40);
    ok(opened === "c2" && w.eval("busy") === false, "background chats: you can open another chat while one is answering");
    w.fire("live", ["c1"]); await tick(10);
    w.eval("markLive()"); // v0.19: no sidebar; the live set is still tracked for project chats
    ok(!d.querySelector("#chats") && w.eval("LIVE.has('c1')"), "background chats: the working chat is tracked (no sidebar in v0.19)");
    A.OpenConversation = async id => ({ id, running: true, messages: [{ role: "user", content: "long task" }, { role: "assistant", content: "Half way" }] });
    await w.eval('openChat("c1")'); await tick(40);
    ok(w.eval("busy") === true && !!w.eval("current"), "background chats: going back picks the live answer up");
    w.fire("delta", "and finished."); w.fire("done", { route: "x" }); await tick(30);
    ok(w.eval("busy") === false && /finished/.test(d.querySelector("#log").textContent), "background chats: the answer finishes on screen");
    A.OpenConversation = oc;
  }

  // v0.9.2 no queue: a message sent mid-answer is sent at once (Go interrupts), never shown as queued.
  {
    const sent = []; const A = w.go.main.App, sw = A.SendWith;
    A.SendWith = async t => { sent.push(t); return "ok"; };
    if (typeof w.show === "function") { w.show("work"); await tick(60); }
    await w.eval('sendText("first", [], [])'); await tick(30);
    w.fire("turn", { kind: "user" }); w.fire("delta", "Working on it");
    await w.eval('sendText("second", [], [])'); await tick(30);
    const users = [...d.querySelectorAll(".msg.user")]; const last = users[users.length - 1];
    ok(sent[sent.length - 1] === "second" && last && !last.classList.contains("queued"), "no queue: a mid-answer message goes straight out");
    w.fire("stopped", { resumable: true }); w.fire("turn", { kind: "user" }); await tick(30);
    const blz = [...d.querySelectorAll(".msg.blaze")];
    ok(blz.length >= 2 && blz[blz.length - 1].compareDocumentPosition(last) & 2, "no queue: the interrupting message gets its own answer right after it");
    w.fire("done", { route: "x" }); await tick(20);
    A.SendWith = sw;
    const g = w.agentsGroup(() => {}); d.body.append(g);
    ok(!g.querySelector("#leadWorks") && /only when you ask/.test(g.textContent), "no lead mode: Blaze does tasks itself, specialists only on request");
    g.remove();
  }

  // v0.9 New design: a scratch artboard, drawn at its real size scaled to fit, with Export.
  if (typeof w.show === "function") { w.show("design"); await tick(150); }
  const nb = d.querySelector("#dnew");
  ok(!!nb, "design: New button in the toolbar");
  if (nb) {
    nb.click(); await tick(60);
    const opts = [...d.querySelectorAll(".pop .abfmt")];
    ok(opts.length === 3 && /1080×1350/.test(opts[0].textContent), "design: New lists the formats with sizes");
    opts[0].click(); await tick(200);
    const ex = d.querySelector("#dexport"), dev = d.querySelector("#ddev");
    ok(ex && ex.style.display !== "none" && dev && dev.style.display === "none", "design: an artboard shows Export and hides the device widths");
    const fr = d.querySelector("#dstage .abholder iframe");
    ok(!!fr && fr.style.width === "1080px" && fr.style.height === "1350px" && /scale\(/.test(fr.style.transform), "design: the frame is the real 1080×1350, scaled");
    ex.click(); await tick(60);
    const pdf = [...d.querySelectorAll(".pop .opt")].find(o => /PDF/.test(o.textContent));
    ok(!!pdf, "design: Export offers PNG and PDF");
    pdf.click(); await tick(80);
    ok([...d.querySelectorAll(".toast")].some(t => /Saved .*exports\/summer-sale-v01\.pdf/.test(t.textContent)), "design: export says where it saved");
  }

  // Settings › Notifications › Private vision renders and toggles.
  const col = d.createElement("div"); d.body.append(col);
  await w.SETTINGS_FNS.notifications(col, () => {}); await tick(60);
  const pvIn = col.querySelector(".pvbox input");
  ok(!!pvIn && !pvIn.checked && /OCR/.test(col.querySelector(".pvnote").textContent), "private vision: toggle shows, off, with the OCR engine named");
  pvIn.checked = true; pvIn.dispatchEvent(new w.Event("change")); await tick(60);
  ok(col.querySelector(".pvbox input").checked && (await w.go.main.App.PrivateVision()).on, "private vision: turning it on saves");

  const S = w.eval("Specs"), conv = w.eval("convId") || "c-test";
  if (!w.eval("convId")) w.eval("convId = 'c-test'");
  const strip = () => d.querySelector("#specs");
  const visible = () => !!strip() && strip().style.display !== "none";
  const sum = () => strip() ? strip().querySelector(".sp-sum").textContent : "";
  const now = Date.now();
  const spec = (h, status, extra) => ({ handle: h, kind: "specialist", conv, status, task: "task " + h, mode: "build", started: now - 5000, ...(status === "running" ? {} : { ended: now }), ...extra });

  ["a", "b", "c"].forEach(h => S.upsert(spec(h, "running")));
  await tick(30);
  ok(visible() && /3 running/.test(sum()), "strip: shows while specialists run (" + sum() + ")");

  S.upsert(spec("a", "done", { review: "applied" })); S.upsert(spec("b", "done", { review: "applied" }));
  await tick(30);
  ok(visible() && /1 running/.test(sum()), "strip: stays up while one still runs");
  S.upsert(spec("c", "done", { review: "pending", changes: [{ path: "x" }] }));
  await tick(6500);
  ok(visible() && /1 to review/.test(sum()), "strip: stays up while a result awaits review");

  S.upsert(spec("c", "done", { review: "applied" }));
  await tick(30);
  ok(visible() && /3 done/.test(sum()) && strip().classList.contains("all-done"), "strip: shows the finished round briefly");
  await tick(6500);
  ok(!visible(), "strip: a finished round tucks away on its own");

  S.upsert(spec("d", "running"));
  await tick(30);
  ok(visible() && sum() === "1 running", "strip: a new round shows only the new specialist (" + sum() + ")");
  S.upsert(spec("d", "failed"));
  await tick(30);
  strip().querySelector(".sp-clear").click(); await tick(30);
  ok(!visible(), "strip: Clear hides a finished round at once");

  S.load([spec("e", "done", { review: "applied" }), spec("f", "running")]);
  await tick(30);
  ok(visible() && sum() === "1 running", "reopened chat: only specialists that still need something show (" + sum() + ")");

  const real = errs.filter(e => !/Could not load|Not implemented|canvas|getContext|globalAlpha|ResizeObserver/i.test(e));
  ok(!real.length, "no page errors" + (real.length ? ": " + real.slice(0, 3).join(" | ") : ""));
  console.log(`\n${n - fails}/${n} passed`);
  w.close();
  process.exit(fails ? 1 : 0);
})().catch(e => { console.log("FAIL crashed: " + (e && e.stack || e)); process.exit(1); });
