// 0.9.4: hard rate limits on the abuse-prone endpoints, using Cloudflare's Workers Rate Limiting
// binding (free plan, no KV writes). Counts are per Cloudflare location and approximate; they sit
// in front of the per-isolate softLimit and the per-token try counters, not instead of them.
import type { Env } from "./util";
import { HttpError, clientIP } from "./util";

interface RL { limit(o: { key: string }): Promise<{ success: boolean }> }
type Bind = "RL_IP" | "RL_ACCT" | "RL_POLL";

async function hit(env: Env, b: Bind, key: string, retry: number): Promise<void> {
  const rl = (env as unknown as Record<string, RL | undefined>)[b];
  if (!rl) return; // not bound (tests, local dev)
  const { success } = await rl.limit({ key });
  if (!success) throw new HttpError(429, "rate_limited", { retry_after: retry });
}

async function sha(s: string): Promise<string> {
  const d = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(s));
  return [...new Uint8Array(d).slice(0, 12)].map(x => x.toString(16).padStart(2, "0")).join("");
}

async function field(req: Request, names: string[]): Promise<string> {
  try {
    const t = await req.clone().text();
    if (t.length > 16384) return "";
    const j = JSON.parse(t) as Record<string, unknown>;
    for (const n of names) if (typeof j[n] === "string" && j[n]) return String(j[n]).trim().toLowerCase();
  } catch { /* not JSON */ }
  return "";
}

const GROUP: Record<string, string> = {
  "/v1/auth/login": "login", "/v1/auth/signup": "signup",
  "/v1/auth/2fa/verify": "mfa", "/v1/auth/2fa/email": "mfa", "/v1/auth/2fa/email/send": "mfasend",
  "/v1/auth/password/forgot": "forgot", "/v1/auth/password/reset": "reset", "/v1/auth/password/set": "pwset",
  "/v1/auth/device/start": "devstart", "/v1/auth/device/approve": "devapprove", "/v1/auth/device/deny": "devdeny",
  "/v1/auth/device/info": "devinfo", "/v1/me/avatar": "avatar", "/v1/me/avatar/delete": "avatar",
  "/v1/auth/web/redeem": "redeem", "/v1/auth/redeem": "redeem", "/oauth/redeem": "redeem",
};

/** Throws 429 when the caller is over a hard limit for this endpoint. */
export async function guard(req: Request, env: Env, path: string): Promise<void> {
  if (req.method === "OPTIONS") return;
  const ip = clientIP(req);
  if (path === "/v1/auth/device/poll") return hit(env, "RL_POLL", "poll:" + ip, 60);
  const g = GROUP[path];
  if (!g) return;
  await hit(env, "RL_IP", g + ":" + ip, 60);
  // Per-account limits: spread-out attackers still get only a few tries a minute per target.
  let acct = "";
  if (g === "login" || g === "forgot") acct = await field(req, ["email"]);
  else if (g === "mfa") acct = await field(req, ["mfa_token", "token"]);
  else if (g === "reset") acct = await field(req, ["token"]);
  if (!acct && (g === "mfasend" || g === "avatar" || g === "pwset" || g === "devapprove")) {
    const a = req.headers.get("Authorization") || "";
    acct = a ? a : await field(req, ["mfa_token", "token"]);
  }
  if (acct) await hit(env, "RL_ACCT", g + ":" + (await sha(acct)), 60);
}
