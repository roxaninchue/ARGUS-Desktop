// blaze-auth 0.9.1: emailed two-factor codes.
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import worker from "../src/index";
import { b64url } from "../src/util";
import { base32Decode, base32Encode, totpAt, timeStep } from "../src/mfa";
import type { Env } from "../src/util";

class FakeKV {
  m = new Map<string, { v: string; exp: number }>();
  async get(k: string, type?: string): Promise<any> {
    const e = this.m.get(k);
    if (!e || (e.exp && e.exp < Date.now())) return null;
    return type === "json" ? JSON.parse(e.v) : e.v;
  }
  async put(k: string, v: string, o?: { expirationTtl?: number }) {
    this.m.set(k, { v, exp: o && o.expirationTtl ? Date.now() + o.expirationTtl * 1000 : 0 });
  }
  async delete(k: string) {
    this.m.delete(k);
  }
  keys(prefix: string) {
    return [...this.m.keys()].filter(k => k.startsWith(prefix));
  }
}

const BASE = "https://blaze-auth.blaze-app.workers.dev";
const SITE = "https://blazeinchue.pages.dev";
const TOKEN = "internal-token-0123456789abcdefghijkl";
let kv: FakeKV;
let env: Env;
let ipN = 0;
let mails: { url: string; token: string | null; body: any }[];

function call(method: string, path: string, body?: unknown, headers: Record<string, string> = {}) {
  const h: Record<string, string> = { "CF-Connecting-IP": `10.9.${ipN}.1`, ...headers };
  if (body !== undefined) h["Content-Type"] = "application/json";
  return worker.fetch(new Request(BASE + path, { method, headers: h, body: body === undefined ? undefined : JSON.stringify(body) }), env);
}
const auth = (t: string) => ({ Authorization: `Bearer ${t}` });
const j = async (r: Response) => (await r.json()) as any;

function gateway() {
  return {
    fetch: async (url: string, init: RequestInit) => {
      mails.push({ url, token: new Headers(init.headers).get("X-Internal-Token"), body: JSON.parse(String(init.body)) });
      return new Response('{"ok":true}');
    },
  } as unknown as Fetcher;
}

function stubGitHub(email: string, subject = 42) {
  vi.stubGlobal("fetch", async (u: string) => {
    if (u.includes("access_token")) return new Response(JSON.stringify({ access_token: "gho_x", token_type: "bearer", scope: "read:user,user:email" }));
    if (u.endsWith("/user")) return new Response(JSON.stringify({ id: subject, login: "luke", name: "Luke", avatar_url: "https://a/x.png" }));
    if (u.endsWith("/user/emails")) return new Response(JSON.stringify([{ email, primary: true, verified: true }]));
    return new Response("{}", { status: 404 });
  });
}

async function webLogin(ret = SITE + "/account?x=1#old"): Promise<URL> {
  let r = await call("GET", `/v1/auth/github/web?return=${encodeURIComponent(ret)}`);
  expect(r.status).toBe(302);
  const a = new URL(r.headers.get("Location") as string);
  expect(a.searchParams.get("redirect_uri")).toBe(BASE + "/oauth/github/callback");
  r = await call("GET", `/oauth/github/callback?state=${a.searchParams.get("state")}&code=c`);
  expect(r.status).toBe(302);
  return new URL(r.headers.get("Location") as string);
}
const frag = (u: URL, k: string) => new URLSearchParams(u.hash.slice(1)).get(k);

async function signup(email = "luke@example.com", password = "hunter2hunter2") {
  return j(await call("POST", "/v1/auth/signup", { email, password }));
}

beforeEach(() => {
  kv = new FakeKV();
  ipN++;
  mails = [];
  env = {
    KV: kv as unknown as KVNamespace, BASE_URL: BASE, PBKDF2_ITERATIONS: "10000",
    ALLOWED_ORIGINS: "https://blazeinchue.netlify.app,https://blazeinchue.pages.dev",
    GITHUB_CLIENT_ID: "x", GITHUB_CLIENT_SECRET: "y",
  };
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});


