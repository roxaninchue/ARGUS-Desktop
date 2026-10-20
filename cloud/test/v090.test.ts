// blaze-auth 0.9.0: web sign-in, password reset / set, TOTP two-factor.
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

describe("0.9.0 discovery", () => {
  it("reports 0.9.0, web_login, two_factor and password_reset only with the mailer", async () => {
    expect(await j(await call("GET", "/health"))).toEqual({ ok: true, service: "blaze-auth", version: "0.9.0" });
    let d = await j(await call("GET", "/v1/providers"));
    expect(d).toMatchObject({ version: "0.9.0", web_login: true, two_factor: true, password_reset: false });
    env.GATEWAY = gateway();
    expect((await j(await call("GET", "/v1/providers"))).password_reset).toBe(false); // token still missing
    env.INTERNAL_TOKEN = TOKEN;
    d = await j(await call("GET", "/v1/providers"));
    expect(d.password_reset).toBe(true);
  });
  it("publicUser carries has_password and two_factor", async () => {
    const s = await signup();
    expect(s.user).toMatchObject({ has_password: true, two_factor: false });
  });
  it("keeps CORS methods as they were", async () => {
    const r = await call("OPTIONS", "/v1/auth/web/redeem", undefined, { Origin: SITE });
    expect(r.status).toBe(204);
    expect(r.headers.get("Access-Control-Allow-Methods")).toBe("GET, POST, OPTIONS");
  });
});

describe("web sign-in: GET /v1/auth/{google|github}/web + POST /v1/auth/web/redeem", () => {
  it("refuses a missing, non-https or foreign return URL and connector-only providers", async () => {
    for (const ret of ["", "http://blazeinchue.pages.dev/", "https://evil.example/", "https://blazeinchue.pages.dev.evil.example/", "javascript:alert(1)"])
      expect((await call("GET", `/v1/auth/github/web?return=${encodeURIComponent(ret)}`)).status).toBe(400);
    expect((await call("GET", `/v1/auth/spotify/web?return=${encodeURIComponent(SITE)}`)).status).toBe(404);
    delete env.GITHUB_CLIENT_ID;
    expect((await call("GET", `/v1/auth/github/web?return=${encodeURIComponent(SITE)}`)).status).toBe(503);
  });
  it("Google web start reuses the callback and asks to pick an account", async () => {
    env.GOOGLE_CLIENT_ID = "g";
    env.GOOGLE_CLIENT_SECRET = "s";
    const r = await call("GET", `/v1/auth/google/web?return=${encodeURIComponent(SITE + "/")}`);
    const a = new URL(r.headers.get("Location") as string);
    expect(a.searchParams.get("redirect_uri")).toBe(BASE + "/oauth/google/callback");
    expect(a.searchParams.get("prompt")).toBe("select_account");
    expect(a.searchParams.get("access_type")).toBeNull();
    const st: any = await kv.get(`oauthstate:${a.searchParams.get("state")}`, "json");
    expect(st).toMatchObject({ flow: "web", redirect: SITE + "/" });
  });
  it("redirects back with a one-time code that redeems once for a session, on the same account as app sign-in", async () => {
    stubGitHub("luke@example.com");
    const pre = await signup("luke@example.com"); // unverified password account: the verified GitHub email takes it over
    const back = await webLogin();
    expect(back.origin + back.pathname + back.search).toBe(SITE + "/account?x=1");
    const code = frag(back, "bz_oauth") as string;
    expect(code).toMatch(/^[A-Za-z0-9_-]{40,}$/);
    let r = await call("POST", "/v1/auth/web/redeem", { code, client: "Blaze web" }, { Origin: SITE });
    expect(r.status).toBe(200);
    expect(r.headers.get("Access-Control-Allow-Origin")).toBe(SITE);
    const s = await j(r);
    expect(s.token).toMatch(/^bz_/);
    expect(s.user).toMatchObject({ id: pre.user.id, providers: ["github"], email_verified: true });
    expect((await call("POST", "/v1/auth/web/redeem", { code }, { Origin: SITE })).status).toBe(400); // single use
    const again = await webLogin();
    const s2 = await j(await call("POST", "/v1/auth/web/redeem", { code: frag(again, "bz_oauth") }));
    expect(s2.user.id).toBe(pre.user.id);
  });
  it("refuses a code redeemed from another origin, and expires codes after 120 s", async () => {
    stubGitHub("luke@example.com");
    let back = await webLogin();
    expect((await call("POST", "/v1/auth/web/redeem", { code: frag(back, "bz_oauth") }, { Origin: "https://blazeinchue.netlify.app" })).status).toBe(400);
    vi.useFakeTimers({ toFake: ["Date"] });
    back = await webLogin();
    vi.setSystemTime(Date.now() + 121_000);
    expect((await call("POST", "/v1/auth/web/redeem", { code: frag(back, "bz_oauth") })).status).toBe(400);
    expect((await call("POST", "/v1/auth/web/redeem", { code: "short" })).status).toBe(400);
  });
  it("hands provider errors back as #bz_oauth_error", async () => {
    const r = await call("GET", `/v1/auth/github/web?return=${encodeURIComponent(SITE)}`);
    const a = new URL(r.headers.get("Location") as string);
    const back = new URL((await call("GET", `/oauth/github/callback?state=${a.searchParams.get("state")}&error=access_denied`)).headers.get("Location") as string);
    expect(back.origin).toBe(SITE);
    expect(frag(back, "bz_oauth_error")).toBe("access_denied");
    expect(frag(back, "bz_oauth")).toBeNull();
  });
});

