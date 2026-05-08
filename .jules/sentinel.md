## 2026-04-25 - [Missing Authentication on API Routes]
**Vulnerability:** The API subrouter for administrative actions (`/api/action/*`), log streaming (`/api/logs`), and update checks (`/api/check-update`) did not have authentication enforced, allowing any user to start/stop the server or read logs.
**Learning:** The web routes `/login` and `/logout` had an auth check conceptually, but the `/api` subrouter was completely exposed. This is a severe architectural gap where API requests bypassed the security layer implemented for web access.
**Prevention:** Always apply authentication middleware at the router level for sensitive subrouters instead of relying on frontend-only protection or per-handler checks. Ensure health check endpoints like `/api/status` are kept out of the authenticated subrouter to avoid breaking liveness probes.

## 2026-05-01 - [Plaintext Password in Session Cookie]
**Vulnerability:** The admin password was being stored directly as plaintext in the `soulmask_session` cookie and used to verify authentication on subsequent requests using simple string comparison (`==`). This exposes the password to the client and potentially any man-in-the-middle or XSS attacks, and opens up the authentication to timing attacks.
**Learning:** This is a severe architectural gap where the application failed to implement basic secure session handling, relying instead on a shared secret embedded on the client side.
**Prevention:** Never store passwords or sensitive secrets in cookies or client-side storage in plaintext. Always use a securely generated random session token for authentication, and use `crypto/subtle.ConstantTimeCompare` when comparing secrets to mitigate timing attacks.

## 2026-05-08 - [Missing Rate Limiting on Login / Proxy IP Issue]
**Vulnerability:** The `/login` endpoint lacked rate limiting, making it vulnerable to brute-force password guessing. Additionally, `IPMiddleware` was setting a dummy port `0` when rewriting `r.RemoteAddr` for proxy requests. If a rate limiter used `r.RemoteAddr` directly without stripping the port, it would treat every request from a different source port as a new IP, effectively bypassing the limit.
**Learning:** This highlights a critical intersection between middleware behavior and security controls. Security features that rely on IP addresses must carefully handle how IPs are resolved, especially in environments utilizing reverse proxies.
**Prevention:** Always implement rate limiting on sensitive endpoints like login. When using `r.RemoteAddr` for IP-based tracking, always use `net.SplitHostPort` to extract just the IP address, regardless of whether a proxy is in use.
