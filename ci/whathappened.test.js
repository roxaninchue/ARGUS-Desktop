// jsdom test for the eye toggles and "What happened" (v0.19.7). Derived from the profile badge test.
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
  // Eye toggle: any password field, including ones added later.
  const f = d.createElement("form"); f.innerHTML = '<input type="password" name="password" value="hunter2">'; d.body.append(f);
  await tick(30);
  const inp = f.querySelector("input"), eye = f.querySelector(".pweye");
  ok(!!eye && inp.parentElement.classList.contains("pwbox"), "a password field gets an eye button");
  eye.click(); ok(inp.type === "text" && eye.getAttribute("aria-pressed") === "true", "eye shows the password");
  eye.click(); ok(inp.type === "password", "eye hides it again");
  eye.click(); f.dispatchEvent(new w.Event("submit", { cancelable: true })); ok(inp.type === "password", "sending the form hides it");
  const nr = d.createElement("input"); nr.type = "password"; nr.setAttribute("data-noreveal", ""); d.body.append(nr); await tick(30);
  ok(!nr.parentElement.classList.contains("pwbox"), "data-noreveal opts out");
  // What happened: a finished reply gets the button; the trace fills the panel.
  if (typeof w.show === "function") { w.show("work"); await tick(80); }
  await w.eval('sendText("hi", [], [])'); await tick(40);
  w.fire("delta", "Hello"); w.fire("done", { route: "Blaze Cloud", seconds: 1.4 }); await tick(30);
  w.fire("trace", { turn: "t0123abcd", seconds: 1.4, ttft_ms: 640, steps: [{ n: 1, route: "Blaze Cloud", seconds: 1.4, ttft_ms: 640, tokens_in: 900, tokens_out: 12, finish: "stop", tools_offered: 30, messages: 3 }],
    attempts: [{ provider: "groq", model: "llama-3.3-70b", status: 200, ttfb_ms: 310, seconds: 1.3, hedged: true }, { provider: "gemini", status: 429, seconds: 0.2, error: "rate limited" }],
    tools: [{ step: 1, name: "read_file", seconds: 0.05 }, { step: 1, name: "run", seconds: 2.1, error: "Error: exit 1" }] });
  await tick(30);
  const btn = d.querySelector(".wh-btn");
  ok(!!btn, "reply has a What happened button");
  btn && btn.click(); await tick(20);
  const pan = d.querySelector(".wh-pan"), t = pan ? pan.textContent : "";
  ok(pan && !pan.hidden, "panel opens");
  ok(/t0123abcd/.test(t) && /640 ms/.test(t), "request ID and first-word time shown");
  ok(/groq · llama-3\.3-70b/.test(t) && /429/.test(t) && /hedged/.test(t) && /rate limited/.test(t), "every Blaze Cloud attempt with status and notes");
  ok(/read_file/.test(t) && /Error: exit 1/.test(t), "tools with timings and errors");
  btn.click(); ok(pan.hidden, "panel closes");
  ok(!errs.length, "no page errors" + (errs.length ? ": " + errs.slice(0, 3).join(" | ") : ""));
  console.log(`\n${n - fails}/${n} passed`);
  process.exit(fails ? 1 : 0);
})().catch(e => { console.log("FAIL crashed: " + (e && e.stack || e)); process.exit(1); });
