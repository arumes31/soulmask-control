## 2026-04-25 - [Missing Authentication on API Routes]
**Vulnerability:** The API subrouter for administrative actions (`/api/action/*`), log streaming (`/api/logs`), and update checks (`/api/check-update`) did not have authentication enforced, allowing any user to start/stop the server or read logs.
**Learning:** The web routes `/login` and `/logout` had an auth check conceptually, but the `/api` subrouter was completely exposed. This is a severe architectural gap where API requests bypassed the security layer implemented for web access.
**Prevention:** Always apply authentication middleware at the router level for sensitive subrouters instead of relying on frontend-only protection or per-handler checks. Ensure health check endpoints like `/api/status` are kept out of the authenticated subrouter to avoid breaking liveness probes.

## 2026-05-01 - [Plaintext Password in Session Cookie]
**Vulnerability:** The admin password was being stored directly as plaintext in the `soulmask_session` cookie and used to verify authentication on subsequent requests using simple string comparison (`==`). This exposes the password to the client and potentially any man-in-the-middle or XSS attacks, and opens up the authentication to timing attacks.
**Learning:** This is a severe architectural gap where the application failed to implement basic secure session handling, relying instead on a shared secret embedded on the client side.
**Prevention:** Never store passwords or sensitive secrets in cookies or client-side storage in plaintext. Always use a securely generated random session token for authentication, and use `crypto/subtle.ConstantTimeCompare` when comparing secrets to mitigate timing attacks.

## 2026-05-04 - [Missing Rate Limiting on Login]
**Vulnerability:** The `/login` endpoint did not have rate limiting, making it vulnerable to brute-force attacks against the admin password.
**Learning:** Adding rate limiters based on connection properties must respect proxy configuration. If `TrustProxy` is enabled but rate-limiting logic directly uses `RemoteAddr`, all clients behind the proxy are subjected to a shared limit, which creates a denial of service (DoS) vulnerability under normal load. Furthermore, rate limiter maps in long-running services without a cleanup mechanism can cause unbound memory growth (a different type of DoS).
**Prevention:** 1. Ensure security rules evaluate the correct client IP, incorporating headers like `X-Forwarded-For` or `CF-Connecting-IP` when trusting a reverse proxy. 2. Implement a background cleanup routine for memory-bound data structures like per-IP rate limiters to remove stale entries over time.
