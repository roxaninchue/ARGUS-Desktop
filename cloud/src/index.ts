// blaze-auth: Blaze accounts + the one-click OAuth connector broker.
// Cloudflare Workers + one KV namespace, free tier. See ../README.md.

import {
  Env, HttpError, json, redirect, html, esc, readJSON, clientIP, randomToken, sha256, pkceChallenge, safeEqual,
  hashPassword, verifyPassword, iterations, normEmail, validPassword, cleanName, loopbackRedirect, appRedirect, appHandoff,
  STATE_RE, CHALLENGE_RE, VERIFIER_RE, CODE_RE, softLimit, webReturn,
} from "./util";
import { PROVIDERS, ProviderId, provider, credentials, callbackURL, pickScopes, tokenRequest, fetchProfile, Tokens } from "./providers";
import {
  publicUser, userByEmail, createUser, createSession, requireUser, endSession, deleteAccount, getUser, userForIdentity,
  checkLimit, hit,
} from "./accounts";
import { getKeys, putKeys } from "./keys";
import { getSkills, putSkills } from "./skills";
import { comments } from "./comments";
import { putAvatar, deleteAvatar, getAvatar } from "./avatar";
import { guard } from "./ratelimit";
import { deviceStart, deviceInfo, deviceApprove, deviceDeny, devicePoll } from "./device";
import { forgot, reset, setPassword, mfaSetup, mfaEnable, mfaDisable, mfaVerify, signInOrMfa, mailerConfigured, mfaEmailLogin, mfaEmailSend, mfaEmailEnable, mfaEmailDisable } from "./password";

export const VERSION = "0.9.5";
const STATE_TTL = 600; // the user has 10 minutes to finish at the provider
const CODE_TTL = 120; // the desktop redeems within seconds of the loopback redirect; 2 minutes is slack, and the KV minimum is 60

/** What /start leaves in KV under oauthstate:<state>; the callback reads it to know the flow. */
interface StateRecord {
  flow: "login" | "connect" | "web";
  provider: ProviderId;
  redirect: string; // desktop loopback, or (flow "web") the https page to come back to
  ds: string; // the desktop's own state, echoed back ("" for web)
  cc: string; // the desktop's PKCE challenge ("" for web)
  scopes: string[];
  exp: number;
}

/** What a web sign-in callback leaves under webcode:<sha256(code)> for /v1/auth/web/redeem (0.9.0). */
interface WebCodeRecord {
  uid: string;
  provider: ProviderId;
  origin: string; // the page that started it; a redeem from another browser origin is refused
  exp: number;
}

/** What the callback leaves under oauthcode:<sha256(code)> for /oauth/redeem. */
interface CodeRecord {
  kind: "tokens" | "login";
  provider: ProviderId;
  cc: string;
  exp: number;
  tokens?: Tokens;
  scopes?: string[];
  uid?: string;
}

const now = () => Math.floor(Date.now() / 1000);

// ---------------- CORS ----------------

function allowedOrigins(env: Env): string[] {
  return (env.ALLOWED_ORIGINS || "").split(",").map(s => s.trim()).filter(Boolean);
}

function cors(env: Env, req: Request): Record<string, string> | null {
  const o = req.headers.get("Origin");
  if (!o) return {};
  if (!allowedOrigins(env).includes(o)) return null;
  return {
    "Access-Control-Allow-Origin": o,
    "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
    "Access-Control-Allow-Headers": "Authorization, Content-Type",
    "Access-Control-Max-Age": "600",
    Vary: "Origin",
  };
}

// ---------------- router ----------------

