// blaze-auth 0.9.3: profile pictures.
import { describe, it, expect, beforeEach } from "vitest";
import worker from "../src/index";
import type { Env } from "../src/util";

class FakeKV {
  m = new Map<string, { v: any; meta: any }>();
  async get(k: string, type?: string): Promise<any> { const e = this.m.get(k); if (!e) return null; return type === "json" ? JSON.parse(e.v) : e.v; }
  async getWithMetadata(k: string): Promise<any> { const e = this.m.get(k); return e ? { value: e.v instanceof Uint8Array ? e.v.buffer : e.v, metadata: e.meta } : { value: null, metadata: null }; }
  async put(k: string, v: any, o?: { metadata?: any }) { this.m.set(k, { v, meta: o && o.metadata }); }
  async delete(k: string) { this.m.delete(k); }
}
const BASE = "https://blaze-auth.blaze-app.workers.dev";
let env: Env; let n = 0;
const PNG = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3, 4]);
function call(method: string, path: string, body?: BodyInit, headers: Record<string, string> = {}) {
  return worker.fetch(new Request(BASE + path, { method, body, headers: { "CF-Connecting-IP": `10.93.${n}.1`, ...headers } }), env);
}
beforeEach(() => { n++; env = { KV: new FakeKV() as unknown as KVNamespace, BASE_URL: BASE, PBKDF2_ITERATIONS: "10000", ALLOWED_ORIGINS: "https://blazeinchue.pages.dev" } as Env; });

describe("0.9.3 avatars", () => {
  it("upload, serve, show on /v1/me, reset to letter", async () => {
    const s = await (await call("POST", "/v1/auth/signup", JSON.stringify({ email: "a@example.com", password: "hunter2hunter2" }), { "Content-Type": "application/json" })).json() as any;
    const A = { Authorization: "Bearer " + s.token };
    expect(s.user.avatar_url).toBe("");
    expect((await call("POST", "/v1/me/avatar", PNG, { ...A, "Content-Type": "image/png" })).status).toBe(200);
    const me = await (await call("GET", "/v1/me", undefined, A)).json() as any;
    expect(me.user.avatar_url).toMatch(new RegExp("^" + BASE + "/v1/avatar/" + s.user.id + "\\?v=\\d+$"));
    const img = await call("GET", "/v1/avatar/" + s.user.id);
    expect(img.status).toBe(200); expect(img.headers.get("Content-Type")).toBe("image/png");
    expect(new Uint8Array(await img.arrayBuffer())).toEqual(PNG);
    expect((await call("POST", "/v1/me/avatar", new Uint8Array([60, 115, 118, 103, 62, 1, 2, 3, 4, 5, 6, 7, 8]), { ...A, "Content-Type": "image/png" })).status).toBe(415);
    expect((await call("POST", "/v1/me/avatar", new Uint8Array(300 * 1024).fill(0xff), { ...A, "Content-Type": "image/jpeg" })).status).toBe(413);
    expect((await call("POST", "/v1/me/avatar", PNG)).status).toBe(401);
    const d = await (await call("POST", "/v1/me/avatar/delete", undefined, A)).json() as any;
    expect(d.avatar_url).toBe("");
    expect((await call("GET", "/v1/avatar/" + s.user.id)).status).toBe(404);
  });
});
