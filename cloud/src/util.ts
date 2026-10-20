// Small helpers: encoding, hashing, passwords, validation and JSON responses.

export interface Env {
  KV: KVNamespace;
  BASE_URL: string;
  ALLOWED_ORIGINS?: string;
  PBKDF2_ITERATIONS?: string;
  GOOGLE_CLIENT_ID?: string;
  GOOGLE_CLIENT_SECRET?: string;
  GITHUB_CLIENT_ID?: string;
  GITHUB_CLIENT_SECRET?: string;
  SPOTIFY_CLIENT_ID?: string;
  SPOTIFY_CLIENT_SECRET?: string;
  NOTION_CLIENT_ID?: string;
  NOTION_CLIENT_SECRET?: string;
  SLACK_CLIENT_ID?: string;
  SLACK_CLIENT_SECRET?: string;
  /** Service binding to blaze-gateway (0.9.0): mail goes out through POST https://gateway/internal/mail. */
  GATEWAY?: Fetcher;
  /** Secret (0.9.0), the same value as on blaze-gateway: sent as X-Internal-Token on /internal/mail. */
  INTERNAL_TOKEN?: string;
}

const enc = new TextEncoder();

export function b64url(bytes: ArrayBuffer | Uint8Array): string {
  const b = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
  let s = "";
  for (const x of b) s += String.fromCharCode(x);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

export function fromB64url(s: string): Uint8Array {
  const p = s.replace(/-/g, "+").replace(/_/g, "/") + "===".slice((s.length + 3) % 4);
  const bin = atob(p);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

export function randomToken(bytes = 32): string {
  const b = new Uint8Array(bytes);
  crypto.getRandomValues(b);
  return b64url(b);
}

export async function sha256(s: string): Promise<string> {
  return b64url(await crypto.subtle.digest("SHA-256", enc.encode(s)));
}

/** Constant-time string compare (both sides are ours or fixed-length hashes). */
export function safeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  let d = 0;
  for (let i = 0; i < a.length; i++) d |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return d === 0;
}

/** PKCE S256: BASE64URL(SHA256(verifier)). */
export async function pkceChallenge(verifier: string): Promise<string> {
  return sha256(verifier);
}

// ---------------- passwords (PBKDF2-SHA256 via WebCrypto) ----------------

const MAX_ITER = 100000; // the Workers runtime refuses more

export function iterations(env: Env): number {
  const n = parseInt(env.PBKDF2_ITERATIONS || "", 10);
  return Number.isFinite(n) && n >= 10000 ? Math.min(n, MAX_ITER) : MAX_ITER;
}

async function pbkdf2(password: string, salt: Uint8Array, iter: number): Promise<Uint8Array> {
  const key = await crypto.subtle.importKey("raw", enc.encode(password), "PBKDF2", false, ["deriveBits"]);
  const bits = await crypto.subtle.deriveBits({ name: "PBKDF2", hash: "SHA-256", salt, iterations: iter }, key, 256);
  return new Uint8Array(bits);
}

/** "pbkdf2$sha256$<iter>$<salt>$<hash>" */
export async function hashPassword(password: string, iter: number): Promise<string> {
  const salt = new Uint8Array(16);
  crypto.getRandomValues(salt);
  const h = await pbkdf2(password, salt, iter);
  return `pbkdf2$sha256$${iter}$${b64url(salt)}$${b64url(h)}`;
}

export async function verifyPassword(password: string, stored: string): Promise<boolean> {
  const p = stored.split("$");
  if (p.length !== 5 || p[0] !== "pbkdf2" || p[1] !== "sha256") return false;
  const iter = parseInt(p[2] || "", 10);
  if (!Number.isFinite(iter) || iter < 1 || iter > MAX_ITER) return false;
  const h = await pbkdf2(password, fromB64url(p[3] || ""), iter);
  return safeEqual(b64url(h), p[4] || "");
}

// ---------------- validation ----------------

const EMAIL_RE = /^[^\s@<>()[\]\\,;:"]{1,64}@[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+$/;

export function normEmail(v: unknown): string | null {
  if (typeof v !== "string") return null;
  const e = v.trim().toLowerCase();
  return e.length <= 254 && EMAIL_RE.test(e) ? e : null;
}

export function validPassword(v: unknown): v is string {
  return typeof v === "string" && v.length >= 8 && v.length <= 256;
}

export function cleanName(v: unknown): string {
  if (typeof v !== "string") return "";
  return v.replace(/[\u0000-\u001f\u007f]/g, "").trim().slice(0, 80);
}

/** Only http://127.0.0.1:<port>/... or http://[::1]:<port>/... may receive a code. */
export function loopbackRedirect(v: string | null): string | null {
  if (!v || v.length > 200) return null;
  let u: URL;
  try {
    u = new URL(v);
  } catch {
    return null;
  }
  if (u.protocol !== "http:" || u.username || u.password || u.hash || u.search) return null;
  if (u.hostname !== "127.0.0.1" && u.hostname !== "[::1]") return null;
  const port = parseInt(u.port, 10);
  if (!Number.isFinite(port) || port < 1024 || port > 65535) return null;
  return u.toString();
}

/** 0.9.5: the Android app's custom-scheme callback. Exact match only; PKCE S256 + state bind the code to the app. */
export const APP_REDIRECT = "blaze://auth/cb";
export function appRedirect(v: string | null): string | null {
  return v === APP_REDIRECT ? APP_REDIRECT : null;
}

/** HTML hand-off to a custom-scheme URL: browsers often block a silent 302 to blaze://, so script + meta refresh + a button. */
export function appHandoff(url: string): Response {
  const nonce = randomToken(12);
  const e = esc(url);
  const body = `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="referrer" content="no-referrer"><title>Blaze</title><meta http-equiv="refresh" content="0;url=${e}"><body style="font:16px/1.5 system-ui,sans-serif;background:#f5f5f7;color:#1d1d1f;display:grid;place-items:center;min-height:100vh;margin:0;text-align:center"><div style="padding:32px"><h2 style="margin:0 0 8px;font-weight:600">Signed in</h2><p style="margin:0 0 20px;color:#6e6e73">Returning you to Blaze…</p><a href="${e}" style="display:inline-block;padding:12px 24px;border-radius:999px;background:#0071e3;color:#fff;text-decoration:none;font-weight:500">Open Blaze</a></div><script nonce="${nonce}">location.href=${JSON.stringify(url).replace(/</g, "\\u003c")};</script></body>`;
  return new Response(body, {
    status: 200,
    headers: { ...BASE_HEADERS, "Content-Type": "text/html; charset=utf-8", "Cache-Control": "no-store", "Content-Security-Policy": `default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-${nonce}'; frame-ancestors 'none'; base-uri 'none'`, "Referrer-Policy": "no-referrer" },
  });
}

export const STATE_RE = /^[A-Za-z0-9_-]{16,128}$/;
export const CHALLENGE_RE = /^[A-Za-z0-9_-]{43}$/;
export const VERIFIER_RE = /^[A-Za-z0-9._~-]{43,128}$/;
export const CODE_RE = /^[A-Za-z0-9_-]{20,128}$/;

// ---------------- HTTP ----------------

export class HttpError extends Error {
  constructor(public status: number, public code: string, public extra: Record<string, unknown> = {}) {
    super(code);
  }
}

const BASE_HEADERS: Record<string, string> = {
  "Cache-Control": "no-store",
  "X-Content-Type-Options": "nosniff",
  "Referrer-Policy": "no-referrer",
  "X-Frame-Options": "DENY",
};

export function json(data: unknown, status = 200, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { ...BASE_HEADERS, "Content-Type": "application/json; charset=utf-8", ...headers },
  });
}