describe("password: forgot / reset / set", () => {
  beforeEach(() => {
    env.GATEWAY = gateway();
    env.INTERNAL_TOKEN = TOKEN;
  });
  it("forgot answers 200 either way and mails a link only to a real account", async () => {
    let r = await call("POST", "/v1/auth/password/forgot", { email: "nobody@example.com" });
    expect(await j(r)).toEqual({ ok: true });
    expect(mails.length).toBe(0);
    await signup();
    r = await call("POST", "/v1/auth/password/forgot", { email: "Luke@Example.com", return: "https://blazeinchue.netlify.app/plans" });
    expect(await j(r)).toEqual({ ok: true });
    expect(mails.length).toBe(1);
    expect(mails[0]!.url).toBe("https://gateway/internal/mail");
    expect(mails[0]!.token).toBe(TOKEN);
    expect(mails[0]!.body).toMatchObject({ to: "luke@example.com", subject: "Set a new password for your Blaze account" });
    expect(mails[0]!.body.text).toContain("Set a new password for your Blaze account");
    expect(mails[0]!.body.text).toContain("If you signed up with Google or GitHub, this lets you add a password");
    expect(mails[0]!.body.text).toMatch(/https:\/\/blazeinchue\.netlify\.app\/\?reset=[A-Za-z0-9_-]{40,}#reset/);
    await call("POST", "/v1/auth/password/forgot", { email: "luke@example.com", return: "https://evil.example/" });
    expect(mails[1]!.body.text).toContain(SITE + "/?reset="); // a foreign return falls back to the site
    expect(kv.keys("reset:").length).toBe(2);
  });
  it("forgot is rate limited per email and per IP, and 503 without the mailer", async () => {
    await signup();
    for (let i = 0; i < 6; i++) expect(await j(await call("POST", "/v1/auth/password/forgot", { email: "luke@example.com" }))).toEqual({ ok: true });
    expect(mails.length).toBe(5); // per address: the 6th answers the same but sends nothing
    for (let i = 0; i < 4; i++) await call("POST", "/v1/auth/password/forgot", { email: `x${i}@example.com` });
    expect((await call("POST", "/v1/auth/password/forgot", { email: "y@example.com" })).status).toBe(429); // 10 per IP an hour
    ipN++;
    delete env.INTERNAL_TOKEN;
    expect((await call("POST", "/v1/auth/password/forgot", { email: "luke@example.com" })).status).toBe(503);
    expect((await call("POST", "/v1/auth/password/forgot", { email: "bad" })).status).toBe(400);
  });
  it("reset sets the password, verifies the email, ends every session and is single use", async () => {
    const s = await signup();
    await call("POST", "/v1/auth/password/forgot", { email: "luke@example.com" });
    const tok = /reset=([A-Za-z0-9_-]+)#/.exec(mails[0]!.body.text)![1]!;
    expect((await call("POST", "/v1/auth/password/reset", { token: tok, password: "short" })).status).toBe(400);
    let r = await call("POST", "/v1/auth/password/reset", { token: tok, password: "brand-new-pass" });
    expect(await j(r)).toEqual({ ok: true });
    expect((await call("GET", "/v1/me", undefined, auth(s.token))).status).toBe(401);
    expect((await call("POST", "/v1/auth/login", { email: "luke@example.com", password: "hunter2hunter2" })).status).toBe(401);
    r = await call("POST", "/v1/auth/login", { email: "luke@example.com", password: "brand-new-pass" });
    expect(r.status).toBe(200);
    expect((await j(r)).user).toMatchObject({ email_verified: true, has_password: true });
    expect((await call("POST", "/v1/auth/password/reset", { token: tok, password: "another-pass-1" })).status).toBe(400);
    expect((await call("POST", "/v1/auth/password/reset", { token: "x".repeat(43), password: "another-pass-1" })).status).toBe(400);
  });
  it("reset links expire after an hour", async () => {
    await signup();
    vi.useFakeTimers({ toFake: ["Date"] });
    await call("POST", "/v1/auth/password/forgot", { email: "luke@example.com" });
    const tok = /reset=([A-Za-z0-9_-]+)#/.exec(mails[0]!.body.text)![1]!;
    vi.setSystemTime(Date.now() + 3601_000);
    expect((await call("POST", "/v1/auth/password/reset", { token: tok, password: "brand-new-pass" })).status).toBe(400);
  });
  it("a GitHub-created account can add a password through reset", async () => {
    stubGitHub("gh@example.com", 7);
    const back = await webLogin();
    const s = await j(await call("POST", "/v1/auth/web/redeem", { code: frag(back, "bz_oauth") }));
    expect(s.user.has_password).toBe(false);
    await call("POST", "/v1/auth/password/forgot", { email: "gh@example.com" });
    const tok = /reset=([A-Za-z0-9_-]+)#/.exec(mails[0]!.body.text)![1]!;
    await call("POST", "/v1/auth/password/reset", { token: tok, password: "added-password" });
    const l = await j(await call("POST", "/v1/auth/login", { email: "gh@example.com", password: "added-password" }));
    expect(l.user).toMatchObject({ id: s.user.id, has_password: true, providers: ["github"] });
  });
  it("set: no current password needed without one; required and checked with one; other sessions end", async () => {
    stubGitHub("gh@example.com", 8);
    const s = await j(await call("POST", "/v1/auth/web/redeem", { code: frag(await webLogin(), "bz_oauth") }));
    expect((await call("POST", "/v1/auth/password/set", { password: "first-password" })).status).toBe(401);
    let r = await call("POST", "/v1/auth/password/set", { password: "first-password" }, auth(s.token));
    expect((await j(r)).user.has_password).toBe(true);
    const other = await j(await call("POST", "/v1/auth/login", { email: "gh@example.com", password: "first-password" }));
    expect((await call("POST", "/v1/auth/password/set", { password: "second-password" }, auth(s.token))).status).toBe(400);
    expect((await call("POST", "/v1/auth/password/set", { password: "second-password", current_password: "wrong-one!" }, auth(s.token))).status).toBe(401);
    r = await call("POST", "/v1/auth/password/set", { password: "second-password", current_password: "first-password" }, auth(s.token));
    expect(r.status).toBe(200);
    expect((await call("GET", "/v1/me", undefined, auth(s.token))).status).toBe(200);
    expect((await call("GET", "/v1/me", undefined, auth(other.token))).status).toBe(401);
    expect((await call("POST", "/v1/auth/login", { email: "gh@example.com", password: "second-password" })).status).toBe(200);
  });
});

describe("two-factor (TOTP)", () => {
  it("matches the RFC 6238 SHA-1 vector and round-trips base32", async () => {
    const secret = base32Encode(new TextEncoder().encode("12345678901234567890"));
    expect(secret).toBe("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ");
    expect(await totpAt(secret, 1)).toBe("287082"); // T=59
    expect(await totpAt(secret, Math.floor(1111111109 / 30))).toBe("081804");
    expect(new TextDecoder().decode(base32Decode(secret))).toBe("12345678901234567890");
  });

  async function enabled() {
    vi.useFakeTimers({ toFake: ["Date"] });
    const s = await signup();
    expect((await call("POST", "/v1/auth/2fa/setup")).status).toBe(401);
    const st = await j(await call("POST", "/v1/auth/2fa/setup", {}, auth(s.token)));
    expect(st.secret).toMatch(/^[A-Z2-7]{32}$/);
    expect(st.otpauth_uri).toBe(`otpauth://totp/Blaze:luke%40example.com?secret=${st.secret}&issuer=Blaze`);
    expect((await call("POST", "/v1/auth/2fa/enable", { code: "000000" }, auth(s.token))).status).toBe(400);
    const en = await j(await call("POST", "/v1/auth/2fa/enable", { code: await totpAt(st.secret, timeStep()) }, auth(s.token)));
    expect(en.recovery_codes).toHaveLength(10);
    expect(new Set(en.recovery_codes).size).toBe(10);
    for (const c of en.recovery_codes) expect(c).toMatch(/^[a-z2-7]{5}-[a-z2-7]{5}$/);
    expect(JSON.stringify([...kv.m.values()])).not.toContain(en.recovery_codes[0]); // stored hashed
    expect((await call("POST", "/v1/auth/2fa/setup", {}, auth(s.token))).status).toBe(409);
    const me = await j(await call("GET", "/v1/me", undefined, auth(s.token)));
    expect(me.user.two_factor).toBe(true);
    return { s, secret: st.secret as string, codes: en.recovery_codes as string[] };
  }
  const tick = () => vi.setSystemTime(Date.now() + 30_000);
  async function mfaLogin() {
    const r = await call("POST", "/v1/auth/login", { email: "luke@example.com", password: "hunter2hunter2" });
    expect(r.status).toBe(401);
    const d = await j(r);
    expect(d.error).toBe("mfa_required");
    expect(d.mfa_token).toMatch(/^mfa_/);
    expect(d.token).toBeUndefined();
    return d.mfa_token as string;
  }

  it("password login needs the second factor; verify issues a session; codes can't be replayed", async () => {
    const { secret } = await enabled();
    tick();
    const m = await mfaLogin();
    const code = await totpAt(secret, timeStep());
    const v = await j(await call("POST", "/v1/auth/2fa/verify", { mfa_token: m, code }));
    expect(v.token).toMatch(/^bz_/);
    expect(v.user).toMatchObject({ email: "luke@example.com", two_factor: true });
    expect((await call("POST", "/v1/auth/2fa/verify", { mfa_token: m, code })).status).toBe(401); // token single use
    const m2 = await mfaLogin();
    const r = await call("POST", "/v1/auth/2fa/verify", { mfa_token: m2, code }); // same step again
    expect(r.status).toBe(401);
    expect((await j(r)).error).toBe("invalid_code");
  });
  it("an mfa token allows 5 tries and lasts 5 minutes", async () => {
    const { secret } = await enabled();
    tick();
    const m = await mfaLogin();
    for (let i = 1; i <= 4; i++) expect(await j(await call("POST", "/v1/auth/2fa/verify", { mfa_token: m, code: "000000" }))).toMatchObject({ error: "invalid_code", tries_left: 5 - i });
    expect((await j(await call("POST", "/v1/auth/2fa/verify", { mfa_token: m, code: "000000" }))).error).toBe("invalid_mfa_token");
    expect((await call("POST", "/v1/auth/2fa/verify", { mfa_token: m, code: await totpAt(secret, timeStep()) })).status).toBe(401);
    const m2 = await mfaLogin();
    vi.setSystemTime(Date.now() + 301_000);
    expect((await call("POST", "/v1/auth/2fa/verify", { mfa_token: m2, code: await totpAt(secret, timeStep()) })).status).toBe(401);
  });
  it("a recovery code signs in once", async () => {
    const { codes } = await enabled();
    const v = await j(await call("POST", "/v1/auth/2fa/verify", { mfa_token: await mfaLogin(), code: codes[0]!.toUpperCase() }));
    expect(v.token).toMatch(/^bz_/);
    expect((await call("POST", "/v1/auth/2fa/verify", { mfa_token: await mfaLogin(), code: codes[0] })).status).toBe(401);
    expect((await call("POST", "/v1/auth/2fa/verify", { mfa_token: await mfaLogin(), code: codes[1]!.replace("-", "") })).status).toBe(200);
  });
  it("web redeem and app OAuth redeem also stop at mfa_required", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    stubGitHub("gh@example.com", 9);
    const s = await j(await call("POST", "/v1/auth/web/redeem", { code: frag(await webLogin(), "bz_oauth") }));
    const st = await j(await call("POST", "/v1/auth/2fa/setup", {}, auth(s.token)));
    await call("POST", "/v1/auth/2fa/enable", { code: await totpAt(st.secret, timeStep()) }, auth(s.token));
    let r = await call("POST", "/v1/auth/web/redeem", { code: frag(await webLogin(), "bz_oauth") }, { Origin: SITE });
    expect(r.status).toBe(401);
    const d = await j(r);
    expect(d.error).toBe("mfa_required");
    tick();
    const v = await j(await call("POST", "/v1/auth/2fa/verify", { mfa_token: d.mfa_token, code: await totpAt(st.secret, timeStep()) }));
    expect(v.user.id).toBe(s.user.id);

    const vb = new Uint8Array(32);
    crypto.getRandomValues(vb);
    const verifier = b64url(vb);
    const challenge = b64url(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier)));
    r = await call("GET", `/v1/auth/github/start?state=${"g".repeat(20)}&code_challenge=${challenge}&redirect=${encodeURIComponent("http://127.0.0.1:7000/cb")}`);
    const a = new URL(r.headers.get("Location") as string);
    const loop = new URL((await call("GET", `/oauth/github/callback?state=${a.searchParams.get("state")}&code=c`)).headers.get("Location") as string);
    r = await call("POST", "/oauth/redeem", { code: loop.searchParams.get("code"), code_verifier: verifier });
    expect(r.status).toBe(401);
    expect((await j(r)).error).toBe("mfa_required");
  });
  it("a verified provider email taking over an unverified account also drops that account's 2FA", async () => {
    await enabled();
    stubGitHub("luke@example.com");
    const r = await call("POST", "/v1/auth/web/redeem", { code: frag(await webLogin(), "bz_oauth") });
    expect(r.status).toBe(200);
    expect((await j(r)).user).toMatchObject({ has_password: false, two_factor: false });
  });
  it("disable takes a TOTP or a recovery code, then sign-in is one step again", async () => {
    const { s, secret, codes } = await enabled();
    expect((await call("POST", "/v1/auth/2fa/disable", { code: "123456" }, auth(s.token))).status).toBe(400);
    let r = await call("POST", "/v1/auth/2fa/disable", { code: codes[3] }, auth(s.token));
    expect((await j(r)).user.two_factor).toBe(false);
    expect((await call("POST", "/v1/auth/2fa/disable", { code: codes[4] }, auth(s.token))).status).toBe(400); // not enabled
    expect((await call("POST", "/v1/auth/login", { email: "luke@example.com", password: "hunter2hunter2" })).status).toBe(200);
    // and again with TOTP
    const st = await j(await call("POST", "/v1/auth/2fa/setup", {}, auth(s.token)));
    tick();
    await call("POST", "/v1/auth/2fa/enable", { code: await totpAt(st.secret, timeStep()) }, auth(s.token));
    tick();
    r = await call("POST", "/v1/auth/2fa/disable", { code: await totpAt(st.secret, timeStep()) }, auth(s.token));
    expect(r.status).toBe(200);
    expect(secret).not.toBe(st.secret);
  });
  it("a reset on an unverified account drops its 2FA; on a verified one 2FA stays", async () => {
    await enabled();
    env.GATEWAY = gateway();
    env.INTERNAL_TOKEN = TOKEN;
    const resetOnce = async (pw: string) => {
      await call("POST", "/v1/auth/password/forgot", { email: "luke@example.com" });
      const tok = /reset=([A-Za-z0-9_-]+)#/.exec(mails[mails.length - 1]!.body.text)![1]!;
      expect((await call("POST", "/v1/auth/password/reset", { token: tok, password: pw })).status).toBe(200);
    };
    await resetOnce("owner-password-1");
    let l = await j(await call("POST", "/v1/auth/login", { email: "luke@example.com", password: "owner-password-1" }));
    expect(l.user).toMatchObject({ email_verified: true, two_factor: false });
    const st = await j(await call("POST", "/v1/auth/2fa/setup", {}, auth(l.token)));
    tick();
    await call("POST", "/v1/auth/2fa/enable", { code: await totpAt(st.secret, timeStep()) }, auth(l.token));
    await resetOnce("owner-password-2");
    const r = await call("POST", "/v1/auth/login", { email: "luke@example.com", password: "owner-password-2" });
    expect((await j(r)).error).toBe("mfa_required");
  });
  it("enable needs setup first", async () => {
    const s = await signup();
    expect((await call("POST", "/v1/auth/2fa/enable", { code: "123456" }, auth(s.token))).status).toBe(400);
    expect((await call("POST", "/v1/auth/2fa/verify", { mfa_token: "nope", code: "123456" })).status).toBe(401);
  });
});
