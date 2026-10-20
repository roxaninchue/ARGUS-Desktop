// Blaze accounts, kept in KV only (free tier):
//   user:<id>                   the account (JSON)
//   email:<lower-email>         -> user id
//   ident:<provider>:<subject>  -> user id (Google / GitHub sign-in)
//   sess:<sha256(token)>        a session, expires by TTL (90 days)
//   rl:<kind>:<key>             rate-limit counters, expire by TTL
//   keys:<id>, keykey:<id>      synced model keys (encrypted blob) and its key, see keys.ts
//   reset:<sha256(token)>       password-reset link {uid}, 1 h, single use (0.9.0)
//   mfa:<sha256(token)>         sign-in waiting for its second factor, 5 min (0.9.0, mfa.ts)
//   webcode:<sha256(code)>      one-time code from a web sign-in redirect, 2 min (0.9.0)
// Session tokens are opaque random strings; only their SHA-256 is stored.

import { Env, HttpError, randomToken, sha256 } from "./util";
import { deleteKeys } from "./keys";
import { deleteSkills } from "./skills";
import type { Profile, ProviderId } from "./providers";

export const SESSION_TTL = 90 * 24 * 3600;
const MAX_SESSIONS = 20;

export interface Identity {
  provider: ProviderId;
  subject: string;
  email: string | null;
}

export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url: string;
  password_hash?: string;
  email_verified: boolean;
  identities: Identity[];
  sessions: string[]; // session hashes, oldest first
  totp_secret?: string; // 0.9.0: active TOTP secret (base32); see mfa.ts
  totp_pending?: string; // set up but not yet confirmed
  totp_recovery?: string[]; // sha256 of unused recovery codes
  totp_last?: number; // last accepted TOTP time step
  avatar_off?: boolean; // 0.9.3: user removed their picture; don't refill it from Google/GitHub
  mfa_email?: boolean; // 0.9.1: emailed sign-in codes are this account's second factor
  email_code?: { h: string; exp: number; tries: number; purpose: string }; // 0.9.1: pending emailed code for an account action
  created_at: number;
  updated_at: number;
}

interface Session {
  uid: string;
  created: number;
  exp: number;
  client: string;
}

export function publicUser(u: User) {
  return {
    id: u.id,
    email: u.email,
    name: u.name,
    avatar_url: u.avatar_url,
    email_verified: u.email_verified,
    has_password: !!u.password_hash,
    two_factor: !!(u.totp_secret || u.mfa_email),
    two_factor_methods: { app: !!u.totp_secret, email: !!u.mfa_email },
    providers: u.identities.map(i => i.provider),
    created_at: u.created_at,
  };
}

const now = () => Math.floor(Date.now() / 1000);

export async function getUser(env: Env, id: string): Promise<User | null> {
  return (await env.KV.get<User>(`user:${id}`, "json")) || null;
}

export async function putUser(env: Env, u: User): Promise<void> {
  u.updated_at = now();
  await env.KV.put(`user:${u.id}`, JSON.stringify(u));
}

export async function userByEmail(env: Env, email: string): Promise<User | null> {
  const id = await env.KV.get(`email:${email}`);
  return id ? getUser(env, id) : null;
}

export async function createUser(env: Env, f: { email: string; name: string; avatar?: string; passwordHash?: string; verified: boolean }): Promise<User> {
  const t = now();
  const u: User = {
    id: "u_" + randomToken(12), email: f.email, name: f.name, avatar_url: f.avatar || "",
    email_verified: f.verified, identities: [], sessions: [], created_at: t, updated_at: t,
  };
  if (f.passwordHash) u.password_hash = f.passwordHash;
  await env.KV.put(`email:${f.email}`, u.id);
  await putUser(env, u);
  return u;
}

// ---------------- sessions ----------------

/** Makes a session and saves the user (its session list changes). */
export async function createSession(env: Env, u: User, client: string): Promise<string> {
  const token = "bz_" + randomToken(32);
  const h = await sha256(token);
  const t = now();
  const s: Session = { uid: u.id, created: t, exp: t + SESSION_TTL, client: client.slice(0, 80) };
  await env.KV.put(`sess:${h}`, JSON.stringify(s), { expirationTtl: SESSION_TTL });
  u.sessions.push(h);
  while (u.sessions.length > MAX_SESSIONS) {
    const old = u.sessions.shift();
    if (old) await env.KV.delete(`sess:${old}`);
  }
  await putUser(env, u);
  return token;
}

export async function revokeAllSessions(env: Env, u: User): Promise<void> {
  for (const h of u.sessions) await env.KV.delete(`sess:${h}`);
  u.sessions = [];
}

