## 2026-05-19 - [Hardcoded Default Admin Password]
**Vulnerability:** The application used a hardcoded default password (`admin`) if the `ADMIN_PASSWORD` environment variable was not set. This could allow attackers to gain unauthorized access to the control panel if administrators failed to explicitly configure a strong password before deployment.
**Learning:** Default credentials are a common security pitfall. Attackers often scan for exposed applications using default credentials. It is crucial to ensure that systems fall back to a secure state (e.g., auto-generated random passwords or refusing to start) rather than a known insecure state.
**Prevention:** Never use easily guessable or hardcoded default passwords. If a password must be provided, either force the user to set it or generate a strong, random password automatically on startup and log it securely for the administrator to retrieve.

## 2026-04-25 - [Missing Authentication on API Routes]
**Vulnerability:** The API subrouter for administrative actions (`/api/action/*`), log streaming (`/api/logs`), and update checks (`/api/check-update`) did not have authentication enforced, allowing any user to start/stop the server or read logs.
**Learning:** The web routes `/login` and `/logout` had an auth check conceptually, but the `/api` subrouter was completely exposed. This is a severe architectural gap where API requests bypassed the security layer implemented for web access.
**Prevention:** Always apply authentication middleware at the router level for sensitive subrouters instead of relying on frontend-only protection or per-handler checks. Ensure health check endpoints like `/api/status` are kept out of the authenticated subrouter to avoid breaking liveness probes.

## 2026-05-01 - [Plaintext Password in Session Cookie]
**Vulnerability:** The admin password was being stored directly as plaintext in the `soulmask_session` cookie and used to verify authentication on subsequent requests using simple string comparison (`==`). This exposes the password to the client and potentially any man-in-the-middle or XSS attacks, and opens up the authentication to timing attacks.
**Learning:** This is a severe architectural gap where the application failed to implement basic secure session handling, relying instead on a shared secret embedded on the client side.
**Prevention:** Never store passwords or sensitive secrets in cookies or client-side storage in plaintext. Always use a securely generated random session token for authentication, and use `crypto/subtle.ConstantTimeCompare` when comparing secrets to mitigate timing attacks.