const withMail = () => { env.GATEWAY = gateway(); env.INTERNAL_TOKEN = TOKEN; };
const codeOf = (i: number) => /Your Blaze code is (\d{6})/.exec(mails[i]!.body.text)![1]!;

describe("0.9.1 emailed codes", () => {
  it("turns on emailed codes, then sign-in needs the emailed code", async () => {
    withMail();
    const s = await signup("luke@example.com");
    let r = await call("POST", "/v1/auth/2fa/email/send", { purpose: "enable" }, auth(s.token));
    expect(await j(r)).toMatchObject({ ok: true, sent_to: "l**e@example.com" });
    expect((await call("POST", "/v1/auth/2fa/email/enable", { code: "000000" === codeOf(0) ? "111111" : "000000" }, auth(s.token))).status).toBe(400);
    r = await call("POST", "/v1/auth/2fa/email/enable", { code: codeOf(0) }, auth(s.token));
    expect((await j(r)).user).toMatchObject({ two_factor: true, two_factor_methods: { app: false, email: true } });
    r = await call("POST", "/v1/auth/login", { email: "luke@example.com", password: "hunter2hunter2" });
    expect(r.status).toBe(401);
    const m = await j(r);
    expect(m).toMatchObject({ error: "mfa_required", methods: ["email"], email_hint: "l**e@example.com" });
    expect((await call("POST", "/v1/auth/2fa/verify", { mfa_token: m.mfa_token, code: "123456" })).status).toBe(401);
    r = await call("POST", "/v1/auth/2fa/email", { mfa_token: m.mfa_token });
    expect(await j(r)).toMatchObject({ ok: true, sends_left: 2 });
    expect(mails[1]!.body.subject).toMatch(/^Your Blaze sign-in code: \d{6}$/);
    r = await call("POST", "/v1/auth/2fa/verify", { mfa_token: m.mfa_token, code: codeOf(1) });
    expect(r.status).toBe(200);
    expect((await j(r)).token).toBeTruthy();
  });

  it("caps sends at 3 and turns off with an emailed code", async () => {
    withMail();
    const s = await signup("luke@example.com");
    await call("POST", "/v1/auth/2fa/email/send", { purpose: "enable" }, auth(s.token));
    await call("POST", "/v1/auth/2fa/email/enable", { code: codeOf(0) }, auth(s.token));
    const m = await j(await call("POST", "/v1/auth/login", { email: "luke@example.com", password: "hunter2hunter2" }));
    for (let i = 0; i < 3; i++) expect((await call("POST", "/v1/auth/2fa/email", { mfa_token: m.mfa_token })).status).toBe(200);
    expect((await call("POST", "/v1/auth/2fa/email", { mfa_token: m.mfa_token })).status).toBe(429);
    const v = await j(await call("POST", "/v1/auth/2fa/verify", { mfa_token: m.mfa_token, code: codeOf(3) }));
    await call("POST", "/v1/auth/2fa/email/send", { purpose: "disable" }, auth(v.token));
    const r = await call("POST", "/v1/auth/2fa/email/disable", { code: codeOf(4) }, auth(v.token));
    expect((await j(r)).user).toMatchObject({ two_factor: false });
    expect((await call("POST", "/v1/auth/login", { email: "luke@example.com", password: "hunter2hunter2" })).status).toBe(200);
  });

  it("an enable code can't disable, and 503 without the mailer", async () => {
    const s = await signup("luke@example.com");
    expect((await call("POST", "/v1/auth/2fa/email/send", {}, auth(s.token))).status).toBe(503);
    withMail();
    await call("POST", "/v1/auth/2fa/email/send", { purpose: "enable" }, auth(s.token));
    expect((await call("POST", "/v1/auth/2fa/email/disable", { code: codeOf(0) }, auth(s.token))).status).toBe(400);
  });
});