export default {
  async fetch(req: Request, env: Env): Promise<Response> {
    // Shared-page comments are open to every origin (shipped sites call them from the browser).
    const cpath = new URL(req.url).pathname.replace(/\/+$/, "");
    if (cpath.startsWith("/v1/comments")) {
      const r = await comments(req, env, cpath, key => softLimit(key, 30, 600)).catch(e => {
        if (e instanceof HttpError) return json({ error: e.code, ...e.extra }, e.status, { "Access-Control-Allow-Origin": "*" });
        throw e;
      });
      if (r) return r;
    }
    const ch = cors(env, req);
    if (req.method === "OPTIONS") {
      return ch && req.headers.get("Origin") ? new Response(null, { status: 204, headers: ch }) : json({ error: "forbidden_origin" }, 403);
    }
    // A browser page from an origin we don't trust may not drive the API.
    if (ch === null && req.method !== "GET") return json({ error: "forbidden_origin" }, 403);
    let res: Response;
    try {
      await guard(req, env, new URL(req.url).pathname.replace(/\/+$/, "") || "/");
      res = await route(req, env);
    } catch (e) {
      if (e instanceof HttpError) res = json({ error: e.code, ...e.extra }, e.status, e.status === 429 ? { "Retry-After": String(e.extra.retry_after || 60) } : {});
      else {
        console.error("unhandled", (e as Error)?.name || "Error");
        res = json({ error: "server_error" }, 500);
      }
    }
    if (ch && Object.keys(ch).length) {
      const h = new Headers(res.headers);
      for (const [k, v] of Object.entries(ch)) h.set(k, v);
      res = new Response(res.body, { status: res.status, headers: h });
    }
    return res;
  },
};

async function route(req: Request, env: Env): Promise<Response> {
  const url = new URL(req.url);
  const path = url.pathname.replace(/\/+$/, "") || "/";
  const m = req.method;
  let r: RegExpExecArray | null;

  if (path === "/" || path === "/health") {
    if (m !== "GET") throw new HttpError(405, "method_not_allowed");
    return json({ ok: true, service: "blaze-auth", version: VERSION });
  }
  if (path === "/v1/providers" && m === "GET") return providers(env);
  if (path === "/v1/auth/signup" && m === "POST") return signup(req, env);
  if (path === "/v1/auth/login" && m === "POST") return login(req, env);
  if (path === "/v1/auth/web/redeem" && m === "POST") return webRedeem(req, env);
  if (path === "/v1/auth/password/forgot" && m === "POST") return forgot(req, env);
  if (path === "/v1/auth/password/reset" && m === "POST") return reset(req, env);
  if (path === "/v1/auth/password/set" && m === "POST") return setPassword(req, env);
  if (path === "/v1/auth/2fa/setup" && m === "POST") return mfaSetup(req, env);
  if (path === "/v1/auth/2fa/enable" && m === "POST") return mfaEnable(req, env);
  if (path === "/v1/auth/2fa/disable" && m === "POST") return mfaDisable(req, env);
  if (path === "/v1/auth/2fa/verify" && m === "POST") return mfaVerify(req, env);
  if (path === "/v1/auth/device/start" && m === "POST") return deviceStart(req, env);
  if (path === "/v1/auth/device/info" && m === "GET") return deviceInfo(req, env);
  if (path === "/v1/auth/device/approve" && m === "POST") return deviceApprove(req, env);
  if (path === "/v1/auth/device/deny" && m === "POST") return deviceDeny(req, env);
  if (path === "/v1/auth/device/poll" && m === "POST") return devicePoll(req, env);
  if (path === "/device" && m === "GET") return Response.redirect("https://blazeinchue.pages.dev/device" + new URL(req.url).search, 302);
  if (path === "/v1/auth/2fa/email" && m === "POST") return mfaEmailLogin(req, env);
  if (path === "/v1/auth/2fa/email/send" && m === "POST") return mfaEmailSend(req, env);
  if (path === "/v1/auth/2fa/email/enable" && m === "POST") return mfaEmailEnable(req, env);
  if (path === "/v1/auth/2fa/email/disable" && m === "POST") return mfaEmailDisable(req, env);
  if ((path === "/oauth/redeem" || path === "/v1/auth/redeem") && m === "POST") return redeem(req, env);
  if (path === "/v1/me" && m === "GET") {
    const { user } = await requireUser(env, req);
    return json({ user: publicUser(user) });
  }
  if (path === "/v1/logout" && m === "POST") {
    const { user, hash } = await requireUser(env, req);
    await endSession(env, user, hash);
    return json({ ok: true });
  }
  if (path === "/v1/keys" && (m === "GET" || m === "POST")) {
    const { user } = await requireUser(env, req);
    if (m === "GET") return getKeys(env, user);
    softLimit(`keys:${user.id}`, 60, 600); // each write spends a scarce KV write
    return putKeys(env, user, req);
  }
  if (path === "/v1/skills" && (m === "GET" || m === "POST")) {
    const { user } = await requireUser(env, req);
    if (m === "GET") return getSkills(env, user);
    softLimit(`skills:${user.id}`, 60, 600);
    return putSkills(env, user, req);
  }
  if (path === "/v1/me/avatar" && m === "POST") {
    const { user } = await requireUser(env, req);
    softLimit(`avatar:${user.id}`, 20, 600);
    return putAvatar(req, env, user);
  }
  if (path === "/v1/me/avatar/delete" && m === "POST") {
    const { user } = await requireUser(env, req);
    softLimit(`avatar:${user.id}`, 20, 600);
    return deleteAvatar(env, user);
  }
  if ((r = /^\/v1\/avatar\/([A-Za-z0-9_-]{1,64})$/.exec(path)) && (m === "GET" || m === "HEAD")) return getAvatar(env, r[1] as string);
  if (path === "/v1/account/delete" && m === "POST") {
    const { user } = await requireUser(env, req);
    const b = await readJSON(req);
    if (b.confirm !== "delete") throw new HttpError(400, "confirm_required", { hint: 'send {"confirm":"delete"}' });
    await deleteAccount(env, user);
    return json({ ok: true, deleted: true });
  }
  if ((r = /^\/v1\/auth\/([a-z]+)\/start$/.exec(path)) && m === "GET") return start(req, env, r[1] as string, "login");
  if ((r = /^\/v1\/auth\/([a-z]+)\/web$/.exec(path)) && m === "GET") return startWeb(req, env, r[1] as string);
  if ((r = /^\/oauth\/([a-z]+)\/start$/.exec(path)) && m === "GET") return start(req, env, r[1] as string, "connect");
  if ((r = /^\/oauth\/([a-z]+)\/callback$/.exec(path)) && m === "GET") return callback(req, env, r[1] as string);
  if ((r = /^\/oauth\/([a-z]+)\/refresh$/.exec(path)) && m === "POST") return refresh(req, env, r[1] as string);
  throw new HttpError(404, "not_found");
}

