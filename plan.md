1. **Remove Hardcoded Default Password**
   - Update `cmd/server/main.go` to auto-generate a random secure hex password on startup if the `ADMIN_PASSWORD` environment variable is not set.
   - Output the generated password to logs so the administrator can retrieve it.

2. **Add Sentinel Journal Entry**
   - Read `.jules/sentinel.md`.
   - Prepend a new entry detailing the hardcoded admin password vulnerability and the rationale for the fix.

3. **Verify Changes**
   - Run `go build ./...` to ensure it compiles.
   - Run `gosec` to verify no security issues exist in the updated code.
   - Run tests using `go test ./...`.

4. **Complete pre commit steps**
   - Follow `pre_commit_instructions` to ensure proper testing, verifications, reviews and reflections are done.

5. **Submit**
   - Submit the PR according to Sentinel's specific PR template formatting rules.