function bearer(req: Request): string | null {
  const a = req.headers.get("Authorization") || "";
  const m = /^Bearer\s+(bz_[A-Za-z0-9_-]{20,100})$/.exec(a);
  return m ? (m[1] as string) : null;
}

/** The signed-in user and their session hash, or 401. */
export async function requireUser(env: Env, req: Request): Promise<{ user: User; hash: string }> {
  const tok = bearer(req);
  if (!tok) throw new HttpError(401, "unauthorized");
  const h = await sha256(tok);
  const s = await env.KV.get<Session>(`sess:${h}`, "json");
  if (!s || s.exp < now()) throw new HttpError(401, "unauthorized");
  const u = await getUser(env, s.uid);
  if (!u || !u.sessions.includes(h)) throw new HttpError(401, "unauthorized");
  return { user: u, hash: h };
}

export async function endSession(env: Env, u: User, hash: string): Promise<void> {
  await env.KV.delete(`sess:${hash}`);
  u.sessions = u.sessions.filter(x => x !== hash);
  await putUser(env, u);
}

export async function deleteAccount(env: Env, u: User): Promise<void> {
  await revokeAllSessions(env, u);
  for (const i of u.identities) await env.KV.delete(`ident:${i.provider}:${i.subject}`);
  const owner = await env.KV.get(`email:${u.email}`);
  if (owner === u.id) await env.KV.delete(`email:${u.email}`);
  await env.KV.delete(`user:${u.id}`);
  await deleteKeys(env, u.id);
  await deleteSkills(env, u.id);
  await env.KV.delete(`avatar:${u.id}`);
}

// ---------------- sign-in with Google / GitHub ----------------

/**
 * Finds or creates the account for a provider identity. A provider-verified email
 * links to an existing account with that email. If that account was made with an
 * unverified email + password, the verified owner wins: the password and every
 * session on it are dropped (stops someone pre-registering your address).
 */
export async function userForIdentity(env: Env, provider: ProviderId, p: Profile): Promise<User> {
  const ik = `ident:${provider}:${p.subject}`;
  const linked = await env.KV.get(ik);
  if (linked) {
    const u = await getUser(env, linked);
    if (u) {
      let dirty = false;
      if (!u.avatar_url && !u.avatar_off && p.avatar) (u.avatar_url = p.avatar), (dirty = true);
      if (!u.name && p.name) (u.name = p.name), (dirty = true);
      if (dirty) await putUser(env, u);
      return u;
    }
  }
  const email = p.email && p.emailVerified ? p.email.trim().toLowerCase() : null;
  let u: User | null = email ? await userByEmail(env, email) : null;
  if (u) {
    if (!u.email_verified && u.password_hash) {
      delete u.password_hash;
      // 0.9.0: a squatter's two-factor goes too, or it would lock the real owner out.
      delete u.totp_secret;
      delete u.totp_pending;
      delete u.totp_recovery;
      delete u.totp_last;
      delete u.mfa_email;
      delete u.email_code;
      await revokeAllSessions(env, u);
    }
    u.email_verified = true;
    if (!u.name) u.name = p.name;
    if (!u.avatar_url && !u.avatar_off) u.avatar_url = p.avatar;
  } else {
    // No verified email: give the account a private placeholder so it can't collide.
    const addr = email || `${provider}-${p.subject}@users.blaze.invalid`;
    u = await createUser(env, { email: addr, name: p.name, avatar: p.avatar, verified: !!email });
  }
  u.identities = u.identities.filter(i => !(i.provider === provider && i.subject === p.subject));
  u.identities.push({ provider, subject: p.subject, email: p.email });
  await env.KV.put(ik, u.id);
  await putUser(env, u);
  return u;
}

// ---------------- rate limits ----------------

/** Throws 429 when key has reached max hits in its window. Reads only. */
export async function checkLimit(env: Env, key: string, max: number): Promise<void> {
  const v = await env.KV.get<{ n: number; reset: number }>(`rl:${key}`, "json");
  if (v && v.n >= max && v.reset > now()) throw new HttpError(429, "rate_limited", { retry_after: v.reset - now() });
}

/** Counts one hit (KV writes are scarce on the free tier, so only failures count). */
export async function hit(env: Env, key: string, windowSec: number): Promise<void> {
  const k = `rl:${key}`;
  const v = await env.KV.get<{ n: number; reset: number }>(k, "json");
  const t = now();
  const cur = v && v.reset > t ? v : { n: 0, reset: t + windowSec };
  cur.n++;
  await env.KV.put(k, JSON.stringify(cur), { expirationTtl: Math.max(60, cur.reset - t) });
}
