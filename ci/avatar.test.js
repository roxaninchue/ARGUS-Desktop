// jsdom test for the profile badge (v0.19.5): the top-bar avatar shows the letter on its hashed
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
      w.__theme = "dark"; w.__plan = "pro";
      w.matchMedia = w.matchMedia || (() => ({ matches: false, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {} }));
      w.HTMLCanvasElement.prototype.getContext = () => null;
      w.Element.prototype.scrollIntoView = function () {};
      w.eval(mock);
      w.__acct.signed_in = true;
      w.__acct.user = { id: "u1", name: "Ananya", email: "ananya@example.com", avatar_url: "", providers: ["google"], created_at: 1760000000 };
    },
  });
  const w = dom.window, d = w.document;
  if (d.readyState !== "complete") await new Promise(r => w.addEventListener("load", r));
  const tick = (ms = 30) => new Promise(r => setTimeout(r, ms));
  for (let i = 0; i < 100 && !d.querySelector("#avatar.hasav"); i++) await tick(40);
  const av = d.querySelector("#avatar");
  ok(av && av.classList.contains("hasav") && av.textContent.trim() === "A", "top-bar avatar shows the initial");
  // The website's hash: h = h*31 + charCode over "Ananya", colour list index h % 8.
  let h = 0; for (const c of "Ananya") h = (h * 31 + c.charCodeAt(0)) >>> 0;
  const want = ["#0071e3", "#34a853", "#af52de", "#ff9500", "#ff2d55", "#5856d6", "#00a2b8", "#a2845e"][h % 8];
  const rgb = x => { const v = parseInt(x.slice(1), 16); return `rgb(${v >> 16}, ${(v >> 8) & 255}, ${v & 255})`; };
  ok(av && (av.style.background === want || av.style.background === rgb(want) || av.style.backgroundColor === rgb(want)), "colour matches the website's hash (" + (av && av.style.background) + ")");
  for (let i = 0; i < 40 && !d.querySelector(".avwrap .avplan"); i++) await tick(40);
  const pill = d.querySelector(".avwrap .avplan");
  ok(pill && pill.textContent === "Pro" && pill.dataset.plan === "pro", "plan pill under the avatar says Pro");
  av.click(); await tick(150);
  const menu = d.querySelector(".shmenu");
  const txt = menu ? menu.textContent : "";
  ok(!!menu && /Ananya/.test(txt) && /ananya@example.com/.test(txt), "menu shows who you are");
  ok(/Add picture/.test(txt) && /Sign out/.test(txt), "menu has Add picture and Sign out");
  ok(!/Upgrade/.test(txt), "no Upgrade row on Pro");
  // Basic: Upgrade shows; signing out drops the pill.
  w.__plan = "basic"; await w.eval("planBadge(ACCT.user, true)"); await tick(30);
  ok(d.querySelector(".avwrap .avplan").textContent === "Basic", "pill follows the plan (Basic)");
  w.eval("typeof closePop===\"function\" && closePop()"); await tick(30); av.click(); await tick(150);
  ok(/Upgrade/.test((d.querySelector(".shmenu") || {}).textContent || ""), "Upgrade row on Basic");
  w.__acct.user.avatar_url = "https://example.test/v1/avatar/u1?v=1"; await w.eval("loadAccount()"); await tick(30);
  ok(av.dataset.key && av.dataset.key.startsWith("https://"), "picture URL is used when set");
  w.__acct.signed_in = false; w.__acct.user = null; await w.eval("loadAccount()"); await tick(30);
  ok(!d.querySelector(".avwrap .avplan") && !av.classList.contains("hasav"), "signed out: icon back, no pill");
  ok(!errs.length, "no page errors" + (errs.length ? ": " + errs.slice(0, 3).join(" | ") : ""));
  console.log(`\n${n - fails}/${n} passed`);
  process.exit(fails ? 1 : 0);
})().catch(e => { console.log("FAIL crashed: " + (e && e.stack || e)); process.exit(1); });
