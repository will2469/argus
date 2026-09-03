package positive // want `\[ARGUS-A27\] 001_unsafe_plain.up.sql:1: CREATE INDEX "idx_users_phone" on existing table "users" must use CONCURRENTLY to prevent production SHARE lockouts`

// Dummy exports a no-op symbol to satisfy package compilation.
func Dummy() {}
