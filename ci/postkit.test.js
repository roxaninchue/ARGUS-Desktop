// jsdom test for the Post kit view (v0.19.11): open from /views and the command, an idea makes a
// kit, a photo makes a kit with crops, copy, export, errors, a stale answer ignored, and no
// provider or model names anywhere in the view.
// Run: node ci/postkit.test.js (jsdom on NODE_PATH)
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
  let copied = null;
  w.runtime = Object.assign(w.runtime || {}, { ClipboardSetText: async t => { copied = t; return true; } });
  const $ = s => d.querySelector(s), $$ = s => [...d.querySelectorAll(s)];

  // 1. Listed in /views and as a command; opens.
  ok(w.BlazeViews.find("postkit") && w.BlazeViews.find("postkit").name === "Post kit", "Post kit is in /views");
  ok(w.eval('COMMANDS.some(c => c.name === "postkit")'), "/postkit command exists");
  w.eval('show("postkit")'); await tick(80);
  ok($("#view h1") && /Post kit/.test($("#view h1").textContent), "view opens with its title");
  ok($$(".pk-plat.on").map(b => b.dataset.id).join(",") === "x,instagram,tiktok", "default platforms on: X, Instagram, TikTok");
  ok($(".pk-make").disabled, "Make kit is off until there's a photo or an idea");

  // 2. Idea path.
  const ta = $(".pk-idea"); ta.value = "A week without my phone"; ta.dispatchEvent(new w.Event("input"));
  ok(!$(".pk-make").disabled, "typing an idea enables Make kit");
  $('.pk-plat[data-id="reddit"]').click(); await tick();
  ok($('.pk-plat[data-id="reddit"]').classList.contains("on"), "Reddit toggled on");
  $(".pk-lang").value = "fr"; $(".pk-lang").dispatchEvent(new w.Event("change"));
  $(".pk-make").click(); await tick(10);
  ok(/Making/.test($(".pk-make").textContent) && $(".pk-make").disabled, "busy while making");
  await tick(80);
  const req = w.__pkReq;
  ok(req && req.idea === "A week without my phone" && req.photo === "" && req.language === "fr" && [...req.platforms].sort().join(",") === "instagram,reddit,tiktok,x", "request carries idea, language and platforms");
  ok($(".pk-kit") && $$(".pk-card").length === 4, "kit shows a card per platform");
  ok($$(".pk-card").map(c => c.dataset.id).join(",") === "x,instagram,tiktok,reddit", "cards in picker order");
  ok(/I gave my phone a week off/.test($(".pk-top").textContent) && /#digitaldetox #SlowLiving #Lusaka/.test($(".pk-top").textContent), "hook and hashtags shown");
  ok(/ \/ 125/.test($(".pk-top").textContent) && /\/ 12 words/.test($(".pk-top").textContent), "alt text and hook counters");
  ok(/\/ 280/.test($('.pk-card[data-id="x"] .pk-count').textContent), "X counter against 280");
  ok($('.pk-card[data-id="reddit"] .pk-title').textContent === "A week without my phone", "Reddit title shown");
  ok(/Shot guide/.test($(".pk-crops").textContent) && $$(".pk-frame").length === 4 && !$(".pk-frame img"), "idea: shot guide frames, no photo");
  $('.pk-card[data-id="x"] .pk-copy').click(); await tick();
  ok(copied && copied.startsWith("I gave my phone a week off"), "copy a caption");
  [...$$(".pk-acts .pk-copy")].find(b => b.textContent === "Copy JSON").click(); await tick();
  ok(copied && JSON.parse(copied).schema === "blaze.postkit/1", "copy JSON gives a blaze.postkit/1 kit");
  $(".pk-export").click(); await tick(40);
  ok(w.__pkExport && w.__pkExport.photo === "" && /Saved 2 files/.test($(".pk-saved").textContent), "export an idea kit");

  // 3. Photo path.
  $(".pk-add").click(); await tick(40);
  ok($(".pk-photo.has") && /1200 × 800/.test($(".pk-photo").textContent), "photo added with its size");
  $(".pk-make").click(); await tick(100);
  ok(w.__pkReq.photo === "/home/luke/Pictures/market.jpg", "photo path sent");
  ok(/Crops/.test($(".pk-crops").textContent) && $$(".pk-frame img").length === 4, "photo: crops drawn over the photo");
  const sq = $('.pk-frame[data-ratio="1:1"] img');
  ok(sq && sq.style.width === "150%" && sq.style.left === "-50%", "1:1 crop positioned from the kit's rectangle");
  $(".pk-export").click(); await tick(40);
  ok(w.__pkExport.photo === "/home/luke/Pictures/market.jpg" && /Saved 6 files/.test($(".pk-saved").textContent), "export with photo saves crops too");

  // 4. Errors keep the form, and a stale answer is ignored.
  w.__pkResult = { error: "Blaze couldn't reach the model right now. Try again in a moment." };
  $(".pk-make").click(); await tick(80);
  ok(/couldn't reach/.test($(".pk-err").textContent) && !$(".pk-kit"), "error shown, no kit");
  w.__pkResult = null;
  $(".pk-make").click(); await tick(5);
  w.PostKit.state.req++; // the user started another request meanwhile
  await tick(80);
  ok(/Making/.test($(".pk-make").textContent), "a late answer for a replaced request is ignored");

  // 5. Never a provider or model name.
  const text = d.getElementById("view").textContent;
  ok(!/groq|gemini|openai|llama|mistral|cerebras|sambanova|openrouter|gpt-|qwen|deepseek/i.test(text), "no provider or model names in the view");
  ok(!errs.length, "no script errors" + (errs.length ? ": " + errs.slice(0, 3).join(" | ") : ""));
  console.log(`${n - fails}/${n} passed`);
  process.exit(fails ? 1 : 0);
})();
