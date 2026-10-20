// Device-code sign-in (0.9.2), RFC 8628 shaped, for terminals that can't receive a loopback redirect.
//   POST /v1/auth/device/start   {client} -> {device_code, user_code "ABCD-EFGH", verification_uri, verification_uri_complete, interval, expires_in}
//   GET  /v1/auth/device/info?code=ABCD-EFGH -> {client, expires_in}     (the /device page shows who is asking)
//   POST /v1/auth/device/approve Bearer {user_code} -> {ok, client}       (a signed-in browser approves; 2FA already happened there)
//   POST /v1/auth/device/deny    {user_code} -> {ok}
//   POST /v1/auth/device/poll    {device_code, client?} -> 400 authorization_pending | slow_down | expired_token | access_denied, or {token, user}
// KV: device:<sha256(device_code)> {user_code, client, status, uid?, exp}, devuser:<user_code> -> the device key. TTL 600.
// user_code is single use: approve/deny remove devuser:<code>. The approved record is deleted on the first good poll.

import { Env, HttpError, json, readJSON, clientIP, randomToken, sha256, softLimit, cleanName } from "./util";
import { getUser, requireUser, createSession, publicUser } from "./accounts";

export const DEVICE_TTL = 600;
export const DEVICE_INTERVAL = 5;
export const DEVICE_SITE = "https://blazeinchue.pages.dev";
const ALPHA = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"; // no 0 O 1 I
const now = () => Math.floor(Date.now() / 1000);

interface DeviceRecord { user_code: string; client: string; status: "pending" | "approved" | "denied"; uid?: string; exp: number }

const lastPoll = new Map<string, number>(); // per isolate; good enough for slow_down

export function newUserCode(): string {
  const b = new Uint8Array(8);
  crypto.getRandomValues(b);
  let s = "";
  for (const x of b) s += ALPHA[x % 32]; // 256 % 32 == 0: no bias
  return s.slice(0, 4) + "-" + s.slice(4);
}
export function normUserCode(v: unknown): string | null {
  if (typeof v !== "string") return null;
  const s = v.toUpperCase().replace(/[^A-Z0-9]/g, "");
  if (s.length !== 8 || [...s].some(c => !ALPHA.includes(c))) return null;
  return s.slice(0, 4) + "-" + s.slice(4);
}

function site(env: Env): string {
  return (env as unknown as { DEVICE_URL?: string }).DEVICE_URL || DEVICE_SITE + "/device";
}

export async function deviceStart(req: Request, env: Env): Promise<Response> {
  softLimit(`device:start:${clientIP(req)}`, 10, 600);
  const b = await readJSON(req).catch(() => ({} as Record<string, unknown>));
  const client = cleanName(b.client) || "Blaze CLI";
  const device_code = "dev_" + randomToken(32);
  let user_code = newUserCode();
  for (let i = 0; i < 3 && (await env.KV.get(`devuser:${user_code}`)); i++) user_code = newUserCode();
  const key = `device:${await sha256(device_code)}`;
  const rec: DeviceRecord = { user_code, client, status: "pending", exp: now() + DEVICE_TTL };
  await env.KV.put(key, JSON.stringify(rec), { expirationTtl: DEVICE_TTL });
  await env.KV.put(`devuser:${user_code}`, key, { expirationTtl: DEVICE_TTL });
  const uri = site(env);
  return json({ device_code, user_code, verification_uri: uri, verification_uri_complete: `${uri}?code=${user_code}`, interval: DEVICE_INTERVAL, expires_in: DEVICE_TTL });
}

async function byUserCode(env: Env, raw: unknown): Promise<{ key: string; rec: DeviceRecord; code: string }> {
  const code = normUserCode(raw);
  if (!code) throw new HttpError(400, "invalid_user_code");
  const key = await env.KV.get(`devuser:${code}`);
  const rec = key ? await env.KV.get<DeviceRecord>(key, "json") : null;
  if (!key || !rec || rec.exp < now() || rec.status !== "pending") throw new HttpError(400, "invalid_user_code");
  return { key, rec, code };
}

export async function deviceInfo(req: Request, env: Env): Promise<Response> {
  softLimit(`device:info:${clientIP(req)}`, 30, 600);
  const { rec } = await byUserCode(env, new URL(req.url).searchParams.get("code"));
  return json({ client: rec.client, expires_in: Math.max(0, rec.exp - now()) });
}

export async function deviceApprove(req: Request, env: Env): Promise<Response> {
  const { user: u } = await requireUser(env, req);
  softLimit(`device:approve:${u.id}`, 10, 600);
  const b = await readJSON(req);
  const { key, rec, code } = await byUserCode(env, b.user_code);
  rec.status = "approved";
  rec.uid = u.id;
  await env.KV.put(key, JSON.stringify(rec), { expirationTtl: Math.max(60, rec.exp - now()) });
  await env.KV.delete(`devuser:${code}`);
  return json({ ok: true, client: rec.client });
}

export async function deviceDeny(req: Request, env: Env): Promise<Response> {
  softLimit(`device:deny:${clientIP(req)}`, 20, 600);
  const b = await readJSON(req);
  const { key, rec, code } = await byUserCode(env, b.user_code);
  rec.status = "denied";
  await env.KV.put(key, JSON.stringify(rec), { expirationTtl: Math.max(60, rec.exp - now()) });
  await env.KV.delete(`devuser:${code}`);
  return json({ ok: true });
}

export async function devicePoll(req: Request, env: Env): Promise<Response> {
  softLimit(`device:poll:${clientIP(req)}`, 300, 600);
  const b = await readJSON(req);
  const dc = typeof b.device_code === "string" ? b.device_code : "";
  if (!/^dev_[A-Za-z0-9_-]{32,100}$/.test(dc)) throw new HttpError(400, "expired_token");
  const key = `device:${await sha256(dc)}`;
  const t = Date.now();
  const prev = lastPoll.get(key);
  lastPoll.set(key, t);
  if (lastPoll.size > 5000) lastPoll.clear();
  const rec = await env.KV.get<DeviceRecord>(key, "json");
  if (!rec || rec.exp < now()) throw new HttpError(400, "expired_token");
  if (rec.status === "denied") {
    await env.KV.delete(key);
    throw new HttpError(400, "access_denied");
  }
  if (rec.status === "pending") {
    if (prev && t - prev < (DEVICE_INTERVAL - 1) * 1000) throw new HttpError(400, "slow_down");
    throw new HttpError(400, "authorization_pending");
  }
  const u = rec.uid ? await getUser(env, rec.uid) : null;
  await env.KV.delete(key); // first good poll wins
  if (!u) throw new HttpError(400, "access_denied");
  const token = await createSession(env, u, cleanName(b.client) || rec.client);
  return json({ token, user: publicUser(u) });
}
