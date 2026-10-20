// jsdom test (v0.19.7): every password field gets a show/hide eye, and a finished reply gets a
// "What happened" panel from the trace event. Run: node ci/reveal.test.js (jsdom on NODE_PATH)
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
  // Sign-in page: the password field gets an eye that toggles it.
  w.eval('show("signin")'); await tick(300);
  const pw = d.querySelector('input[name="password"]');
  const eye = pw && pw.parentElement.querySelector(".pweye");
  ok(!!eye && pw.parentElement.classList.contains("pwbox"), "sign-in password has an eye");
  eye.click(); await tick(10);
  ok(pw.type === "text" && eye.getAttribute("aria-pressed") === "true", "eye shows the password");
  eye.click(); await tick(10);
  ok(pw.type === "password", "eye hides it again");
  // A field added later is covered too.
  const late = d.createElement("input"); late.type = "password"; d.body.append(late); await tick(30);
  ok(late.parentElement.classList.contains("pwbox"), "fields added later get an eye");
  // What happened: a reply, then its trace.
  w.eval('show("work")'); await tick(100);
  await w.eval('sendText("hello", [], [])'); await tick(40);
  w.fire("delta", "Hi there"); w.fire("done", { route: "Blaze Cloud", seconds: 1.2 }); await tick(30);
  w.fire("trace", { turn: "tabc123", seconds: 1.2, ttft_ms: 640, steps: [{ n: 1, route: "Blaze Cloud", seconds: 1.2, ttft_ms: 640, finish: "stop", tokens_in: 900, tokens_out: 12, tools_offered: 20, messages: 3 }], attempts: [{ provider: "groq", model: "llama-3.3-70b", status: 200, ttfb_ms: 410, seconds: 1.1 }], tools: [{ step: 1, name: "read_file", seconds: 0.05 }, { step: 1, name: "web_fetch", seconds: 2.1, error: "Error: timed out" }] });
  await tick(30);
  const btn = [...d.querySelectorAll(".wh-btn")].pop();
  ok(!!btn, "reply has a What happened button");
  btn.click(); await tick(20);
  const pan = btn.parentElement.querySelector(".wh-pan");
  const t = pan ? pan.textContent : "";
  ok(!pan.hidden && /tabc123/.test(t) && /640 ms/.test(t), "panel shows the request ID and first word time");
  ok(/groq/.test(t) && /llama-3.3-70b/.test(t) && /410 ms/.test(t), "panel shows provider, model and gateway first byte");
  ok(/read_file/.test(t) && /timed out/.test(t), "panel lists tools with their errors");
  ok(!errs.length, "no page errors" + (errs.length ? ": " + errs.slice(0, 3).join(" | ") : ""));
  console.log(`\n${n - fails}/${n} passed`);
  process.exit(fails ? 1 : 0);
})().catch(e => { console.log("FAIL crashed: " + (e && e.stack || e)); process.exit(1); });
