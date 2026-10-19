// jsdom test for the preview's move/resize handles (picker.js, v0.20 Design studio):
// eight handles, a drag on the element moves it, a corner resizes both ways, a side one way,
// arrow keys nudge after a pause, and the studio's exact values (setbox) apply and write back.
// Run: npm i jsdom (anywhere on NODE_PATH), then node ci/handles.test.js
const fs = require("fs"), path = require("path");
const { JSDOM } = require("jsdom");
const picker = fs.readFileSync(path.join(__dirname, "..", "picker.js"), "utf8");

let fails = 0, n = 0;
const ok = (cond, msg) => { n++; if (!cond) { fails++; console.log("FAIL " + msg); } else console.log("ok   " + msg); };
const tick = ms => new Promise(r => setTimeout(r, ms || 0));

(async () => {
  const dom = new JSDOM(`<!doctype html><body><section class="hero"><h1 class="title">Warm light</h1><p>Copy</p></section></body>`, { runScripts: "dangerously", pretendToBeVisual: true, url: "http://preview.test/" });
  const w = dom.window, d = w.document;
  const got = [];
  w.addEventListener("message", e => got.push(e.data));
  w.eval(picker);
  const h1 = d.querySelector("h1");
  // jsdom has no layout: give the heading a box.
  h1.getBoundingClientRect = () => ({ left: 100, top: 50, right: 500, bottom: 110, width: 400, height: 60 });
  const PE = w.PointerEvent || w.MouseEvent;
  const pe = (type, x, y, o) => new PE(type, Object.assign({ bubbles: true, cancelable: true, clientX: x, clientY: y, button: 0, pointerId: 1 }, o || {}));
  const last = kind => got.filter(m => m && m.blaze === kind).slice(-1)[0];

  w.postMessage({ blaze: "mode", on: true }, "*"); await tick(10);
  h1.dispatchEvent(new w.MouseEvent("click", { bubbles: true, cancelable: true, clientX: 200, clientY: 70 })); await tick(10);
  const sel = last("selected");
  ok(sel && sel.items.length === 1 && sel.items[0].tag === "h1", "click selects the element");
  ok(sel && sel.items[0].box && sel.items[0].box.w === 400 && sel.items[0].box.x === 0, "the selection carries its box (x, y, w, h)");
  const knobs = [...d.querySelectorAll("[data-blaze-ui][data-h]")].map(k => k.dataset.h).sort().join(",");
  ok(knobs === "e,n,ne,nw,s,se,sw,w", "eight resize handles (" + knobs + ")");
  const ui = [...d.querySelectorAll("[data-blaze-ui]")];
  ok(ui.some(u => u.textContent === "400 × 60"), "a size readout under the frame");
  ok(ui.some(u => /^h1\.title$/.test(u.textContent) && u.style.cursor === "move"), "a move tab named after the element");

  // Drag the corner: width and height change, written back once on release.
  got.length = 0;
  const se = d.querySelector("[data-blaze-ui][data-h=se]");
  se.dispatchEvent(pe("pointerdown", 500, 110));
  w.dispatchEvent(pe("pointermove", 540, 130));
  ok(h1.style.width === "440px" && h1.style.height === "80px", "corner drag resizes live (" + h1.style.width + " × " + h1.style.height + ")");
  await tick(5);
  ok(got.some(m => m.blaze === "box"), "the studio hears the box while dragging");
  w.dispatchEvent(pe("pointerup", 540, 130)); await tick(10);
  let ed = last("edit");
  ok(ed && ed.edit.kind === "style" && ed.edit.props.width && ed.edit.props.height && ed.edit.target.tag === "h1", "release writes width and height back (" + JSON.stringify(ed && ed.edit.props) + ")");

  // A side resizes one way only.
  got.length = 0;
  const e = d.querySelector("[data-blaze-ui][data-h=e]");
  e.dispatchEvent(pe("pointerdown", 500, 80)); w.dispatchEvent(pe("pointermove", 450, 80)); w.dispatchEvent(pe("pointerup", 450, 80)); await tick(10);
  ed = last("edit");
  ok(ed && ed.edit.props.width && !("height" in ed.edit.props), "a side handle writes just the width");

  // The west side moves the left edge: width grows and the element shifts left.
  got.length = 0;
  const wk = d.querySelector("[data-blaze-ui][data-h=w]");
  wk.dispatchEvent(pe("pointerdown", 100, 80)); w.dispatchEvent(pe("pointermove", 80, 80)); w.dispatchEvent(pe("pointerup", 80, 80)); await tick(10);
  ed = last("edit");
  ok(ed && ed.edit.props.translate === "-20px 0px" && ed.edit.props.width, "the left handle grows leftwards (" + JSON.stringify(ed && ed.edit.props) + ")");

  // Dragging the element itself moves it, and the trailing click doesn't re-select.
  got.length = 0;
  h1.style.translate = "0px 0px";
  h1.dispatchEvent(pe("pointerdown", 200, 70));
  w.dispatchEvent(pe("pointermove", 230, 95));
  w.dispatchEvent(pe("pointerup", 230, 95));
  h1.dispatchEvent(new w.MouseEvent("click", { bubbles: true, cancelable: true, clientX: 230, clientY: 95 })); await tick(10);
  ed = last("edit");
  ok(ed && ed.edit.props.translate === "30px 25px" && Object.keys(ed.edit.props).length === 1, "dragging the element writes its move (" + JSON.stringify(ed && ed.edit.props) + ")");
  ok(!got.some(m => m.blaze === "selected" && m.items[0] && m.items[0].tag !== "h1"), "the drag's click doesn't change the selection");

  // A click without a drag still selects what's under it (here the heading's parent's sibling).
  const p = d.querySelector("p");
  p.getBoundingClientRect = () => ({ left: 100, top: 120, right: 500, bottom: 140, width: 400, height: 20 });
  got.length = 0;
  p.dispatchEvent(new w.MouseEvent("click", { bubbles: true, cancelable: true, clientX: 150, clientY: 130 })); await tick(10);
  ok(last("selected") && last("selected").items[0].tag === "p", "a plain click selects another element");
  h1.dispatchEvent(new w.MouseEvent("click", { bubbles: true, cancelable: true })); await tick(10);

  // Arrow keys nudge live and write once after a pause.
  got.length = 0;
  h1.style.translate = "10px 10px";
  d.dispatchEvent(new w.KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true, cancelable: true }));
  d.dispatchEvent(new w.KeyboardEvent("keydown", { key: "ArrowDown", shiftKey: true, bubbles: true, cancelable: true }));
  ok(h1.style.translate === "11px 20px", "arrows nudge (Shift ×10): " + h1.style.translate);
  ok(!last("edit"), "no write while still nudging");
  await tick(700);
  ed = last("edit");
  ok(ed && ed.edit.props.translate === "11px 20px", "one write after the pause");

  // Forwarded nudges and exact values from the studio.
  got.length = 0;
  w.postMessage({ blaze: "nudge", dx: -1, dy: 0 }, "*"); await tick(10);
  ok(h1.style.translate === "10px 20px", "studio nudge applies");
  w.postMessage({ blaze: "setbox", box: { w: 333, x: 5 } }, "*"); await tick(30);
  ed = last("edit");
  ok(h1.style.width === "333px" && h1.style.translate === "5px 20px" && ed && ed.edit.props.width === "333px" && ed.edit.props.translate === "5px 20px", "setbox applies and writes exact values");

  // Off: no handles left behind.
  w.postMessage({ blaze: "mode", on: false }, "*"); await tick(10);
  ok(!d.querySelector("[data-blaze-ui][data-h]"), "turning Select off removes the handles");

  console.log(`\n${n - fails}/${n} passed`);
  process.exit(fails ? 1 : 0);
})().catch(e => { console.log("FAIL crashed: " + (e && e.stack || e)); process.exit(1); });
