CREATE TABLE IF NOT EXISTS legacy_raw_ticks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- argus:ignore ARGUS-A30 intentional raw clock tick without timezone
    tick_time TIMESTAMP NOT NULL
);
