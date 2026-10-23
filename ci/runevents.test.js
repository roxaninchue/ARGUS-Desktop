// 0.19.16 run events adapter, sanitizer, 2 s pill clock, delivery transitions. Run: node ci/runevents.test.js
const R = require("../frontend/dist/runEvents.js");
const P = require("../frontend/dist/privacy.js");
let fails = 0, n = 0;
const ok = (c, m) => { n++; if (!c) { fails++; console.log("FAIL " + m); } else console.log("ok   " + m); };
const eq = (a, b, m) => ok(a === b, `${m} (got ${JSON.stringify(a)}, want ${JSON.stringify(b)})`);
const LEAK = /groq|cerebras|sambanova|cohere|mistral|gemini|openrouter|nvidia|\bnim\b|z\.ai|glm|llama|qwen|deepseek|gpt|openai|anthropic|claude|kimi|hugging ?face|luke|lackson|kapapa/i;

// sanitizer
eq(R.sanitize("Reading your CSV"), "Reading your CSV", "plain step kept");
eq(R.sanitize('run_shell {"command":"ls -la ~"}'), "", "raw tool JSON dropped");
eq(R.sanitize('Reading {"path":"/x/a.csv"}'), "Reading", "args dropped");
eq(R.sanitize("Asking Groq llama-3.3-70b"), "", "provider/model line dropped");
eq(R.sanitize("Handing this to Luke"), "", "owner name dropped");
eq(R.sanitize("file_read(path=a) Reading your notes\nsecond line"), "Reading your notes", "call + tool id + second line dropped");
const long = R.sanitize("Reading every single file in your project folder ".repeat(4));
ok([...long].length <= 60 && long.endsWith("…"), "clipped to 60 chars: " + long.length);

// step mapping
eq(R.stepOf("tool", { name: "file_read", args: '{"path":"/home/a/sales.csv"}' }), "Reading sales.csv", "local tool -> step from local intent");
eq(R.stepOf("tool", { name: "web_search", args: "{}" }), "Searching the web", "web search step");
eq(R.stepOf("tool", { name: "csv_report", args: '{"path":"/d/a.csv"}' }), "Making a report from a.csv", "no tool name in the pill, names the file (was 'Using csv report')");
eq(R.stepOf("tool", { name: "zz_unknown_tool", args: "{}" }), "Working on it", "unknown tool: generic line, never its name");
// specialties / specialists: what the help is for, from the event's own detail; never which, never a fixed label
const STATIC = /^(Bringing in a specialist|Updating a specialist|Using a specialty|Updating a specialty)$/;
eq(R.stepOf("tool", { name: "dispatch_specialist", args: '{"name":"build-fixer","brief":"Fix the build error in main.go. Files: main.go"}' }), "Getting help to fix the build error in main.go", "specialist: its brief's goal, not its name");
eq(R.stepOf("tool", { name: "dispatch_specialist", args: '{"name":"data-analyst","brief":"Chart sales from cafe-sales-sept.csv"}' }), "Getting help to chart sales from cafe-sales-sept.csv", "specialist: keeps the real file name");
eq(R.stepOf("tool", { name: "apply_specialist", args: '{"name":"data-analyst","action":"apply"}' }), "Bringing in the reviewed changes", "specialist apply: what happens, never which");
eq(R.stepOf("run_event", { type: "specialty_loaded", text: "Using a specialty", detail: "fix the build error" }), "Getting help to fix the build error", "specialty_loaded: from its detail");
eq(R.stepOf("run_event", { type: "step", kind: "specialty", text: "Using a specialty", task: "Planning the Lusaka trip" }), "Getting help with planning the Lusaka trip", "specialty step: from its task");
eq(R.stepOf("run_event", { type: "specialty_updated", text: "Updating a specialty", detail: "Always book aisle seats", name: "flights" }), "Saving what I learned: always book aisle seats", "specialty_updated: what was learned");
eq(R.stepOf("run_event", { type: "specialty_loaded", text: "Using a specialty" }, "Book me a table for two at 8"), "Getting help to book me a table for two at 8", "no detail: what the request is for");
eq(R.stepOf("run_event", { type: "specialty_loaded", text: "data-analyst via groq", name: "data-analyst" }), "Bringing in a specialist", "nothing usable at all: the general line, last resort, never which");
eq(R.stepOf("run_event", { type: "specialty_updated", name: "csv-pro" }), "Updating a specialist", "specialty_updated with nothing: last resort");
eq(R.stepOf("run_event", { type: "specialty_loaded", text: "Getting help to fix the build error" }, "anything"), "Getting help to fix the build error", "a line the adapter already built is kept, not wrapped twice");
{
  const evs = [["fix the build error", "Getting help to fix the build error"], ["chart revenue by store", "Getting help to chart revenue by store"], ["Writing your summary", "Getting help with writing your summary"]];
  const got = evs.map(([d]) => R.stepOf("run_event", { type: "specialty_loaded", text: "Using a specialty", detail: d, name: "data-analyst", _name: "csv-pro" }));
  ok(new Set(got).size === evs.length && got.every((g, i) => g === evs[i][1]), "pills vary with the actual detail: " + JSON.stringify(got));
  ok(got.every(g => !STATIC.test(g) && !/data-analyst|csv-pro|specialt|specialist/i.test(g) && [...g].length <= 60), "no static specialist phrase and no name when detail exists");
  const leaky = R.stepOf("run_event", { type: "specialty_loaded", detail: "fix the build error with data-analyst (groq llama)", name: "data-analyst" });
  ok(leaky === "Getting help to fix the build error" && !LEAK.test(leaky), "detail with the name/provider: both dropped: " + leaky);
}
// tool steps name the real object, and vary with it
eq(R.stepOf("tool", { name: "web_search", args: '{"query":"Lusaka weather"}' }), "Searching the web for Lusaka weather", "web search names its query");
eq(R.stepOf("tool", { name: "file_read", args: '{"path":"/h/cafe-sales-sept.csv"}' }), "Reading cafe-sales-sept.csv", "file read names the file");
eq(R.stepOf("tool", { name: "file_read", args: '{"path":"/h/my_data.csv"}' }), "Reading my_data.csv", "a snake_case file name is kept (only tool names go)");
eq(R.stepOf("tool", { name: "csv_report", args: '{"path":"/h/cafe.csv","group_by":"store","value":"revenue"}' }), "Charting revenue by store", "csv report: what it charts");
eq(R.stepOf("tool", { name: "csv_summary", args: '{"report":"r1","summary":"x"}' }), "Writing your summary", "csv summary: writing");
eq(R.stepOf("tool", { name: "get_weather", args: '{"place":"Lusaka"}' }), "Checking the weather in Lusaka", "weather names the place");
eq(R.stepOf("tool", { name: "memory_search", args: '{"query":"sister"}' }), "Checking what I remember about sister", "memory search names the topic");
eq(R.stepOf("tool", { name: "schedule_add", args: '{"text":"call mum","at":"17:00"}' }), "Setting a reminder to call mum", "reminder names what");
ok(new Set(["a.csv", "b.csv", "c.md"].map(f => R.stepOf("tool", { name: "file_read", args: JSON.stringify({ path: "/x/" + f }) }))).size === 3, "pills vary with the actual file");
eq(R.stepOf("run_event", { type: "step", text: 'web_search {"q":"x"}' }), "", "step with raw JSON shows nothing");
eq(R.stepOf("run_event", { type: "step", text: "Searching the web" }), "Searching the web", "step text");
eq(R.stepOf("status", "Asking SambaNova"), "", "status naming a provider shows nothing");
eq(R.stepOf("delta", "hello"), "", "reply text is not a step");

