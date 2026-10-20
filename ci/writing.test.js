// jsdom test for the writing tools (v0.19.10): select in the message box or a reply, Rewrite or
// Translate, Accept / Undo / Reject, stale answers ignored, errors change nothing.
// Run: node ci/writing.test.js (jsdom on NODE_PATH)
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

    },
  });
  const w = dom.window, d = w.document;
  if (d.readyState !== "complete") await new Promise(r => w.addEventListener("load", r));
  const tick = (ms = 30) => new Promise(r => setTimeout(r, ms));
  for (let i = 0; i < 100 && !d.querySelector("#input"); i++) await tick(30);
  w.eval('show("work")'); await tick(80);
  let copied = null;
  w.runtime = { ClipboardSetText: async t => { copied = t; return true; } };
  const ta = d.getElementById("input");
  // 1. Message box: select a word, Rewrite, Accept replaces exactly that range, Undo restores.
  ta.value = "Dear team, pls send the report by friday. Thanks"; ta.focus();
  ta.setSelectionRange(11, 41);
  d.dispatchEvent(new w.MouseEvent("mouseup")); await tick(20);
  const bar = d.querySelector(".wbar");
  ok(!!bar, "selection bar appears for a selection in the message box");
  bar.querySelector(".wb-rw").click(); await tick(10);
  const item = [...d.querySelectorAll(".wmenu button")].find(b => b.textContent === "More formal");
  ok(!!item, "tone menu lists tones");
  item.click(); await tick(80);
  ok(w.__wa && w.__wa.text === "pls send the report by friday." && w.__wa.mode === "rewrite" && w.__wa.arg === "More formal", "sends exactly the selection with the tone");
  const card = d.querySelector(".wcard");
  ok(card && /pls send the report/.test(card.querySelector(".wc-orig").textContent) && /\[More formal\]/.test(card.querySelector(".wc-new").textContent), "card shows the original and the new version");
  card.querySelector(".wc-ok").click(); await tick(20);
  ok(ta.value === "Dear team, [More formal] pls send the report by friday. Thanks", "Accept replaces only the selected text");
  d.querySelector(".wundo button").click(); await tick(10);
  ok(ta.value === "Dear team, pls send the report by friday. Thanks", "Undo puts the original back");
  // 2. The box changed while waiting: still replaces the same text, wherever it moved.
  ta.focus(); ta.setSelectionRange(0, 9); d.dispatchEvent(new w.MouseEvent("mouseup")); await tick(20);
  d.querySelector(".wbar .wb-tr").click(); await tick(10);
  [...d.querySelectorAll(".wmenu button")].find(b => b.textContent === "French").click();
  ta.value = "Hi! " + ta.value; await tick(80);
  d.querySelector(".wcard .wc-ok").click(); await tick(20);
  ok(ta.value === "Hi! [French] Dear team, pls send the report by friday. Thanks", "text that moved is still replaced in the right place");
  // 3. An error changes nothing and offers Try again.
  w.__waResult = { error: "Blaze couldn't keep every link or placeholder in that text, so nothing was changed." };
  const before = ta.value; ta.focus(); ta.setSelectionRange(4, 15); d.dispatchEvent(new w.MouseEvent("mouseup")); await tick(20);
  d.querySelector(".wbar .wb-rw").click(); await tick(10); d.querySelector(".wmenu button").click(); await tick(80);
  const c3 = d.querySelector(".wcard");
  ok(c3.querySelector(".wc-ok").disabled && /nothing was changed/.test(c3.textContent) && !c3.querySelector(".wc-again").disabled && ta.value === before, "an error changes nothing and offers Try again");
  // 4. Reject while working: the late answer is ignored.
  w.__waResult = null;
  c3.querySelector(".wc-again").click(); await tick(1);
  d.querySelector(".wcard .wc-no").click(); await tick(60);
  ok(!d.querySelector(".wcard") && ta.value === before, "a rejected request's late answer is ignored");
  // 5. From a reply: Accept copies the result for the user's document.
  await w.eval('sendText("draft", [], [])'); await tick(30);
  w.fire("delta", "The quick brown fox jumps over the lazy dog."); w.fire("done", { route: "Blaze Cloud" }); await tick(30);
  const bub = [...d.querySelectorAll(".msg.blaze .md")].find(b => /quick brown fox/.test(b.textContent));
  const rng = d.createRange(); rng.selectNodeContents(bub); const s = w.getSelection(); s.removeAllRanges(); s.addRange(rng);
  ta.blur(); d.dispatchEvent(new w.MouseEvent("mouseup")); await tick(20);
  ok(!!d.querySelector(".wbar"), "selection bar appears for text in a reply");
  d.querySelector(".wbar .wb-tr").click(); await tick(10);
  [...d.querySelectorAll(".wmenu button")].find(b => b.textContent === "Swahili").click(); await tick(80);
  d.querySelector(".wcard .wc-ok").click(); await tick(30);
  ok(copied && /^\[Swahili\] The quick brown fox/.test(copied), "Accept from a reply copies the result (" + JSON.stringify(copied) + " / " + JSON.stringify(w.__wa) + ")");
  ok(!errs.length, "no page errors" + (errs.length ? ": " + errs.slice(0, 3).join(" | ") : ""));
  console.log(`\n${n - fails}/${n} passed`);
  process.exit(fails ? 1 : 0);
})().catch(e => { console.log("FAIL crashed: " + (e && e.stack || e)); process.exit(1); });
