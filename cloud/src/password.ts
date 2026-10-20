// Password reset / set and two-factor management (0.9.0).
//   POST /v1/auth/password/forgot  {email, return?}            -> 200 {ok:true} always (no account enumeration)
//   POST /v1/auth/password/reset   {token, password}           -> {ok:true}; ends every session
//   POST /v1/auth/password/set     Bearer {password, current_password?}
//   POST /v1/auth/2fa/setup        Bearer -> {secret, otpauth_uri}
//   POST /v1/auth/2fa/enable       Bearer {code} -> {recovery_codes}
//   POST /v1/auth/2fa/disable      Bearer {code}  (TOTP or recovery code)
//   POST /v1/auth/2fa/verify       {mfa_token, code} -> {token, user}
// Reset mail goes through blaze-gateway (service binding GATEWAY, POST https://gateway/internal/mail with
// X-Internal-Token = INTERNAL_TOKEN); without both, password_reset is off and /forgot answers 503.

import {
  Env, HttpError, json, readJSON, clientIP, randomToken, sha256, hashPassword, verifyPassword, iterations,
  normEmail, validPassword, cleanName, softLimit, webReturn, CODE_RE,
} from "./util";
import { User, getUser, putUser, userByEmail, requireUser, createSession, revokeAllSessions, publicUser, checkLimit, hit } from "./accounts";
import { MfaRecord, MFA_TRIES, consumeSecondFactor, matchTotp, newRecoveryCodes, newTotpSecret, otpauthURI, requireSecondFactor } from "./mfa";

export const RESET_TTL = 3600;
export const DEFAULT_SITE = "https://blazeinchue.pages.dev";
const now = () => Math.floor(Date.now() / 1000);

export function mailerConfigured(env: Env): boolean {
  return !!(env.GATEWAY && env.INTERNAL_TOKEN);
}

/** Sends one message through blaze-gateway. Never throws; false on any failure. */
export async function sendMail(env: Env, to: string, subject: string, text: string): Promise<boolean> {
  if (!env.GATEWAY || !env.INTERNAL_TOKEN) return false;
  try {
    const r = await env.GATEWAY.fetch("https://gateway/internal/mail", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Internal-Token": env.INTERNAL_TOKEN },
      body: JSON.stringify({ to, subject, text }),
    });
    if (!r.ok) console.error("mail failed", r.status);
    return r.ok;
  } catch (e) {
    console.error("mail error", (e as Error)?.name || "Error");
    return false;
  }
}

interface ResetRecord { uid: string; exp: number }

export async function forgot(req: Request, env: Env): Promise<Response> {
  softLimit(`forgot:ip:${clientIP(req)}`, 10, 3600);
  const b = await readJSON(req);
  const email = normEmail(b.email);
  if (!email) throw new HttpError(400, "invalid_email");
  if (!mailerConfigured(env)) throw new HttpError(503, "not_configured");
  const site = webReturn(env, typeof b.return === "string" ? b.return : null)?.origin || DEFAULT_SITE;
  const u = await userByEmail(env, email);
  // Same answer whether or not the account exists; a full per-address quota (5 mails an hour) just stops sending.
  if (!u || u.email.endsWith(".invalid")) return json({ ok: true });
  const ek = `forgot:email:${await sha256(email)}`;
  try {
    await checkLimit(env, ek, 5);
  } catch {
    return json({ ok: true });
  }
  const token = randomToken(32);
  const rec: ResetRecord = { uid: u.id, exp: now() + RESET_TTL };
  await env.KV.put(`reset:${await sha256(token)}`, JSON.stringify(rec), { expirationTtl: RESET_TTL });
  await hit(env, ek, 3600);
  const link = `${site}/?reset=${token}#reset`;
  const text = [
    "Set a new password for your Blaze account",
    `Open this link within 1 hour to choose a new password:\n${link}`,
    "If you signed up with Google or GitHub, this lets you add a password, so you can also sign in with your email.",
    "If you didn't ask for this, ignore this email. Nothing changes until the link is used.",
  ].join("\n\n");
  await sendMail(env, u.email, "Set a new password for your Blaze account", text);
  return json({ ok: true });
}

export async function reset(req: Request, env: Env): Promise<Response> {
  softLimit(`reset:${clientIP(req)}`, 20, 600);
  const b = await readJSON(req);
  const token = typeof b.token === "string" ? b.token : "";
  if (!CODE_RE.test(token)) throw new HttpError(400, "invalid_token");
  if (!validPassword(b.password)) throw new HttpError(400, "invalid_password", { hint: "8 to 256 characters" });
  const key = `reset:${await sha256(token)}`;
  const rec = await env.KV.get<ResetRecord>(key, "json");
  if (!rec || rec.exp < now()) throw new HttpError(400, "invalid_token");
  await env.KV.delete(key); // single use
  const u = await getUser(env, rec.uid);
  if (!u) throw new HttpError(400, "invalid_token");
  if (!u.email_verified) {
    // Whoever made an unverified account never proved the address; their 2FA must not lock the owner out.
    delete u.totp_secret;
    delete u.totp_pending;
    delete u.totp_recovery;
    delete u.totp_last;
  }
  u.password_hash = await hashPassword(b.password, iterations(env));
  u.email_verified = true; // the link reached the inbox
  await revokeAllSessions(env, u);
  await putUser(env, u);
  return json({ ok: true });
}

