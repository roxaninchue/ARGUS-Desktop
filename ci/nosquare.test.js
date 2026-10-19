// "No square UI" guard (port of Blaze-agent's ui/NoSquareTest, v0.24.8 / desktop v0.17.1).
// Fails if any stylesheet in frontend/dist sets a zero corner radius, brings back the old global
// `* { border-radius: 0 }` reset or square icon caps, if a script writes a zero radius inline,
// or if one of the core components stops declaring a radius. Plain node, no packages:
//   node ci/nosquare.test.js
const fs = require("fs"), path = require("path");
const dist = path.join(__dirname, "..", "frontend", "dist");
let fails = 0, n = 0;
const ok = (cond, msg) => { n++; if (!cond) { fails++; console.log("FAIL " + msg); } else console.log("ok   " + msg); };

// Selectors allowed a zero radius (scrollbars, 1px hairlines). Keep it tiny; empty today.
const ALLOW = [];

const strip = s => s.replace(/\/\*[\s\S]*?\*\//g, "");
const rules = []; // {file, sel, body}
for (const f of fs.readdirSync(dist).filter(f => f.endsWith(".css"))) {
  const css = strip(fs.readFileSync(path.join(dist, f), "utf8"));
  // Innermost "selector { declarations }" pairs; @media / @container wrappers fall away.
  const re = /([^{}]+)\{([^{}]*)\}/g; let m;
  while ((m = re.exec(css))) rules.push({ file: f, sel: m[1].trim().replace(/^@[^{]*$/, ""), body: m[2] });
}
ok(rules.length > 300, `parsed ${rules.length} rules from frontend/dist/*.css`);

const zero = v => v.replace(/!important/, "").trim().split(/[\s/]+/).some(t => /^[-+]?0(\.0+)?(px|%|em|rem|vw|vh)?$/.test(t));
const bad = [];
for (const r of rules) {
  const decls = [...r.body.matchAll(/(border(?:-(?:top|bottom|start|end)-(?:left|right|start|end))?-radius)\s*:\s*([^;]+)/gi)];
  for (const d of decls) if (zero(d[2]) && !ALLOW.some(a => r.sel.split(",").map(s => s.trim()).includes(a))) bad.push(`${r.file}: ${r.sel} { ${d[1]}: ${d[2].trim()} }`);
}
ok(!bad.length, "no zero border-radius in any stylesheet" + (bad.length ? ":\n  " + bad.join("\n  ") : ""));

const reset = rules.filter(r => r.sel.split(",").some(s => s.trim() === "*") && /border-radius/.test(r.body));
ok(!reset.length, "no global * { border-radius } reset" + (reset.length ? ": " + reset.map(r => r.file).join(", ") : ""));
const caps = rules.filter(r => /stroke-linecap\s*:\s*square|stroke-linejoin\s*:\s*miter/.test(r.body));
ok(!caps.length, "icons have round caps (no stroke-linecap: square)" + (caps.length ? ": " + caps.map(r => r.file + " " + r.sel).join(", ") : ""));

const inline = [];
for (const f of fs.readdirSync(dist).filter(f => f.endsWith(".js"))) {
  strip(fs.readFileSync(path.join(dist, f), "utf8")).split("\n").forEach((l, i) => {
    if (/border-?radius\s*[:=]\s*["'`]?\s*0(px|%)?\s*(["'`;!]|$)/i.test(l) || /borderRadius\s*=\s*["'`]0/.test(l)) inline.push(`${f}:${i + 1}`);
  });
}
ok(!inline.length, "no script sets a zero radius inline" + (inline.length ? ": " + inline.join(", ") : ""));

// Every core component still declares a radius somewhere (the audit list from mobile v0.24.8, desktop names).
const MUST = [".btn", ".field", ".inbox", ".sendbtn", ".pop", ".menu", ".opt", ".card", ".acard", ".code", ".box", ".rows", ".kv",
  ".chat", ".nav", ".snav button", ".tabs", ".tabs button", ".chip", ".seg", ".seg button", ".mseg", ".toggle input", ".pill",
  ".suggest button", ".msg.user .bubble", ".msg.blaze > .md:not(:empty)", ".fup", ".jump", ".shavatar", ".shexit", ".dot", ".work .sq",
  ".pframe", ".vtile", ".selbar", ".dempty", ".picker", ".agcard", ".specs", ".abox", ".toast", ".brcard", ".achip", ".callout",
  ".kbd", ".logo", ".md ol.flow .fdot", ".md .deck .dcard", ".step .st::before"];
const has = sel => rules.some(r => /border-radius/.test(r.body) && r.sel.split(",").map(s => s.trim().replace(/\s+/g, " ")).includes(sel));
const missing = MUST.filter(s => !has(s));
ok(!missing.length, `all ${MUST.length} core components are rounded` + (missing.length ? "; missing: " + missing.join(", ") : ""));

console.log(`\n${n - fails}/${n} passed`);
process.exit(fails ? 1 : 0);
