// blaze-auth 0.9.5: blaze://auth/cb app redirect for login/connect.
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import worker from "../src/index";
import type { Env } from "../src/util";
class KV { m = new Map<string, any>(); async get(k: string, t?: string) { const v = this.m.get(k); return v == null ? null : t === "json" ? JSON.parse(v) : v; } async put(k: string, v: any) { this.m.set(k, v); } async delete(k: string) { this.m.delete(k); } }
const BASE = "https://blaze-auth.blaze-app.workers.dev";
let env: Env; let n = 0;
const call = (p: string, init: RequestInit = {}) => worker.fetch(new Request(BASE + p, { ...init, headers: { "CF-Connecting-IP": "10.95." + n + ".1", ...(init.headers || {}) } }), env);
const STATE = "s".repeat(24), CC = "c".repeat(43);
beforeEach(() => { n++; env = { KV: new KV() as unknown as KVNamespace, BASE_URL: BASE, PBKDF2_ITERATIONS: "10000", ALLOWED_ORIGINS: "", GITHUB_CLIENT_ID: "x", GITHUB_CLIENT_SECRET: "y" } as Env;
  vi.stubGlobal("fetch", async (u: string) => {
    if (u.includes("access_token")) return new Response(JSON.stringify({ access_token: "gho_x", token_type: "bearer", scope: "read:user,user:email" }));
    if (u.endsWith("/user")) return new Response(JSON.stringify({ id: 7, login: "luke", name: "Luke", avatar_url: "" }));
    if (u.endsWith("/user/emails")) return new Response(JSON.stringify([{ email: "l@example.com", primary: true, verified: true }]));
    return new Response("{}", { status: 404 });
  });
});
afterEach(() => vi.unstubAllGlobals());
const start = (redirect: string, path = "/v1/auth/github/start") => call(`${path}?state=${STATE}&code_challenge=${CC}&code_challenge_method=S256&redirect=${encodeURIComponent(redirect)}`);
describe("0.9.5 app redirect", () => {
  it("accepts exactly blaze://auth/cb and hands off with an HTML page", async () => {
    for (const bad of ["blaze://auth/cb/x", "blaze://auth/cb?x=1", "blaze://other", "blazex://auth/cb", "https://evil.example/cb", "BLAZE://auth/cb"]) expect((await start(bad)).status).toBe(400);
    const r = await start("blaze://auth/cb"); expect(r.status).toBe(302);
    const st = new URL(r.headers.get("Location")!).searchParams.get("state")!;
    const cb = await call(`/oauth/github/callback?state=${st}&code=abc`);
    expect(cb.status).toBe(200);
    expect(cb.headers.get("Content-Type")).toContain("text/html");
    const html = await cb.text();
    const m = /href="(blaze:\/\/auth\/cb\?[^"]+)"/.exec(html)!; expect(m).toBeTruthy();
    const u = new URL(m[1]!.replace(/&amp;/g, "&"));
    expect(u.searchParams.get("state")).toBe(STATE);
    const code = u.searchParams.get("code")!; expect(code.length).toBeGreaterThan(20);
    expect(html).toContain("Open Blaze"); expect(html).toContain("location.href=");
    expect(cb.headers.get("Content-Security-Policy")).toMatch(/script-src 'nonce-/);
    // error path also hands off
    const r2 = await start("blaze://auth/cb"); const st2 = new URL(r2.headers.get("Location")!).searchParams.get("state")!;
    const cb2 = await (await call(`/oauth/github/callback?state=${st2}&error=access_denied`)).text();
    expect(cb2).toContain("error=access_denied");
    // loopback still a plain 302
    const r3 = await start("http://127.0.0.1:53682/cb"); const st3 = new URL(r3.headers.get("Location")!).searchParams.get("state")!;
    const cb3 = await call(`/oauth/github/callback?state=${st3}&code=abc`); expect(cb3.status).toBe(302); expect(cb3.headers.get("Location")!).toMatch(/^http:\/\/127\.0\.0\.1:53682\/cb\?/);
    const prov = await (await call("/v1/providers")).json() as any;
    expect(prov.features.app_redirect).toBe(true);
  });
});
