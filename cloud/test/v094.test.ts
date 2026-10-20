// blaze-auth 0.9.4: hard rate limits.
import { describe, it, expect } from "vitest";
import worker from "../src/index";
import type { Env } from "../src/util";
class KV { m = new Map<string, any>(); async get(k: string, t?: string) { const v = this.m.get(k); return v == null ? null : t === "json" ? JSON.parse(v) : v; } async put(k: string, v: any) { this.m.set(k, v); } async delete(k: string) { this.m.delete(k); } }
function rl(limit: number) { const c = new Map<string, number>(); return { seen: c, async limit({ key }: { key: string }) { const n = (c.get(key) || 0) + 1; c.set(key, n); return { success: n <= limit }; } }; }
describe("0.9.4 rate limits", () => {
  it("per-account limit on login across IPs, per-IP limit, poll limit", async () => {
    const acct = rl(6), ip = rl(20), poll = rl(30);
    const env = { KV: new KV(), BASE_URL: "https://a", PBKDF2_ITERATIONS: "10000", ALLOWED_ORIGINS: "", RL_ACCT: acct, RL_IP: ip, RL_POLL: poll } as unknown as Env;
    const login = (n: number, email = "x@example.com") => worker.fetch(new Request("https://a/v1/auth/login", { method: "POST", headers: { "Content-Type": "application/json", "CF-Connecting-IP": "10.0.0." + n }, body: JSON.stringify({ email, password: "wrongwrongwrong" }) }), env);
    const st: number[] = [];
    for (let i = 0; i < 8; i++) st.push((await login(i)).status);
    expect(st.slice(0, 6).every(s => s === 401)).toBe(true);
    expect(st.slice(6)).toEqual([429, 429]);
    const r = await login(99, "other@example.com"); expect(r.status).toBe(401); // other accounts unaffected
    let last = 0;
    for (let i = 0; i < 31; i++) last = (await worker.fetch(new Request("https://a/v1/auth/device/poll", { method: "POST", headers: { "Content-Type": "application/json", "CF-Connecting-IP": "10.1.1.1" }, body: JSON.stringify({ device_code: "x".repeat(40) }) }), env)).status;
    expect(last).toBe(429);
    expect((await worker.fetch(new Request("https://a/health"), env)).status).toBe(200);
    expect([...acct.seen.keys()].some(k => k.includes("example.com"))).toBe(false); // keys are hashed
  });
});
