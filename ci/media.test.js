// v0.19 Blaze Music / Video: the player's pure rules (frontend/dist/media-logic.js). Run: node ci/media.test.js
const assert = require("assert");
const L = require("../frontend/dist/media-logic.js");
let n = 0; const t = (name, fn) => { fn(); n++; };

t("shuffle starts with the tapped song and is stable for a seed", () => {
  const o = L.shuffleOrder(6, 4, 99);
  assert.strictEqual(o[0], 4); assert.strictEqual(o.length, 6); assert.deepStrictEqual([...o].sort(), [0, 1, 2, 3, 4, 5]);
  assert.deepStrictEqual(L.shuffleOrder(6, 4, 99), o);
});
t("next / previous with repeat", () => {
  assert.strictEqual(L.nextIndex(4, 5, "off"), -1); assert.strictEqual(L.nextIndex(4, 5, "all"), 0); assert.strictEqual(L.nextIndex(2, 5, "one"), 2);
  assert.strictEqual(L.prevIndex(0, 5, "off"), -1); assert.strictEqual(L.prevIndex(0, 5, "all"), 4);
  assert.strictEqual(L.nextIndex(1, 3, "off", [2, 1, 0]), 0);
  assert.deepStrictEqual(L.upNext(3, 5, "all", null, 10), [4, 0, 1, 2]);
  assert.strictEqual(L.nextRepeat("off"), "all"); assert.strictEqual(L.nextRepeat("one"), "off");
});
t("fades and sleep", () => {
  assert.strictEqual(L.fadeVolume(0, 300000, 6000), 0.05); assert.strictEqual(L.fadeVolume(150000, 300000, 6000), 1); assert.strictEqual(L.fadeVolume(0, 300000, 0), 1);
  assert.strictEqual(L.sleepVolume(15000, 30000), 0.5); assert.strictEqual(L.sleepVolume(60000), 1);
});
t("lyrics line", () => {
  const l = [{ ms: 1000 }, { ms: 5000 }, { ms: 9000 }];
  assert.strictEqual(L.lrcCurrent(l, 0), -1); assert.strictEqual(L.lrcCurrent(l, 6000), 1); assert.strictEqual(L.lrcCurrent(l, 1e9), 2);
});
t("video rules", () => {
  assert.strictEqual(L.resumeAt(3000, 60000), 0); assert.strictEqual(L.resumeAt(55000, 60000), 0); assert.strictEqual(L.resumeAt(30000, 60000), 30000);
  assert.ok(L.showSkip(10000, 90000)); assert.ok(!L.showSkip(1000, 90000)); assert.ok(!L.showSkip(88000, 90000));
  assert.strictEqual(L.countdown(0, 1200), 4); assert.strictEqual(L.countdown(0, 6000), -1);
  assert.strictEqual(L.nextSpeed(1), 1.25); assert.strictEqual(L.nextSpeed(2), 0.5); assert.strictEqual(L.speedLabel(1.5), "1.5\u00d7");
  assert.deepStrictEqual(L.episode("The.Office.S02E05.720p"), { series: "The Office", season: 2, episode: 5 });
  assert.deepStrictEqual(L.episode("Lessons - Episode 3"), { series: "Lessons", season: 1, episode: 3 });
  assert.strictEqual(L.episode("A Film"), null);
});
t("clock, search, groups", () => {
  assert.strictEqual(L.clock(65000), "1:05"); assert.strictEqual(L.clock(3725000), "1:02:05"); assert.strictEqual(L.clock(), "0:00");
  const s = [{ t: "Slow Morning", a: "Ana" }, { t: "Piano Rain", a: "Ana", al: "Calm" }, { t: "Party", a: "Ben" }];
  assert.deepStrictEqual(L.search(s, "ana piano").map(x => x.t), ["Piano Rain"]);
  assert.deepStrictEqual(L.artists(s).map(g => [g[0], g[1].length]), [["Ana", 2], ["Ben", 1]]);
  assert.deepStrictEqual(L.albums(s).map(g => g[0]), ["Ana", "Ben", "Calm"]);
});
console.log(`media: ${n} groups ok`);
