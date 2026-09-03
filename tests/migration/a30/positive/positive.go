package positive // want `\[ARGUS-A30\] 001_unsafe_timestamp\.up\.sql:3: Column "created_at" on table "events" uses bare TIMESTAMP without time zone; use TIMESTAMPTZ for UTC determinism`

// Dummy exports a no-op symbol to satisfy package compilation.
func Dummy() {}
