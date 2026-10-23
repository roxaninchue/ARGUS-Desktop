// 0.19.16 jsdom test: working pill, delivery status, no provider/model/owner names in the chat chrome.
// Run: NODE_PATH=<jsdom> node ci/workpills.test.js. Header kept from the eye toggles and "What happened" (v0.19.7). Derived from the profile badge test.
// (v0.19.5): the top-bar avatar shows the letter on its hashed
// colour (or the picture), a Basic/Standard/Pro pill sits on its bottom edge, and the avatar menu
// has the plan, Upgrade, picture and Sign out rows. Run: node ci/avatar.test.js (jsdom on NODE_PATH)
const fs = require("fs"), path = require("path");
const { JSDOM, VirtualConsole } = require("jsdom");
const dist = path.join(__dirname, "..", "frontend", "dist");
let fails = 0, n = 0;
const ok = (cond, msg) => { n++; if (!cond) { fails++; console.log("FAIL " + msg); } else console.log("ok   " + msg); };

(async () => {
  const html = fs.readFileSync(path.join(dist, "index.html"), "utf8");
  const mock = fs.readFileSync(path.join(__dirname, "mock.js"), "utf8");
  const vc = new VirtualConsole(); const errs = [];
  vc.on("jsdomError", e => errs.push(String(e && (e.detail || e.message || e))));
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

  const LEAK = /groq|cerebras|sambanova|cohere|mistral|gemini|openrouter|nvidia|\bnim\b|z\.ai|\bglm|llama|qwen|deepseek|gpt|openai|anthropic|claude|kimi|hugging ?face|luke|lackson|kapapa/i;
  const chrome = () => [...d.querySelectorAll("#log .meta, #log .card, #log .wpill, #log .dstat, #log .work, #log .thinking, #routeText, #route")].map(e => e.textContent + " " + (e.title || "")).join(" | ");
  if (typeof w.show === "function") { w.show("work"); await tick(80); }
  // 1. delivery status: Sending -> (offline) -> Sent -> Read, latest message only
  await w.eval('sendText("make a chart from sales.csv", [], [])'); await tick(30);
  const st = () => { const all = d.querySelectorAll("#log .msg.user > .dstat"); return { n: all.length, el: all[all.length - 1], t: all.length ? all[all.length - 1].textContent.trim() : "" }; };
  ok(st().n === 1 && st().t === "Sending", "Sending under the new message: " + st().t);
  w.fire("run_event", { type: "message_sending", client_msg_id: "c1", attempt: 1 });
  w.fire("thinking", 1); await tick(10);
  ok(st().t === "Sending", "run path: still Sending before the 202");
  w.fire("run_event", { type: "message_sending", client_msg_id: "c1", attempt: 2, offline: true }); await tick(10);
  ok(st().t === "Sending" && st().el.classList.contains("offline"), "offline: stays Sending (marked offline)");
  w.fire("run_event", { type: "message_received", client_msg_id: "c1", run_id: "run_1" }); await tick(10);
  ok(st().t === "Sent" && !st().el.classList.contains("offline"), "202 -> Sent");
  w.fire("run_event", { type: "message_read", client_msg_id: "c1", run_id: "run_1" }); await tick(10);
  ok(st().t === "Read" && st().el.dataset.state === "read", "first run event -> Read");
  // 2. working pill: nothing before 2 s, then the current step, replaced in place
  w.fire("tool", { id: "t1", name: "file_read", args: '{"path":"/home/a/sales.csv"}' }); await tick(30);
  ok(!d.querySelector("#log .wpill"), "no pill before ~2 s");
  await tick(2100);
  const pill = () => [...d.querySelectorAll("#log .wpill")];
  ok(pill().length === 1 && /Reading sales\.csv/.test(pill()[0].textContent), "pill shows the step after 2 s: " + (pill()[0] || {}).textContent);
  ok(pill()[0] && pill()[0].parentElement.classList.contains("blaze"), "pill sits under the pending Blaze reply");
  ok(pill()[0] && pill()[0].parentElement.classList.contains("haspill"), "the live Working line drops its copy of the step while the pill shows");
  w.fire("tool_done", { id: "t1", result: "ok" });
  w.fire("tool", { id: "t2", name: "dispatch_specialist", args: '{"name":"data-analyst","brief":"Chart revenue by store from sales.csv. Files: sales.csv"}' }); await tick(20);
  ok(pill().length === 1 && pill()[0].textContent.trim() === "Getting help to chart revenue by store from sales.csv", "specialist: what the help is for, never which: " + (pill()[0] || {}).textContent);
  w.fire("run_event", { type: "specialty_updated", text: "data-analyst on groq" }); await tick(20);
  ok(pill()[0].textContent.trim() === "Saving your rule: make a chart from sales.csv", "specialty_updated with no detail: from the request, never which: " + pill()[0].textContent);
  w.fire("run_event", { type: "specialty_loaded", text: "Using a specialty", detail: "Fix the build error" }); await tick(20);
  ok(pill()[0].textContent.trim() === "Getting help to fix the build error", "specialty_loaded with detail: its detail: " + pill()[0].textContent);
  ok(!/^(Bringing in a specialist|Updating a specialist|Using a specialty|Updating a specialty)$/.test(pill()[0].textContent.trim()), "no static specialist phrase when detail exists");
  w.fire("status", "Asking Groq llama-3.3-70b"); w.fire("run_event", { type: "step", text: 'web_search {"query":"x"}' }); await tick(20);
  ok(pill()[0].textContent.trim() === "Getting help to fix the build error", "provider status / raw JSON steps never reach the pill");
  w.fire("tool", { id: "t3", name: "web_search", args: '{"query":"sales benchmarks"}' }); await tick(20);
  ok(/^Searching the web/.test(pill()[0].textContent.trim()) && pill()[0].textContent.length <= 62, "step replaced in place: " + pill()[0].textContent);
  ok(!LEAK.test(chrome()), "chrome clean while working: " + (chrome().match(LEAK) || [""])[0]);
  // 3. the reply starts streaming: the pill goes (the words are the progress); then the final lands
  w.fire("delta", "Here is your chart. Llama is a model family too."); await tick(10);
  ok(!d.querySelector("#log .wpill"), "pill hidden once the reply streams");
  ok(!d.querySelector("#log .msg.blaze.haspill"), "the Working line gets its step text back when the pill goes");
  w.fire("done", { route: "Groq · llama-3.3-70b-versatile", note: "Cerebras didn't answer, so Groq did", provider: "groq", model: "llama-3.3-70b", seconds: 4.2 });
  await tick(30);
  ok(!d.querySelector("#log .wpill"), "pill removed when the reply lands");
  ok(/Llama is a model family/.test(d.querySelector("#log").textContent), "the reply text itself is untouched");
  w.fire("model", { label: "Groq · llama-3.3-70b", selection: { auto: false, provider: "groq", model: "llama-3.3-70b", label: "Groq · llama-3.3-70b" } }); await tick(20);
  w.fire("route", { route: "SambaNova · Meta-Llama-3.3-70B" }); await tick(20);
  ok(!LEAK.test(chrome()), "footer, model pill and its tooltip clean: " + (chrome().match(LEAK) || [""])[0]);
  // 4. a failed turn: the error card says what failed, never which provider; status stays on the latest only
  await w.eval('sendText("again", [], [])'); await tick(20);
  ok(st().n === 1 && st().t === "Sending", "status moves to the latest message only");
  w.fire("thinking", 1); w.fire("status", "Trying Cerebras");
  w.fire("error", { message: "Groq is rate limited and Cerebras is out of quota.", detail: "openrouter 429: deepseek/deepseek-chat-v3:free", kind: "model",
    failures: [{ name: "Groq", message: "429 llama-3.3-70b" }, { name: "Cerebras", message: "quota" }] });
  await tick(30);
  ok(d.querySelector("#log .card.err") && !LEAK.test(chrome()), "error card and fallback lines clean: " + (chrome().match(LEAK) || [""])[0]);
  ok(st().t === "Read", "a failed turn Blaze worked on stays Read");
  // 5. offline classic turn stays at Sending
  await w.eval('sendText("hello?", [], [])'); await tick(20);
  w.fire("error", { message: "Blaze can't reach Blaze Cloud. Check your internet connection.", kind: "model" }); await tick(20);
  ok(st().t === "Sending" && st().el.classList.contains("offline"), "offline: stays Sending");
  // 6. a background run: the pill moves under the ack bubble and goes when the final lands
  await w.eval('sendText("create notes.txt", [], [])'); await tick(20);
  w.fire("run_event", { type: "message_sending", client_msg_id: "c9", attempt: 1 });
  w.fire("run_event", { type: "message_received", client_msg_id: "c9", run_id: "run_9" });
  w.fire("run_event", { type: "message_read", client_msg_id: "c9", run_id: "run_9" });
  w.fire("delta", "Got it, creating notes.txt. I'll confirm when it's done."); w.fire("done", { ack: true, route: "Blaze Cloud" }); await tick(20);
  w.fire("run_event", { type: "step", run_id: "run_9", client_msg_id: "c9", text: "Writing notes.txt" }); await tick(2100);
  const ack = d.querySelector("#log .msg.blaze.ack");
  ok(ack && ack.querySelector(".wpill") && /Writing notes\.txt/.test(ack.querySelector(".wpill").textContent), "background run: pill under its ack bubble");
  w.fire("blaze_msg", { conv: w.eval("convId"), run_id: "run_9", kind: "done", text: "Done ✓ Created notes.txt" }); await tick(20);
  ok(!d.querySelector("#log .wpill"), "background pill gone when the final lands");
  // 7. developer flag shows names again
  w.localStorage.setItem("blaze.dev", "1");
  ok(w.PRIVACY.route("Groq · llama") === "Groq · llama", "developer flag lets names through");
  w.localStorage.removeItem("blaze.dev");
  // 8. About: maker Blazeinchue, version
  if (typeof w.show === "function") { w.show("settings", "about"); await tick(150); }
  const about = d.querySelector(".about");
  ok(about && /Blazeinchue/.test(about.textContent) && !LEAK.test(about.textContent), "About names Blazeinchue as maker: " + (about ? about.textContent.replace(/\s+/g, " ").slice(0, 80) : "missing"));
  // jsdom has no canvas (getContext is null): the orb's clearRect error is the same on 0.19.15 and not ours.
  const own = errs.filter(e => !/reading 'clearRect'/.test(e));
  ok(!own.length, "no page errors" + (own.length ? ": " + own.slice(0, 3).join(" | ") : ""));
  console.log(`\n${n - fails}/${n} passed`);
  process.exit(fails ? 1 : 0);
})().catch(e => { console.log("FAIL crashed: " + (e && e.stack || e)); process.exit(1); });
