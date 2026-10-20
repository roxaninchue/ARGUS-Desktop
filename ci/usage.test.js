// jsdom test for the usage meter (v0.19.9): the top-bar chip, its levels, the details and the
// daily cap. Run: node ci/usage.test.js (jsdom on NODE_PATH)
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
      w.__acct.signed_in = true;
      w.__acct.user = { id: "u1", name: "Ananya", email: "a@example.com", avatar_url: "" };
      w.__usage = { cloud: true, signed_in: true, cap: 0, quota: { plan: "basic", requests_used: 12, requests_limit: 150, requests_left: 138, tokens_used: 9120, tokens_limit: -1, tokens_left: -1, resets_at: "2026-10-21T00:00:00Z", last_reply_tokens: 710 } };
    },
  });
  const w = dom.window, d = w.document;
  if (d.readyState !== "complete") await new Promise(r => w.addEventListener("load", r));
  const tick = (ms = 30) => new Promise(r => setTimeout(r, ms));
  for (let i = 0; i < 120 && !d.getElementById("usageChip"); i++) await tick(40);
  const chip = d.getElementById("usageChip");
  ok(!!chip && chip.textContent === "138 left" && chip.dataset.level === "ok", "chip shows messages left today");
  chip.click(); await tick(20);
  const pop = d.querySelector(".usagepop"), t = pop ? pop.textContent : "";
  ok(/12 of 150/.test(t) && /no cap/.test(t) && /710 tokens/.test(t) && /Basic/.test(t), "details: messages, tokens without a cap, last reply, plan");
  pop.querySelector(".ucap input").value = "100";
  pop.querySelector(".ucap").dispatchEvent(new w.Event("submit", { cancelable: true })); await tick(30);
  ok(w.__usage.cap === 100, "the daily cap is saved");
  w.__usage = { ...w.__usage, quota: { ...w.__usage.quota, requests_used: 140, requests_left: 10 } };
  await w.UsageMeter.refresh(); await tick(10);
  ok(chip.dataset.level === "low" && chip.textContent === "10 left", "amber at 20% or less");
  w.__usage = { ...w.__usage, quota: { ...w.__usage.quota, requests_used: 150, requests_left: 0 } };
  await w.UsageMeter.refresh(); await tick(10);
  ok(chip.dataset.level === "out", "red when nothing is left");
  w.__usage = { ...w.__usage, cloud: false };
  await w.UsageMeter.refresh(); await tick(10);
  ok(!d.getElementById("usageChip"), "hidden when Blaze Cloud isn't answering (Custom)");
  ok(!errs.length, "no page errors" + (errs.length ? ": " + errs.slice(0, 3).join(" | ") : ""));
  console.log(`\n${n - fails}/${n} passed`);
  process.exit(fails ? 1 : 0);
})().catch(e => { console.log("FAIL crashed: " + (e && e.stack || e)); process.exit(1); });
