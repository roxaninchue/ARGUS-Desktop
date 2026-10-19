// jsdom test for the v0.17.1 mobile design: messaging bubbles (grouping, corners, one meta line per
// group with Copy · Quote · Retry), follow-up pills, the pill composer (mic → send → stop) and its
// rotating placeholder, the greeting, and rich replies (step-flow, pictures, deck, mermaid).
// Run: npm i jsdom (anywhere on NODE_PATH), then node ci/design.test.js
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

  const B = w.Bubbles, MD = w.MD;
  // ---- pure helpers ----
  const T0 = new Date(2026, 9, 14, 14, 2).getTime();
  ok(JSON.stringify(B.positions(["me", "me", "me", "blaze", "me"], [T0, T0 + 1000, T0 + 2000, T0 + 3000, T0 + 4000])) === JSON.stringify(["first", "mid", "last", "single", "single"]), "grouping: a run from one side is first/mid/last");
  ok(JSON.stringify(B.positions(["me", "me"], [T0, T0 + 11 * 60000])) === JSON.stringify(["single", "single"]), "grouping: a 10-minute pause starts a new group");
  ok(JSON.stringify(B.positions(["blaze", null, "blaze"], [0, 0, 0])) === JSON.stringify(["single", "", "single"]), "grouping: anything else in the thread breaks a group");
  ok(B.time(T0, T0 + 60000) === "14:02" && B.time(T0, T0 + 864e5) === "Yesterday 14:02" && /^\w{3} 14:02$/.test(B.time(T0, T0 + 3 * 864e5)) && B.time(T0, T0 + 30 * 864e5) === "14 Oct 14:02" && B.time(0) === "", "time: 14:02 · Yesterday · weekday · date · unknown");
  const fu = MD.splitFollowups("Done.\n\n<<follow-ups: Open a PR | Run the tests | Show the diff>>", true);
  ok(fu.text === "Done." && fu.followups.length === 3 && fu.followups[0] === "Open a PR", "follow-ups: parsed and cut from the text");
  ok(MD.splitFollowups("Done. <<follow-u", false).text === "Done.", "follow-ups: a half-typed marker is hidden while streaming");
  const hints = w.Hints.build({ connectors: ["google", "github"], projects: ["HomeGlow"], folder: "/home/luke/code/homeglow" });
  ok(hints[0] === "Check my Gmail…" && hints.includes("Check my GitHub notifications…") && hints.includes("Pick up HomeGlow…") && hints.some(h => /homeglow/.test(h)) && hints.length <= w.Hints.MAX, "hints: from connected accounts, recent projects and the open folder");
  ok(w.Hints.build({}).length >= 5 && w.Hints.build({ connectors: ["spotify"] })[0] === "Play something on Spotify…", "hints: generic fallbacks fill the rest");

  // ---- rich replies ----
  const flow = MD.render("1. **Shoot** the products\n2. **List**: write them up\n3. **Launch** it");
  ok(/<ol class="flow">/.test(flow) && (flow.match(/class="fdot"/g) || []).length === 3 && /class="ftitle">Shoot/.test(flow), "rich: numbered bold-title list becomes a step-flow");
  ok(!/flow/.test(MD.render("1. one\n2. two")), "rich: a plain numbered list stays a list");
  ok(/class="rside"/.test(MD.render("![lamp](https://x.test/a.png)\nThe lamp.")), "rich: a picture before a paragraph sits beside it");
  ok(/class="rgroup" style="--n:3"/.test(MD.render("![a](https://x.test/1.png) ![b](https://x.test/2.png) ![c](https://x.test/3.png)")), "rich: up to 3 pictures side by side");
  const deckHtml = MD.render([1, 2, 3, 4, 5].map(i => `![p${i}](https://x.test/${i}.png)`).join("\n"));
  ok(/class="deck"/.test(deckHtml) && (deckHtml.match(/class="dcard"/g) || []).length === 5, "rich: 4+ pictures become a deck");
  ok(/mermaid\.ink\/svg\//.test(MD.render("```mermaid\ngraph TD; A-->B\n```")), "rich: mermaid blocks are drawn inline");
  ok(!/<img/.test(MD.render("![x](http://insecure.test/a.png)")), "rich: only https pictures load");

  // ---- the thread ----
  w.show("work"); await tick(60);
  w.eval("newChat()"); await tick(80);
  const hero = d.querySelector(".hero");
  ok(hero && /Good (morning|afternoon|evening)/.test(hero.querySelector(".big").textContent) && hero.querySelector("img.hmark") && hero.querySelectorAll(".suggest button").length === 4, "empty state: mobile's greeting, the mascot and four rounded tries");
  const A = w.go.main.App, sw = A.SendWith; A.SendWith = async () => "ok";
  await w.eval('sendText("Why is the build slow?", [], [])'); await tick(30);
  const ib = d.querySelector(".inbox"), send = d.querySelector("#send");
  ok(ib.classList.contains("busy") && send.dataset.stop === "true", "composer: a stop square while Blaze works");
  w.fire("tool", { id: "t1", name: "file_read", args: '{"path":"vite.config.ts"}' }); w.fire("tool_done", { id: "t1", result: "ok" });
  w.fire("delta", "Source maps. Fixed.\n\n<<follow-ups: Commit it | Run the tests>>"); w.fire("done", { route: "Groq · x", seconds: 3 }); await tick(40);
  const col = d.querySelector("#log .col");
  w.eval('(() => { const c = document.querySelector("#log .col"); c.append(userMsg("Nice")); c.append(userMsg("Commit it")); })()'); await tick(40);
  const users = [...col.querySelectorAll(":scope > .msg.user")];
  ok(users[0].classList.contains("g-single") && users[1].classList.contains("g-first") && users[2].classList.contains("g-last"), "thread: quick messages from you group, with their own corners");
  ok(!users[1].querySelector(".gm") && !!users[2].querySelector(".gm .gt"), "thread: one timestamp closes the group");
  const bl = col.querySelector(".msg.blaze");
  ok(bl.classList.contains("newest") && bl.querySelectorAll(".gm .ga").length === 3 && /Copy.*Quote.*Retry/.test(bl.querySelector(".gm").textContent), "thread: Blaze's group ends with Copy · Quote · Retry");
  ok(bl.querySelectorAll(".fups .fup").length === 2 && !/follow-ups/.test(bl.querySelector(".md").textContent), "follow-ups: pills under the newest answer, marker hidden");
  bl.querySelector('.gm .ga[data-a="quote"]').click(); await tick(20);
  ok(/^> Source maps\. Fixed\./.test(d.querySelector("#input").value), "thread: Quote puts the answer in the composer");
  ok(ib.classList.contains("has-text") && w.getComputedStyle(send).display !== "none", "composer: text turns the mic into the round send arrow");
  ok(w.getComputedStyle(d.querySelector(".phint")).display === "none", "composer: the hint never sits over typed text");
  d.querySelector("#input").value = ""; d.querySelector("#input").dispatchEvent(new w.Event("input")); await tick(20);
  ok(!ib.classList.contains("has-text") && w.getComputedStyle(d.querySelector("#mic")).display !== "none" && w.getComputedStyle(send).display === "none", "composer: empty shows the mic, not Send");
  let sent = ""; A.SendWith = async t => { sent = t; return "ok"; };
  bl.querySelector(".fups .fup").click(); await tick(30);
  ok(sent === "Commit it", "follow-ups: a pill sends itself");
  w.fire("done", { route: "x" }); await tick(30);
  ok(!bl.classList.contains("newest"), "follow-ups: an older answer gives up the pills");
  ok(!!d.querySelector(".chatpane .jump"), "thread: a round jump-to-newest button");
  // the deck flips
  w.fire("turn", { kind: "user" }); w.fire("delta", [1, 2, 3, 4].map(i => `![p${i}](https://x.test/${i}.png)`).join("\n")); w.fire("done", { route: "x" }); await tick(30);
  const deck = [...d.querySelectorAll(".deck")].pop();
  deck.querySelector(".dnav.next").click();
  ok(deck.dataset.i === "1" && deck.querySelectorAll(".dcard")[1].dataset.o === "0" && deck.querySelector(".dcount").textContent === "2 / 4", "rich: the deck's arrow brings the next picture to the top");
  A.SendWith = sw;

  // ---- composer hints rotate, and stop when focused ----
  const ph = d.querySelector(".phint");
  ok(ib.classList.contains("hinting") && ph.textContent.length > 0, "placeholder: a suggestion shows in the pill");
  d.querySelector("#input").blur(); const before = ph.textContent; w.Hints.tick(); await tick(350);
  ok(ph.textContent !== before || w.Hints.list.length < 2, "placeholder: rotates to the next suggestion");
  d.querySelector("#input").focus(); const now = ph.textContent; w.Hints.tick(); await tick(350);
  ok(ph.textContent === now, "placeholder: stays put while the field is focused");
  d.querySelector("#input").blur();
  w.eval("setApproval('autopilot')"); await tick(20);
  ok(!ib.classList.contains("hinting") && /Autopilot/.test(d.querySelector("#input").placeholder), "placeholder: Autopilot keeps its own text");
  w.eval("setApproval('auto')"); await tick(20);

  const real = errs.filter(e => !/Could not load|Not implemented|canvas|getContext|globalAlpha|ResizeObserver/i.test(e));
  ok(!real.length, "no page errors" + (real.length ? ": " + real.slice(0, 3).join(" | ") : ""));
  console.log(`\n${n - fails}/${n} passed`);
  w.close();
  process.exit(fails ? 1 : 0);
})().catch(e => { console.log("FAIL crashed: " + (e && e.stack || e)); process.exit(1); });