export async function setPassword(req: Request, env: Env): Promise<Response> {
  const { user: u, hash } = await requireUser(env, req);
  const b = await readJSON(req);
  if (!validPassword(b.password)) throw new HttpError(400, "invalid_password", { hint: "8 to 256 characters" });
  if (u.password_hash) {
    const k = `pwset:${u.id}`;
    await checkLimit(env, k, 8);
    const cur = typeof b.current_password === "string" && b.current_password.length <= 256 ? b.current_password : "";
    if (!cur) throw new HttpError(400, "current_password_required");
    if (!(await verifyPassword(cur, u.password_hash))) {
      await hit(env, k, 900);
      throw new HttpError(401, "invalid_credentials");
    }
    // Changing a password signs out every other device; this one stays.
    for (const h of u.sessions) if (h !== hash) await env.KV.delete(`sess:${h}`);
    u.sessions = u.sessions.filter(h => h === hash);
  }
  u.password_hash = await hashPassword(b.password, iterations(env));
  await putUser(env, u);
  return json({ ok: true, user: publicUser(u) });
}

// ---------------- 2FA ----------------

export async function mfaSetup(req: Request, env: Env): Promise<Response> {
  const { user: u } = await requireUser(env, req);
  if (u.totp_secret) throw new HttpError(409, "already_enabled");
  softLimit(`2fa:setup:${u.id}`, 10, 600);
  u.totp_pending = newTotpSecret();
  await putUser(env, u);
  return json({ secret: u.totp_pending, otpauth_uri: otpauthURI(u.totp_pending, u.email) });
}

export async function mfaEnable(req: Request, env: Env): Promise<Response> {
  const { user: u } = await requireUser(env, req);
  if (u.totp_secret) throw new HttpError(409, "already_enabled");
  if (!u.totp_pending) throw new HttpError(400, "setup_required");
  softLimit(`2fa:code:${u.id}`, 10, 600);
  const b = await readJSON(req);
  const code = typeof b.code === "string" ? b.code.replace(/\s/g, "") : "";
  const step = await matchTotp(u.totp_pending, code);
  if (step === null) throw new HttpError(400, "invalid_code");
  const { codes, hashes } = await newRecoveryCodes();
  u.totp_secret = u.totp_pending;
  delete u.totp_pending;
  u.totp_recovery = hashes;
  u.totp_last = step;
  await putUser(env, u);
  return json({ recovery_codes: codes, user: publicUser(u) });
}

export async function mfaDisable(req: Request, env: Env): Promise<Response> {
  const { user: u } = await requireUser(env, req);
  if (!u.totp_secret) throw new HttpError(400, "not_enabled");
  softLimit(`2fa:code:${u.id}`, 10, 600);
  const b = await readJSON(req);
  if (!(await consumeSecondFactor(u, b.code))) throw new HttpError(400, "invalid_code");
  delete u.totp_secret;
  delete u.totp_pending;
  delete u.totp_recovery;
  delete u.totp_last;
  await putUser(env, u);
  return json({ ok: true, user: publicUser(u) });
}

export async function mfaVerify(req: Request, env: Env): Promise<Response> {
  softLimit(`2fa:verify:${clientIP(req)}`, 30, 600);
  const b = await readJSON(req);
  const tok = typeof b.mfa_token === "string" ? b.mfa_token : "";
  if (!/^mfa_[A-Za-z0-9_-]{20,100}$/.test(tok)) throw new HttpError(401, "invalid_mfa_token");
  const key = `mfa:${await sha256(tok)}`;
  const rec = await env.KV.get<MfaRecord>(key, "json");
  if (!rec || rec.exp < now()) throw new HttpError(401, "invalid_mfa_token");
  const u = await getUser(env, rec.uid);
  if (!u) {
    await env.KV.delete(key);
    throw new HttpError(401, "invalid_mfa_token");
  }
  if (!u.totp_secret) {
    // 2FA was turned off meanwhile: the first factor already passed.
    await env.KV.delete(key);
    return json({ token: await createSession(env, u, rec.client), user: publicUser(u) });
  }
  if (!(await consumeSecondFactor(u, b.code))) {
    rec.tries++;
    if (rec.tries >= MFA_TRIES) {
      await env.KV.delete(key);
      throw new HttpError(401, "invalid_mfa_token", { reason: "too_many_tries" });
    }
    await env.KV.put(key, JSON.stringify(rec), { expirationTtl: Math.max(60, rec.exp - now()) });
    throw new HttpError(401, "invalid_code", { tries_left: MFA_TRIES - rec.tries });
  }
  await env.KV.delete(key); // single use
  const token = await createSession(env, u, rec.client); // saves u (consumed step / recovery code) too
  return json({ token, user: publicUser(u) });
}

/** Ends a first-factor sign-in: a session, or 401 mfa_required when 2FA is on. */
export async function signInOrMfa(env: Env, u: User, client: unknown, fallback = "Blaze"): Promise<string> {
  const c = cleanName(client) || fallback;
  if (u.totp_secret) await requireSecondFactor(env, u, c);
  return createSession(env, u, c);
}