export function redirect(to: string): Response {
  return new Response(null, { status: 302, headers: { ...BASE_HEADERS, Location: to } });
}

export function html(body: string, status = 200): Response {
  return new Response(body, {
    status,
    headers: { ...BASE_HEADERS, "Content-Type": "text/html; charset=utf-8", "Content-Security-Policy": "default-src 'none'; style-src 'unsafe-inline'" },
  });
}

export function esc(s: string): string {
  return s.replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c] as string);
}

export async function readJSON(req: Request, max = 16384): Promise<Record<string, unknown>> {
  const len = parseInt(req.headers.get("Content-Length") || "0", 10);
  if (len > max) throw new HttpError(413, "too_large");
  const text = await req.text();
  if (text.length > max) throw new HttpError(413, "too_large");
  if (!text) return {};
  try {
    const v = JSON.parse(text);
    if (v && typeof v === "object" && !Array.isArray(v)) return v as Record<string, unknown>;
  } catch {
    /* fallthrough */
  }
  throw new HttpError(400, "invalid_json");
}

// Best-effort per-isolate limiter for unauthenticated endpoints that would otherwise
// spend scarce KV writes (free tier: 1,000 writes a day).
const buckets = new Map<string, { n: number; reset: number }>();
export function softLimit(key: string, max: number, windowSec: number): void {
  const t = Date.now();
  if (buckets.size > 5000) buckets.clear();
  const b = buckets.get(key);
  if (!b || b.reset < t) {
    buckets.set(key, { n: 1, reset: t + windowSec * 1000 });
    return;
  }
  if (++b.n > max) throw new HttpError(429, "rate_limited", { retry_after: Math.ceil((b.reset - t) / 1000) });
}

/** An https URL whose origin is in ALLOWED_ORIGINS (web sign-in and reset links come back there), else null. */
export function webReturn(env: Env, v: string | null | undefined): URL | null {
  if (!v || typeof v !== "string" || v.length > 500) return null;
  let u: URL;
  try {
    u = new URL(v);
  } catch {
    return null;
  }
  if (u.protocol !== "https:" || u.username || u.password) return null;
  const allowed = (env.ALLOWED_ORIGINS || "").split(",").map(s => s.trim()).filter(Boolean);
  if (!allowed.includes(u.origin)) return null;
  u.hash = "";
  return u;
}

export function clientIP(req: Request): string {
  return req.headers.get("CF-Connecting-IP") || "0.0.0.0";
}
