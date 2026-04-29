## 2026-04-25 - [Missing Authentication on API Routes]
**Vulnerability:** The API subrouter for administrative actions (`/api/action/*`), log streaming (`/api/logs`), and update checks (`/api/check-update`) did not have authentication enforced, allowing any user to start/stop the server or read logs.
**Learning:** The web routes `/login` and `/logout` had an auth check conceptually, but the `/api` subrouter was completely exposed. This is a severe architectural gap where API requests bypassed the security layer implemented for web access.
**Prevention:** Always apply authentication middleware at the router level for sensitive subrouters instead of relying on frontend-only protection or per-handler checks. Ensure health check endpoints like `/api/status` are kept out of the authenticated subrouter to avoid breaking liveness probes.

## 2025-02-27 - Plaintext Password Leakage in Session Cookies
**Vulnerability:** The application was previously storing the plaintext admin password in the `soulmask_session` cookie for authentication. This exposed the admin password to any network traffic that wasn't secured or if cookies were somehow compromised.
**Learning:** Returning a plain text password as part of the session token or cookie allows attackers to get access easily if the cookie value is intercepted, and exposes the exact password the user selected. We must never store or expose plaintext passwords.
**Prevention:** Created a random cryptographically secure token on authentication setup and used this token as the cookie value for authenticated sessions, removing the plaintext password from the cookie entirely.
