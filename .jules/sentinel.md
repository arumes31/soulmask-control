## 2026-04-25 - [Missing Authentication on API Routes]
**Vulnerability:** The API subrouter for administrative actions (`/api/action/*`), log streaming (`/api/logs`), and update checks (`/api/check-update`) did not have authentication enforced, allowing any user to start/stop the server or read logs.
**Learning:** The web routes `/login` and `/logout` had an auth check conceptually, but the `/api` subrouter was completely exposed. This is a severe architectural gap where API requests bypassed the security layer implemented for web access.
**Prevention:** Always apply authentication middleware at the router level for sensitive subrouters instead of relying on frontend-only protection or per-handler checks. Ensure health check endpoints like `/api/status` are kept out of the authenticated subrouter to avoid breaking liveness probes.

## 2026-04-28 - [Plaintext Credentials in Session Cookie]
**Vulnerability:** The session cookie value was storing the plaintext administration password. If a user was authenticated and their cookie was intercepted or leaked, the attacker would have the plaintext password.
**Learning:** Never store raw passwords or sensitive credentials directly in cookies or local storage, even if HttpOnly and Secure flags are set.
**Prevention:** Generate a random session token (e.g., using `crypto/rand` to create a hex string) upon server start or user login, and use this opaque token as the cookie value to validate authenticated requests. Also ensure `SameSite=StrictMode` is used instead of `LaxMode` for session cookies where applicable to improve CSRF protection.
