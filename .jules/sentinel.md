## 2026-04-25 - [Missing Authentication on API Routes]
**Vulnerability:** The API subrouter for administrative actions (`/api/action/*`), log streaming (`/api/logs`), and update checks (`/api/check-update`) did not have authentication enforced, allowing any user to start/stop the server or read logs.
**Learning:** The web routes `/login` and `/logout` had an auth check conceptually, but the `/api` subrouter was completely exposed. This is a severe architectural gap where API requests bypassed the security layer implemented for web access.
**Prevention:** Always apply authentication middleware at the router level for sensitive subrouters instead of relying on frontend-only protection or per-handler checks. Ensure health check endpoints like `/api/status` are kept out of the authenticated subrouter to avoid breaking liveness probes.

## 2026-05-01 - [Plaintext Password in Session Cookie]
**Vulnerability:** The admin password was being stored directly as plaintext in the `soulmask_session` cookie and used to verify authentication on subsequent requests using simple string comparison (`==`). This exposes the password to the client and potentially any man-in-the-middle or XSS attacks, and opens up the authentication to timing attacks.
**Learning:** This is a severe architectural gap where the application failed to implement basic secure session handling, relying instead on a shared secret embedded on the client side.
**Prevention:** Never store passwords or sensitive secrets in cookies or client-side storage in plaintext. Always use a securely generated random session token for authentication, and use `crypto/subtle.ConstantTimeCompare` when comparing secrets to mitigate timing attacks.

## 2026-05-25 - [Hardcoded Default Administrator Password]
**Vulnerability:** The application used a weak, easily guessable default password ("admin") for administrative access if the `ADMIN_PASSWORD` environment variable was not provided by the user.
**Learning:** Hardcoded default passwords on administrative or root interfaces are a critical security gap that enables trivial unauthorized access when instances are deployed with default configurations. Relying on users to securely configure applications is not an effective defense mechanism for core administrative functions.
**Prevention:** Always follow a secure-by-default philosophy. If an explicit secret is not provided, either refuse to start or securely auto-generate a strong, random password on startup and display it to the user.