// ---------------- discovery ----------------

function providers(env: Env): Response {
  const connectors: Record<string, boolean> = {};
  for (const id of Object.keys(PROVIDERS) as ProviderId[]) connectors[id] = !!credentials(env, id);
  return json({
    version: VERSION,
    base_url: env.BASE_URL,
    login: { email: true, google: !!credentials(env, "google"), github: !!credentials(env, "github") },
    connectors,
    web_login: true,
    password_reset: mailerConfigured(env),
    two_factor: true,
    two_factor_methods: { app: true, email: mailerConfigured(env) },
    device_login: true,
    features: { app_redirect: true },
    app_redirect: "blaze://auth/cb",
  });
}

// ---------------- email + password ----------------

async function signup(req: Request, env: Env): Promise<Response> {
  const ip = clientIP(req);
  softLimit(`signup:${ip}`, 10, 3600);
  await checkLimit(env, `signup:${ip}`, 20);
  const b = await readJSON(req);
  const email = normEmail(b.email);
  if (!email) throw new HttpError(400, "invalid_email");
  if (!validPassword(b.password)) throw new HttpError(400, "invalid_password", { hint: "8 to 256 characters" });
  const name = cleanName(b.name);
  if (await userByEmail(env, email)) {
    await hit(env, `signup:${ip}`, 3600);
    throw new HttpError(409, "email_taken");
  }
  const u = await createUser(env, { email, name, passwordHash: await hashPassword(b.password, iterations(env)), verified: false });
  const token = await createSession(env, u, cleanName(b.client) || "Blaze");
  return json({ token, user: publicUser(u) }, 201);
}

