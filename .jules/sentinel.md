## 2026-04-25 - [Missing Authentication on API Routes]
**Vulnerability:** The API subrouter for administrative actions (`/api/action/*`), log streaming (`/api/logs`), and update checks (`/api/check-update`) did not have authentication enforced, allowing any user to start/stop the server or read logs.
**Learning:** The web routes `/login` and `/logout` had an auth check conceptually, but the `/api` subrouter was completely exposed. This is a severe architectural gap where API requests bypassed the security layer implemented for web access.
**Prevention:** Always apply authentication middleware at the router level for sensitive subrouters instead of relying on frontend-only protection or per-handler checks. Ensure health check endpoints like `/api/status` are kept out of the authenticated subrouter to avoid breaking liveness probes.

## 2026-05-01 - [Plaintext Password in Session Cookie]
**Vulnerability:** The admin password was being stored directly as plaintext in the `soulmask_session` cookie and used to verify authentication on subsequent requests using simple string comparison (`==`). This exposes the password to the client and potentially any man-in-the-middle or XSS attacks, and opens up the authentication to timing attacks.
**Learning:** This is a severe architectural gap where the application failed to implement basic secure session handling, relying instead on a shared secret embedded on the client side.
**Prevention:** Never store passwords or sensitive secrets in cookies or client-side storage in plaintext. Always use a securely generated random session token for authentication, and use `crypto/subtle.ConstantTimeCompare` when comparing secrets to mitigate timing attacks.

## 2026-05-09 - [Missing Authentication on /api/status]
**Vulnerability:** The `/api/status` endpoint was mounted directly on `apiRouter` rather than `authApiRouter`. This means it lacked the `AuthMiddleware` protection, exposing sensitive container information (e.g., container ID, image versions, CPU/Memory stats) to unauthenticated users.
**Learning:** Router scoping caused an unauthenticated API endpoint. While it was intended to be public in a previous fix for liveness probes, it leaked sensitive stats and container IDs, which is a significant information disclosure vulnerability for a control panel application.
**Prevention:** Ensure that health check endpoints that are public only return safe, generic liveness indicators. Endpoints that return detailed system and container information must be secured behind authentication middleware.
