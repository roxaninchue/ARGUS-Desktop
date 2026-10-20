// Two-factor sign-in (0.9.0): TOTP per RFC 6238 (HMAC-SHA1, 6 digits, 30 s steps, ±1 step of drift)
// plus 10 single-use recovery codes, stored as SHA-256 hashes on the user record.
//   user.totp_pending   secret from /v1/auth/2fa/setup, until /enable confirms a code
//   user.totp_secret    the active secret (two_factor = true)
//   user.totp_recovery  sha256 of each unused recovery code
//   user.totp_last      the last accepted time step (a code can't be replayed)
// A sign-in on a 2FA account answers 401 {error:"mfa_required", mfa_token}; the token lives under
// mfa:<sha256(token)> for 5 minutes, allows 5 tries and is single use (/v1/auth/2fa/verify).

import { Env, HttpError, randomToken, sha256 } from "./util";
import type { User } from "./accounts";

const B32 = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
export const MFA_TTL = 300;
export const MFA_TRIES = 5;
export const RECOVERY_COUNT = 10;
const STEP = 30;

export function base32Encode(b: Uint8Array): string {
  let bits = 0, val = 0, out = "";
  for (const x of b) {
    val = (val << 8) | x;
    bits += 8;
    while (bits >= 5) {
      out += B32[(val >>> (bits - 5)) & 31];
      bits -= 5;
    }
  }
  if (bits > 0) out += B32[(val << (5 - bits)) & 31];
  return out;
}

export function base32Decode(s: string): Uint8Array {
  const clean = s.toUpperCase().replace(/[\s=-]/g, "");
  const out: number[] = [];
  let bits = 0, val = 0;
  for (const c of clean) {
    const i = B32.indexOf(c);
    if (i < 0) throw new Error("bad base32");
    val = (val << 5) | i;
    bits += 5;
    if (bits >= 8) {
      out.push((val >>> (bits - 8)) & 255);
      bits -= 8;
    }
  }
  return new Uint8Array(out);
}

export function newTotpSecret(): string {
  const b = new Uint8Array(20);
  crypto.getRandomValues(b);
  return base32Encode(b);
}

/** The 6-digit code for one time step. */
export async function totpAt(secret: string, step: number): Promise<string> {
  const key = await crypto.subtle.importKey("raw", base32Decode(secret), { name: "HMAC", hash: "SHA-1" }, false, ["sign"]);
  const msg = new Uint8Array(8);
  let c = step;
  for (let i = 7; i >= 0; i--) {
    msg[i] = c & 255;
    c = Math.floor(c / 256);
  }
  const h = new Uint8Array(await crypto.subtle.sign("HMAC", key, msg));
  const off = (h[19] as number) & 15;
  const bin = (((h[off] as number) & 127) << 24) | ((h[off + 1] as number) << 16) | ((h[off + 2] as number) << 8) | (h[off + 3] as number);
  return String(bin % 1_000_000).padStart(6, "0");
}

export const timeStep = (ms = Date.now()) => Math.floor(ms / 1000 / STEP);

/** The matching step within ±1 of now, newer than [after]; null when the code is wrong. */
export async function matchTotp(secret: string, code: string, after = -1, ms = Date.now()): Promise<number | null> {
  if (!/^\d{6}$/.test(code)) return null;
  const t = timeStep(ms);
  let hit: number | null = null;
  for (const s of [t - 1, t, t + 1]) if (s > after && (await totpAt(secret, s)) === code) hit = s; // check all three: no early exit
  return hit;
}

export function otpauthURI(secret: string, email: string): string {
  return `otpauth://totp/Blaze:${encodeURIComponent(email)}?secret=${secret}&issuer=Blaze`;
}

const normRecovery = (c: string) => c.toLowerCase().replace(/[^a-z0-9]/g, "");

export async function newRecoveryCodes(): Promise<{ codes: string[]; hashes: string[] }> {
  const codes: string[] = [];
  for (let i = 0; i < RECOVERY_COUNT; i++) {
    const b = new Uint8Array(7);
    crypto.getRandomValues(b);
    const s = base32Encode(b).toLowerCase().slice(0, 10);
    codes.push(`${s.slice(0, 5)}-${s.slice(5)}`);
  }
  return { codes, hashes: await Promise.all(codes.map(c => sha256("rc:" + normRecovery(c)))) };
}

/**
 * Checks a TOTP code (6 digits) or a recovery code against the user's active 2FA and consumes it:
 * a TOTP step is remembered, a recovery code is removed. Mutates [u]; the caller saves it.
 */
export async function consumeSecondFactor(u: User, raw: unknown): Promise<boolean> {
  if (!u.totp_secret || typeof raw !== "string" || raw.length > 64) return false;
  const code = raw.replace(/\s/g, "");
  if (/^\d{6}$/.test(code)) {
    const s = await matchTotp(u.totp_secret, code, u.totp_last ?? -1);
    if (s === null) return false;
    u.totp_last = s;
    return true;
  }
  const n = normRecovery(code);
  if (n.length !== 10) return false;
  const h = await sha256("rc:" + n);
  const list = u.totp_recovery || [];
  if (!list.includes(h)) return false;
  u.totp_recovery = list.filter(x => x !== h);
  return true;
}

export interface MfaRecord {
  uid: string;
  client: string;
  tries: number;
  exp: number;
}

/** Parks a sign-in that still needs the second factor and throws 401 mfa_required. */
export async function requireSecondFactor(env: Env, u: User, client: string): Promise<never> {
  const tok = "mfa_" + randomToken(32);
  const rec: MfaRecord = { uid: u.id, client, tries: 0, exp: Math.floor(Date.now() / 1000) + MFA_TTL };
  await env.KV.put(`mfa:${await sha256(tok)}`, JSON.stringify(rec), { expirationTtl: MFA_TTL });
  throw new HttpError(401, "mfa_required", { mfa_token: tok });
}