async function login(req: Request, env: Env): Promise<Response> {
  const ip = clientIP(req);
  const b = await readJSON(req);
  const email = normEmail(b.email);
  if (!email || typeof b.password !== "string" || b.password.length > 256) throw new HttpError(400, "invalid_request");
  const ek = `login:email:${await sha256(email)}`;
  const ik = `login:ip:${ip}`;
  softLimit(ik, 30, 900);
  await checkLimit(env, ik, 20);
  await checkLimit(env, ek, 8);
  const u = await userByEmail(env, email);
  const ok = !!(u && u.password_hash && (await verifyPassword(b.password, u.password_hash)));
  if (!ok || !u) {
    await hit(env, ik, 900);
    await hit(env, ek, 900);
    throw new HttpError(401, "invalid_credentials");
  }
  const token = await signInOrMfa(env, u, b.client); // 401 mfa_required when 2FA is on
  return json({ token, user: publicUser(u) });
}

// ---------------- OAuth: start / callback / redeem / refresh ----------------

async function start(req: Request, env: Env, pid: string, flow: "login" | "connect"): Promise<Response> {
  const p = provider(pid);
  if (flow === "login" && !p.login) throw new HttpError(404, "login_not_supported");
  const creds = credentials(env, p.id);
  if (!creds) throw new HttpError(503, "not_configured", { provider: p.id });
  softLimit(`start:${clientIP(req)}`, 30, 600);
  const q = new URL(req.url).searchParams;
  const ds = q.get("state") || "";
  const cc = q.get("code_challenge") || "";
  const method = q.get("code_challenge_method") || "S256";
  const rd = loopbackRedirect(q.get("redirect")) || appRedirect(q.get("redirect"));
  if (!STATE_RE.test(ds)) throw new HttpError(400, "invalid_state");
  if (!CHALLENGE_RE.test(cc) || method !== "S256") throw new HttpError(400, "invalid_code_challenge");
  if (!rd) throw new HttpError(400, "invalid_redirect", { hint: "only http://127.0.0.1:<port>/..., http://[::1]:<port>/... or blaze://auth/cb" });
  const scopes = flow === "login" ? (p.login as string[]) : pickScopes(p, q.get("scope"));

  const state = randomToken(24);
  const rec: StateRecord = { flow, provider: p.id, redirect: rd, ds, cc, scopes, exp: now() + STATE_TTL };
  await env.KV.put(`oauthstate:${state}`, JSON.stringify(rec), { expirationTtl: STATE_TTL });

  const a = new URL(p.authUrl);
  a.searchParams.set("client_id", creds.id);
  a.searchParams.set("redirect_uri", callbackURL(env, p.id));
  a.searchParams.set("response_type", "code");
  a.searchParams.set("state", state);
  if (p.scopeParam && scopes.length) a.searchParams.set(p.scopeParam, scopes.join(p.scopeSep));
  for (const [k, v] of Object.entries(p.authParams || {})) a.searchParams.set(k, v);
  if (flow === "login" && p.id === "google") a.searchParams.set("prompt", "select_account");
  if (flow === "login") a.searchParams.delete("access_type");
  return redirect(a.toString());
}

function page(title: string, msg: string, status = 400): Response {
  return html(`<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Blaze</title><body style="font:15px/1.5 system-ui,sans-serif;background:#FBFBFA;color:#111110;display:grid;place-items:center;min-height:100vh;margin:0"><div style="max-width:440px;padding:32px;border:1px solid rgba(17,17,16,.18);background:#fff;box-shadow:inset 3px 0 0 #111110"><div style="font:500 11px monospace;letter-spacing:.12em;text-transform:uppercase">Blaze</div><h2 style="margin:8px 0 6px;font-weight:600">${esc(title)}</h2><p style="margin:0;color:rgba(17,17,16,.6)">${esc(msg)}</p></div></body>`, status);
}

function back(rec: StateRecord, params: Record<string, string>): Response {
  const u = new URL(rec.redirect);
  u.searchParams.set("state", rec.ds);
  for (const [k, v] of Object.entries(params)) u.searchParams.set(k, v);
  if (rec.redirect === "blaze://auth/cb") return appHandoff(u.toString());
  return redirect(u.toString());
}

