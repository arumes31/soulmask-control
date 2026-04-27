## 2024-05-23 - [CRITICAL] Plaintext Password in Session Cookie
**Vulnerability:** The application stored the admin's plaintext password in the `soulmask_session` cookie and used it for authentication verification on subsequent requests.
**Learning:** This is a severe security flaw that directly exposed credentials to the client. Single-tenant applications must still use proper session management.
**Prevention:** Always generate a cryptographically secure random session token (e.g., using `crypto/rand`) for session cookies instead of raw credentials. Use constant-time comparisons (`crypto/subtle.ConstantTimeCompare`) to prevent timing attacks.
