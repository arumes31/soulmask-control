## 2026-04-25 - [Missing Authentication on API Routes]
**Vulnerability:** The API subrouter for administrative actions (`/api/action/*`), log streaming (`/api/logs`), and update checks (`/api/check-update`) did not have authentication enforced, allowing any user to start/stop the server or read logs.
**Learning:** The web routes `/login` and `/logout` had an auth check conceptually, but the `/api` subrouter was completely exposed. This is a severe architectural gap where API requests bypassed the security layer implemented for web access.
**Prevention:** Always apply authentication middleware at the router level for sensitive subrouters instead of relying on frontend-only protection or per-handler checks. Ensure health check endpoints like `/api/status` are kept out of the authenticated subrouter to avoid breaking liveness probes.

## 2026-05-01 - [Plaintext Password in Session Cookie]
**Vulnerability:** The admin password was being stored directly as plaintext in the `soulmask_session` cookie and used to verify authentication on subsequent requests using simple string comparison (`==`). This exposes the password to the client and potentially any man-in-the-middle or XSS attacks, and opens up the authentication to timing attacks.
**Learning:** This is a severe architectural gap where the application failed to implement basic secure session handling, relying instead on a shared secret embedded on the client side.
**Prevention:** Never store passwords or sensitive secrets in cookies or client-side storage in plaintext. Always use a securely generated random session token for authentication, and use `crypto/subtle.ConstantTimeCompare` when comparing secrets to mitigate timing attacks.

## 2026-05-02 - [Overly Permissive WebSocket CORS]
**Vulnerability:** The API WebSocket upgrade endpoint (`/api/logs`) was explicitly configured to bypass origin checks when `ALLOWED_ORIGINS` was empty by returning `true` unconditionally from `checkOrigin`. This exposed the WebSocket to Cross-Site WebSocket Hijacking (CSWSH), allowing malicious sites to stream server logs on behalf of an authenticated user.
**Learning:** Bypassing library-provided secure defaults (like `gorilla/websocket`'s `CheckOrigin`) to handle "empty configurations" introduces significant security gaps. When no origins are configured, the application should fallback to the library's safe default (same-origin policy), rather than allowing all origins.
**Prevention:** Avoid writing custom CORS checks that return `true` for all origins unless explicitly intended (e.g., a public API). For WebSockets, leave `CheckOrigin` as `nil` to utilize the standard same-origin protection if no specific allowed origins are provided.