async function callback(req: Request, env: Env, pid: string): Promise<Response> {
  const p = provider(pid);
  const q = new URL(req.url).searchParams;
  const state = q.get("state") || "";
  if (!/^[A-Za-z0-9_-]{20,64}$/.test(state)) return page("Sign-in didn't finish", "This link is missing its sign-in state. Start again from Blaze.");
  const key = `oauthstate:${state}`;
  const rec = await env.KV.get<StateRecord>(key, "json");
  if (!rec || rec.provider !== p.id || rec.exp < now()) return page("Sign-in expired", "That sign-in link was already used or took too long. Start again from Blaze.");
  await env.KV.delete(key); // single use
  if (rec.flow === "web") return webCallback(env, p.id, rec, q);
  const perr = q.get("error");
  if (perr) return back(rec, { error: perr.slice(0, 64), provider: p.id });
  const code = q.get("code");
  if (!code || code.length > 2048) return back(rec, { error: "missing_code", provider: p.id });
  const creds = credentials(env, p.id);
  if (!creds) return back(rec, { error: "not_configured", provider: p.id });

  let out: CodeRecord;
  try {
    const tokens = await tokenRequest(p, creds, { grant_type: "authorization_code", code, redirect_uri: callbackURL(env, p.id) });
    if (rec.flow === "login") {
      const prof = await fetchProfile(p.id, tokens.access_token);
      const u = await userForIdentity(env, p.id, prof);
      out = { kind: "login", provider: p.id, cc: rec.cc, uid: u.id, exp: now() + CODE_TTL };
    } else {
      out = { kind: "tokens", provider: p.id, cc: rec.cc, tokens, scopes: tokens.scope ? tokens.scope.split(/[\s,]+/).filter(Boolean) : rec.scopes, exp: now() + CODE_TTL };
    }
  } catch (e) {
    const c = e instanceof HttpError ? String(e.extra.provider_error || e.code) : "server_error";
    if (!(e instanceof HttpError)) console.error("callback", (e as Error)?.name || "Error");
    return back(rec, { error: c.slice(0, 64), provider: p.id });
  }
  const one = randomToken(32);
  await env.KV.put(`oauthcode:${await sha256(one)}`, JSON.stringify(out), { expirationTtl: CODE_TTL });
  return back(rec, { code: one, provider: p.id, flow: rec.flow });
}

async function redeem(req: Request, env: Env): Promise<Response> {
  softLimit(`redeem:${clientIP(req)}`, 60, 600);
  const b = await readJSON(req);
  const code = typeof b.code === "string" ? b.code : "";
  const ver = typeof b.code_verifier === "string" ? b.code_verifier : "";
  if (!CODE_RE.test(code)) throw new HttpError(400, "invalid_code");
  if (!VERIFIER_RE.test(ver)) throw new HttpError(400, "invalid_code_verifier");
  const key = `oauthcode:${await sha256(code)}`;
  const rec = await env.KV.get<CodeRecord>(key, "json");
  if (!rec || rec.exp < now()) throw new HttpError(400, "invalid_code");
  if (!safeEqual(await pkceChallenge(ver), rec.cc)) throw new HttpError(400, "invalid_code_verifier");
  await env.KV.delete(key);
  if (rec.kind === "login") {
    const u = rec.uid ? await getUser(env, rec.uid) : null;
    if (!u) throw new HttpError(400, "invalid_code");
    const token = await signInOrMfa(env, u, b.client);
    return json({ kind: "session", provider: rec.provider, token, user: publicUser(u) });
  }
  return json({ kind: "tokens", provider: rec.provider, tokens: rec.tokens, scopes: rec.scopes || [] });
}

async function refresh(req: Request, env: Env, pid: string): Promise<Response> {
  const p = provider(pid);
  const creds = credentials(env, p.id);
  if (!creds) throw new HttpError(503, "not_configured", { provider: p.id });
  softLimit(`refresh:${clientIP(req)}`, 120, 600);
  const b = await readJSON(req);
  const rt = typeof b.refresh_token === "string" ? b.refresh_token.trim() : "";
  if (!rt || rt.length > 4096) throw new HttpError(400, "invalid_refresh_token");
  const tokens = await tokenRequest(p, creds, { grant_type: "refresh_token", refresh_token: rt });
  return json({ kind: "tokens", provider: p.id, tokens });
}

// ---------------- web sign-in (0.9.0): Google / GitHub from the website ----------------
// GET /v1/auth/<p>/web?return=<https url on an ALLOWED_ORIGINS origin> -> provider -> the same callback
// -> 302 <return>#bz_oauth=<one-time code> (or #bz_oauth_error=<code>) -> POST /v1/auth/web/redeem.

