# blaze-auth changelog

## 0.9.0 (prepared 2026-10-20, NOT deployed; live is 0.5.0, so 0.6.0-0.8.0 ship with it)

- **Web sign-in with Google and GitHub.** `GET /v1/auth/{google|github}/web?return=<url>` (`return` must be https on
  an `ALLOWED_ORIGINS` origin) reuses the existing callback (`BASE_URL/oauth/<p>/callback`, state `flow:"web"`), finds
  or creates the account like app sign-in, and redirects to `<return>#bz_oauth=<one-time code>` or
  `#bz_oauth_error=<code>`. `POST /v1/auth/web/redeem {code, client}` -> `{token, user}`; codes are single use, 120 s,
  and only redeem from the origin that started them.
- **Password reset / set.** `POST /v1/auth/password/forgot {email, return?}` always answers `200 {ok:true}`; limited per
  IP and per address; mails a 1-hour single-use link (`<return origin or https://blazeinchue.pages.dev>/?reset=<token>#reset`)
  through blaze-gateway (`GATEWAY` service binding, `POST https://gateway/internal/mail`, `X-Internal-Token` =
  secret `INTERNAL_TOKEN`). `POST /v1/auth/password/reset {token, password}` sets the password, marks the email
  verified and ends every session. `POST /v1/auth/password/set` (Bearer) `{password, current_password?}`; changing an
  existing password signs out the other devices.
- **Two-factor (TOTP).** `POST /v1/auth/2fa/setup|enable|disable` (Bearer) and `POST /v1/auth/2fa/verify
  {mfa_token, code}`. 10 recovery codes, stored hashed, each single use. Password login, web redeem and app OAuth
  redeem answer `401 {error:"mfa_required", mfa_token}` when 2FA is on (5 min, single use, 5 tries).
- A verified Google/GitHub email taking over an unverified password account, or a reset on an unverified account,
  also drops any 2FA on it (it can't lock the real owner out).
- `user` adds `two_factor` (`has_password` was already there). `/v1/providers` adds `web_login: true`,
  `password_reset` (true when `GATEWAY` and `INTERNAL_TOKEN` are both set) and `two_factor: true`.
- `wrangler.toml`: `[[services]] GATEWAY -> blaze-gateway`; `ALLOWED_ORIGINS` = Netlify + Pages site; secret
  `INTERNAL_TOKEN` documented. CORS methods unchanged (GET, POST, OPTIONS). No new KV namespace, no D1.
- Needs blaze-gateway 0.6.3+ (`POST /internal/mail`) for reset mail.

## Earlier

- 0.6.0 `/v1/keys` model-key sync; `/v1/comments` pinned comments for shipped pages; 0.8.0 `/v1/skills` skill sync and
  the GitHub `workflow` scope. See the git history of `cloud/` and the README.
