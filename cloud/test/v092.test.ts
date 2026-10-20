// blaze-auth 0.9.2: device-code sign-in.
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



describe("0.9.2 device sign-in", () => {
  it("start -> pending -> approve -> one session, single use", async () => {
    const s = await signup("luke@example.com");
    let r = await call("POST", "/v1/auth/device/start", { client: "Blaze CLI" });
    const d = await j(r);
    expect(d.device_code.length).toBeGreaterThanOrEqual(32);
    expect(d.user_code).toMatch(/^[A-HJ-NP-Z2-9]{4}-[A-HJ-NP-Z2-9]{4}$/);
    expect(d).toMatchObject({ verification_uri: "https://blazeinchue.pages.dev/device", interval: 5, expires_in: 600 });
    expect(d.verification_uri_complete).toBe(d.verification_uri + "?code=" + d.user_code);
    r = await call("POST", "/v1/auth/device/poll", { device_code: d.device_code });
    expect(r.status).toBe(400); expect((await j(r)).error).toBe("authorization_pending");
    r = await call("POST", "/v1/auth/device/poll", { device_code: d.device_code });
    expect((await j(r)).error).toBe("slow_down");
    expect(await j(await call("GET", "/v1/auth/device/info?code=" + d.user_code.toLowerCase().replace("-", "")))).toMatchObject({ client: "Blaze CLI" });
    expect((await call("POST", "/v1/auth/device/approve", { user_code: d.user_code })).status).toBe(401);
    expect(await j(await call("POST", "/v1/auth/device/approve", { user_code: d.user_code }, auth(s.token)))).toMatchObject({ ok: true });
    expect((await call("POST", "/v1/auth/device/approve", { user_code: d.user_code }, auth(s.token))).status).toBe(400); // single use
    vi.useFakeTimers(); vi.setSystemTime(Date.now() + 6000);
    r = await call("POST", "/v1/auth/device/poll", { device_code: d.device_code, client: "Blaze CLI" });
    expect(r.status).toBe(200);
    const ok = await j(r);
    expect(ok.user.email).toBe("luke@example.com");
    expect((await call("GET", "/v1/me", undefined, auth(ok.token))).status).toBe(200);
    vi.setSystemTime(Date.now() + 6000);
    r = await call("POST", "/v1/auth/device/poll", { device_code: d.device_code });
    expect((await j(r)).error).toBe("expired_token");
  });

  it("deny -> access_denied; expiry -> expired_token", async () => {
    let d = await j(await call("POST", "/v1/auth/device/start", {}));
    expect(await j(await call("POST", "/v1/auth/device/deny", { user_code: d.user_code }))).toEqual({ ok: true });
    expect((await j(await call("POST", "/v1/auth/device/poll", { device_code: d.device_code }))).error).toBe("access_denied");
    d = await j(await call("POST", "/v1/auth/device/start", {}));
    vi.useFakeTimers(); vi.setSystemTime(Date.now() + 601000);
    expect((await j(await call("POST", "/v1/auth/device/poll", { device_code: d.device_code }))).error).toBe("expired_token");
    expect((await call("GET", "/v1/auth/device/info?code=" + d.user_code)).status).toBe(400);
  });

  it("start is rate limited per IP and /device redirects to the site", async () => {
    for (let i = 0; i < 10; i++) expect((await call("POST", "/v1/auth/device/start", {})).status).toBe(200);
    expect((await call("POST", "/v1/auth/device/start", {})).status).toBe(429);
    const r = await call("GET", "/device?code=ABCD-EFGH");
    expect(r.headers.get("Location")).toBe("https://blazeinchue.pages.dev/device?code=ABCD-EFGH");
  });
});
