package a30 // want `\[ARGUS-A30\] 000003_unsafe_timestamp\.up\.sql:3: Column "created_at" on table "events" uses bare TIMESTAMP without time zone; use TIMESTAMPTZ for UTC determinism`

func dummy() {}
