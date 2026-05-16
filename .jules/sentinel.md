## 2026-04-25 - [Missing Authentication on API Routes]
**Vulnerability:** The API subrouter for administrative actions (`/api/action/*`), log streaming (`/api/logs`), and update checks (`/api/check-update`) did not have authentication enforced, allowing any user to start/stop the server or read logs.
**Learning:** The web routes `/login` and `/logout` had an auth check conceptually, but the `/api` subrouter was completely exposed. This is a severe architectural gap where API requests bypassed the security layer implemented for web access.
**Prevention:** Always apply authentication middleware at the router level for sensitive subrouters instead of relying on frontend-only protection or per-handler checks. Ensure health check endpoints like `/api/status` are kept out of the authenticated subrouter to avoid breaking liveness probes.

## 2026-05-01 - [Plaintext Password in Session Cookie]
**Vulnerability:** The admin password was being stored directly as plaintext in the `soulmask_session` cookie and used to verify authentication on subsequent requests using simple string comparison (`==`). This exposes the password to the client and potentially any man-in-the-middle or XSS attacks, and opens up the authentication to timing attacks.
**Learning:** This is a severe architectural gap where the application failed to implement basic secure session handling, relying instead on a shared secret embedded on the client side.
**Prevention:** Never store passwords or sensitive secrets in cookies or client-side storage in plaintext. Always use a securely generated random session token for authentication, and use `crypto/subtle.ConstantTimeCompare` when comparing secrets to mitigate timing attacks.

## 2026-05-16 - [Missing Rate Limiting on Authentication Endpoint]
**Vulnerability:** The `/login` endpoint lacked any form of rate limiting. This allowed unlimited login attempts per IP address, leaving the application vulnerable to brute force and credential stuffing attacks.
**Learning:** Even with an internal tool or dashboard, exposing an authentication endpoint without rate limits provides attackers with a trivial way to systematically guess passwords or overwhelm the server (DoS). The middleware relies on accurate IP extraction (e.g. `X-Forwarded-For` or `CF-Connecting-IP` via `TrustProxy`) which is vital for correct rate limiting.
**Prevention:** Always apply rate limiting middleware to sensitive endpoints, especially authentication, password resets, and user creation. Ensure rate limiters map correctly to the true client IP to prevent spoofing bypasses.
