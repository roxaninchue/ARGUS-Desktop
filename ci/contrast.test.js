// WCAG AA contrast guard (v0.19.8), plain node. Resolves the theme variables from style.css for
// every theme (light = :root, dark = :root[data-theme=dark]), composites translucent colours over
// the window ground, and checks text/background pairs at 4.5:1 (normal text) or 3:1 (large /
// secondary UI). The user bubble pair is read from the LAST stylesheet that styles it (chat.css
// loads last), so a regression like var(--paper) on var(--ink) fails here.
const fs = require("fs"), path = require("path");
const dist = path.join(__dirname, "..", "frontend", "dist");
let fails = 0, n = 0;
const ok = (c, m) => { n++; if (!c) { fails++; console.log("FAIL " + m); } else console.log("ok   " + m); };
const css = f => fs.readFileSync(path.join(dist, f), "utf8").replace(/\/\*[\s\S]*?\*\//g, "");
const style = css("style.css");
const block = sel => { const i = style.indexOf(sel + " {"); if (i < 0) return {}; const b = style.slice(style.indexOf("{", i) + 1, style.indexOf("}", i)); const v = {}; for (const m of b.matchAll(/--([\w-]+)\s*:\s*([^;]+);/g)) v[m[1]] = m[2].trim(); return v; };
const light = block(":root"), themes = { light, dark: { ...light, ...block(":root[data-theme=dark]") } };

function parse(c, vars) {
  c = c.trim();
  for (let i = 0; i < 5 && /var\(/.test(c); i++) c = c.replace(/var\(--([\w-]+)\)/, (_, k) => vars[k] || "");
  if (/^#[0-9a-f]{3}$/i.test(c)) c = "#" + c.slice(1).split("").map(x => x + x).join("");
  let m = c.match(/^#([0-9a-f]{6})$/i);
  if (m) { const v = parseInt(m[1], 16); return [v >> 16, (v >> 8) & 255, v & 255, 1]; }
  m = c.match(/^rgba?\(([^)]+)\)$/i);
  if (m) { const p = m[1].split(",").map(Number); return [p[0], p[1], p[2], p.length > 3 ? p[3] : 1]; }
  return null;
}
const over = (top, under) => { const a = top[3]; return [0, 1, 2].map(i => top[i] * a + under[i] * (1 - a)).concat(1); };
const lum = c => { const f = v => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); }; return 0.2126 * f(c[0]) + 0.7152 * f(c[1]) + 0.0722 * f(c[2]); };
const ratio = (a, b) => { const x = lum(a), y = lum(b); return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05); };

// The user bubble as the cascade ends: the last rule for ".msg.user .bubble" (chat.css loads last).
const order = [...fs.readFileSync(path.join(dist, "index.html"), "utf8").matchAll(/href="([\w-]+\.css)"/g)].map(m => m[1]);
let bubbleBg = null, bubbleFg = null;
for (const f of order) {
  for (const m of css(f).matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const sels = m[1].split(",").map(s => s.trim());
    if (!sels.some(s => s === ".msg.user .bubble" || s === "body.shell .msg.user .bubble")) continue;
    const bg = m[2].match(/(?:^|;)\s*background\s*:\s*([^;]+)/), fg = m[2].match(/(?:^|;)\s*color\s*:\s*([^;]+)/);
    if (bg && !/gradient/.test(bg[1])) bubbleBg = bg[1].trim();
    if (fg) bubbleFg = fg[1].trim();
  }
}
ok(bubbleBg && bubbleFg, `found the user bubble rule (${bubbleBg} / ${bubbleFg})`);

const chat = css("chat.css");
const ruleOf = sel => { const m = chat.match(new RegExp(sel.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "\\s*\\{([^}]*)\\}")); return m ? m[1] : ""; };
const prop = (r, k) => ((r.match(new RegExp("(?:^|;)\\s*" + k + "\\s*:\\s*([^;]+)")) || [])[1] || "").trim();
const pillRule = ruleOf(".msg.blaze > .wpill"), dstatRule = ruleOf(".msg.user > .dstat");
const pillFg = prop(pillRule, "color"), pillBg = prop(pillRule, "background"), dstatFg = prop(dstatRule, "color");
ok(/^var\(--/.test(pillFg) && /^var\(--/.test(pillBg) && /^var\(--/.test(dstatFg), `pill and status use theme variables (${pillFg} on ${pillBg}; ${dstatFg})`);

for (const [name, v] of Object.entries(themes)) {
  const ground = parse("var(--ground)", v);
  const on = (fg, bg, min, what) => {
    const b = over(parse(bg, v), ground), f = over(parse(fg, v), b), r = ratio(f, b);
    ok(r >= min, `${name}: ${what} ${r.toFixed(2)}:1 (needs ${min})`);
  };
  on(bubbleFg, bubbleBg, 4.5, "your chat bubble text");
  on("var(--ink)", "var(--ground)", 4.5, "headline text on the window");
  on("var(--body)", "var(--ground)", 4.5, "body text on the window");
  on("var(--body)", "var(--bubble)", 4.5, "Blaze's reply text on its bubble");
  on("var(--sub)", "var(--ground)", 4.5, "secondary text on the window");
  on("var(--on-accent)", "var(--accent)", 4.5, "button text on accent");
  on("var(--on-primary)", "var(--primary)", 4.5, "primary button text");
  on("var(--weak)", "var(--ground)", 1.8, "hint text (placeholder-level, not AA-bound)");
  // 0.19.16 working pill and delivery status, with the colours their rules use (read from chat.css).
  on(pillFg, pillBg, 4.5, "working pill text on its pill");
  on(dstatFg, "var(--ground)", 4.5, "delivery status (Sending / Sent / Read)");
  on(dstatFg, "var(--bubble)", 4.5, "delivery status over a reply bubble's glass");
}
console.log(`\n${n - fails}/${n} passed`);
process.exit(fails ? 1 : 0);
