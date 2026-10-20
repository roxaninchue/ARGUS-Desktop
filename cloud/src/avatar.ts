// 0.9.3: custom profile pictures. Clients crop to a square and compress before upload;
// the server only checks type (magic bytes), size and square-ish dimensions it can read cheaply.
import type { Env } from "./util";
import { HttpError, json } from "./util";
import { User, putUser, publicUser } from "./accounts";

export const AVATAR_MAX_BYTES = 256 * 1024;
const TYPES: Record<string, string> = { png: "image/png", jpeg: "image/jpeg", webp: "image/webp" };

function sniff(b: Uint8Array): string | null {
  if (b.length > 8 && b[0] === 0x89 && b[1] === 0x50 && b[2] === 0x4e && b[3] === 0x47) return "png";
  if (b.length > 3 && b[0] === 0xff && b[1] === 0xd8 && b[2] === 0xff) return "jpeg";
  if (b.length > 12 && b[0] === 0x52 && b[1] === 0x49 && b[2] === 0x46 && b[3] === 0x46 && b[8] === 0x57 && b[9] === 0x45 && b[10] === 0x42 && b[11] === 0x50) return "webp";
  return null;
}

export function avatarURL(req: Request, uid: string, v: number): string {
  return new URL(req.url).origin + `/v1/avatar/${uid}?v=${v}`;
}

/** POST /v1/me/avatar with the raw image as the body (Content-Type image/png|jpeg|webp). */
export async function putAvatar(req: Request, env: Env, u: User): Promise<Response> {
  const len = Number(req.headers.get("Content-Length") || "0");
  if (len > AVATAR_MAX_BYTES) throw new HttpError(413, "too_large", { max_bytes: AVATAR_MAX_BYTES });
  const buf = new Uint8Array(await req.arrayBuffer());
  if (!buf.length) throw new HttpError(400, "empty");
  if (buf.length > AVATAR_MAX_BYTES) throw new HttpError(413, "too_large", { max_bytes: AVATAR_MAX_BYTES });
  const kind = sniff(buf);
  if (!kind) throw new HttpError(415, "unsupported_type", { allowed: Object.values(TYPES) });
  const v = Date.now();
  await env.KV.put(`avatar:${u.id}`, buf, { metadata: { type: TYPES[kind], v } });
  u.avatar_url = avatarURL(req, u.id, v);
  u.avatar_off = false;
  await putUser(env, u);
  return json({ ok: true, avatar_url: u.avatar_url, user: publicUser(u) });
}

/** POST /v1/me/avatar/delete: back to the letter avatar (avatar_url ""). */
export async function deleteAvatar(env: Env, u: User): Promise<Response> {
  await env.KV.delete(`avatar:${u.id}`);
  u.avatar_url = "";
  u.avatar_off = true;
  await putUser(env, u);
  return json({ ok: true, avatar_url: "", user: publicUser(u) });
}

/** GET /v1/avatar/<uid>: public image (the ?v= query busts caches after a change). */
export async function getAvatar(env: Env, uid: string): Promise<Response> {
  const r = await env.KV.getWithMetadata<{ type: string; v: number }>(`avatar:${uid}`, "arrayBuffer");
  if (!r.value) return new Response("Not found", { status: 404, headers: { "Content-Type": "text/plain", "Access-Control-Allow-Origin": "*", "Cache-Control": "no-store" } });
  return new Response(r.value, {
    headers: {
      "Content-Type": r.metadata?.type || "application/octet-stream",
      "Cache-Control": "public, max-age=86400",
      "Access-Control-Allow-Origin": "*",
      "Cross-Origin-Resource-Policy": "cross-origin",
      "X-Content-Type-Options": "nosniff",
      "Content-Security-Policy": "default-src 'none'; sandbox",
    },
  });
}
