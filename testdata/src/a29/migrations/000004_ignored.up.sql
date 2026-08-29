CREATE TABLE IF NOT EXISTS legacy_logs (
    id UUID PRIMARY KEY,
    -- argus:ignore ARGUS-A29 intentional append-only unindexed fk
    device_id UUID NOT NULL REFERENCES devices(id)
);
