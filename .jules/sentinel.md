## 2026-04-25 - [Missing Authentication on API Routes]
**Vulnerability:** The API subrouter for administrative actions (`/api/action/*`), log streaming (`/api/logs`), and update checks (`/api/check-update`) did not have authentication enforced, allowing any user to start/stop the server or read logs.
**Learning:** The web routes `/login` and `/logout` had an auth check conceptually, but the `/api` subrouter was completely exposed. This is a severe architectural gap where API requests bypassed the security layer implemented for web access.
**Prevention:** Always apply authentication middleware at the router level for sensitive subrouters instead of relying on frontend-only protection or per-handler checks. Ensure health check endpoints like `/api/status` are kept out of the authenticated subrouter to avoid breaking liveness probes.

## 2026-05-01 - [Plaintext Password in Session Cookie]
**Vulnerability:** The admin password was being stored directly as plaintext in the `soulmask_session` cookie and used to verify authentication on subsequent requests using simple string comparison (`==`). This exposes the password to the client and potentially any man-in-the-middle or XSS attacks, and opens up the authentication to timing attacks.
**Learning:** This is a severe architectural gap where the application failed to implement basic secure session handling, relying instead on a shared secret embedded on the client side.
**Prevention:** Never store passwords or sensitive secrets in cookies or client-side storage in plaintext. Always use a securely generated random session token for authentication, and use `crypto/subtle.ConstantTimeCompare` when comparing secrets to mitigate timing attacks.

## 2026-05-11 - [Missing Brute-Force Protection on Login Endpoint]
**Vulnerability:** The `/login` endpoint did not have any rate limiting mechanism, allowing attackers to continuously submit password guesses. This is especially risky since the system uses a single admin password for authentication.
**Learning:** For systems reliant on a single shared secret (admin password), the lack of rate limiting is a high-severity risk as brute force attempts are straightforward. Simple time-based constraints per IP address can significantly mitigate this. Furthermore, in-memory rate limiting requires cache eviction logic (e.g. background cleanup goroutine) to prevent memory exhaustion DoS vectors over time.
**Prevention:** Implement IP-based rate limiting on sensitive endpoints such as `/login`. Since IPs might be rewritten by reverse proxies, ensure you use the correct originating IP by properly parsing `r.RemoteAddr` (e.g. accounting for proxy middleware). Always include cleanup logic for maps used to track connections.