// 2 s threshold
const p = R.pill(1000);
ok(!p.visible(1500), "no pill without a step");
p.set("Reading your CSV");
ok(!p.visible(2999), "no pill before 2 s");
ok(p.visible(3000), "pill at 2 s");
ok(!p.set("Reading your CSV") && p.set("Searching the web") && p.text === "Searching the web", "replaced in place, same text is no change");

// delivery transitions: run path
let m = R.message();
eq(R.statusText(m), "Sending", "starts Sending");
m.apply("run_event", { type: "message_sending", client_msg_id: "c1", attempt: 1 });
m.apply("thinking"); eq(m.state, "sending", "run path: local thinking does not mark Sent");
m.apply("run_event", { type: "message_sending", client_msg_id: "c1", attempt: 2, offline: true });
ok(m.state === "sending" && m.offline, "offline: stays Sending");
m.apply("run_event", { type: "message_sending", client_msg_id: "OTHER", attempt: 3 });
eq(m.cmid, "c1", "another message's events ignored");
m.apply("run_event", { type: "message_received", client_msg_id: "c1", run_id: "run_1" });
eq(R.statusText(m), "Sent", "202 -> Sent");
ok(!m.offline, "offline cleared once sent");
m.apply("run_event", { type: "message_read", client_msg_id: "c1", run_id: "run_1" });
eq(R.statusText(m), "Read", "first run event -> Read");
m.apply("run_event", { type: "message_received", client_msg_id: "c1" });
eq(m.state, "read", "never goes backwards");

// classic stub
m = R.message();
m.apply("thinking"); eq(m.state, "sending", "classic: getting ready is still Sending (it happens offline too)");
m.apply("delta", "Hi"); eq(m.state, "read", "classic: first words -> Read");
m = R.message(); m.apply("thinking");
m.apply("error", { message: "Blaze can't reach Blaze Cloud. Check your internet connection." });
ok(m.state === "sending" && m.offline, "classic offline after thinking: stays Sending");
m = R.message();
m.apply("error", { message: "Blaze can't reach Blaze Cloud. Check your connection." });
ok(m.state === "sending" && m.offline, "classic: network error stays Sending");
m = R.message(); m.apply("done", {}); eq(m.state, "read", "a reply always means Read");

// privacy helpers
ok(P.leaks("Groq didn't answer, so Cerebras did") && P.note("Groq didn't answer, so Cerebras did") === "", "fallback notice dropped");
ok(!LEAK.test(P.scrub("SambaNova 429: Meta-Llama-3.3-70B-Instruct is busy")), "error line scrubbed");
ok(P.route("Groq · llama-3.3-70b") === "Blaze" && P.route("Blaze Cloud") === "Blaze Cloud", "route label");
ok(!P.leaks("Sign in to Google again") && !P.leaks("nimble glimpse"), "no false positives");
global.BLAZE_DEV = undefined;
console.log(`\n${n - fails}/${n} passed`);
process.exit(fails ? 1 : 0);