async function startWeb(req: Request, env: Env, pid: string): Promise<Response> {
  const p = provider(pid);
  if (!p.login) throw new HttpError(404, "login_not_supported");
  const creds = credentials(env, p.id);
  if (!creds) throw new HttpError(503, "not_configured", { provider: p.id });
  softLimit(`start:${clientIP(req)}`, 30, 600);
  const ret = webReturn(env, new URL(req.url).searchParams.get("return"));
  if (!ret) throw new HttpError(400, "invalid_return", { hint: "an https URL on an allowed origin" });
  const state = randomToken(24);
  const rec: StateRecord = { flow: "web", provider: p.id, redirect: ret.toString(), ds: "", cc: "", scopes: p.login, exp: now() + STATE_TTL };
  await env.KV.put(`oauthstate:${state}`, JSON.stringify(rec), { expirationTtl: STATE_TTL });
  const a = new URL(p.authUrl);
  a.searchParams.set("client_id", creds.id);
  a.searchParams.set("redirect_uri", callbackURL(env, p.id));
  a.searchParams.set("response_type", "code");
  a.searchParams.set("state", state);
  if (p.scopeParam && p.login.length) a.searchParams.set(p.scopeParam, p.login.join(p.scopeSep));
  for (const [k, v] of Object.entries(p.authParams || {})) a.searchParams.set(k, v);
  if (p.id === "google") a.searchParams.set("prompt", "select_account");
  a.searchParams.delete("access_type");
  return redirect(a.toString());
}

function webBack(rec: StateRecord, frag: string): Response {
  const u = new URL(rec.redirect);
  u.hash = frag;
  return redirect(u.toString());
}

async function webCallback(env: Env, pid: ProviderId, rec: StateRecord, q: URLSearchParams): Promise<Response> {
  const p = provider(pid);
  const fail = (c: string) => webBack(rec, `bz_oauth_error=${encodeURIComponent(c.replace(/[^A-Za-z0-9_.-]/g, "_").slice(0, 64))}`);
  const perr = q.get("error");
  if (perr) return fail(perr);
  const code = q.get("code");
  if (!code || code.length > 2048) return fail("missing_code");
  const creds = credentials(env, p.id);
  if (!creds) return fail("not_configured");
  let uid: string;
  try {
    const tokens = await tokenRequest(p, creds, { grant_type: "authorization_code", code, redirect_uri: callbackURL(env, p.id) });
    const prof = await fetchProfile(p.id, tokens.access_token);
    uid = (await userForIdentity(env, p.id, prof)).id; // same account as app sign-in
  } catch (e) {
    const c = e instanceof HttpError ? String(e.extra.provider_error || e.code) : "server_error";
    if (!(e instanceof HttpError)) console.error("web callback", (e as Error)?.name || "Error");
    return fail(c);
  }
  const one = randomToken(32);
  const out: WebCodeRecord = { uid, provider: p.id, origin: new URL(rec.redirect).origin, exp: now() + CODE_TTL };
  await env.KV.put(`webcode:${await sha256(one)}`, JSON.stringify(out), { expirationTtl: CODE_TTL });
  return webBack(rec, `bz_oauth=${one}`);
}

async function webRedeem(req: Request, env: Env): Promise<Response> {
  softLimit(`redeem:${clientIP(req)}`, 60, 600);
  const b = await readJSON(req);
  const code = typeof b.code === "string" ? b.code : "";
  if (!CODE_RE.test(code)) throw new HttpError(400, "invalid_code");
  const key = `webcode:${await sha256(code)}`;
  const rec = await env.KV.get<WebCodeRecord>(key, "json");
  if (!rec || rec.exp < now()) throw new HttpError(400, "invalid_code");
  const origin = req.headers.get("Origin");
  if (origin && origin !== rec.origin) throw new HttpError(400, "invalid_code");
  await env.KV.delete(key); // single use
  const u = await getUser(env, rec.uid);
  if (!u) throw new HttpError(400, "invalid_code");
  const token = await signInOrMfa(env, u, b.client, "Blaze web");
  return json({ token, user: publicUser(u) });
}
