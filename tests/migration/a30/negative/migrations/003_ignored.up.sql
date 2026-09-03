CREATE TABLE legacy_ticks (
    id UUID PRIMARY KEY,
    -- argus:ignore ARGUS-A30 legacy integration with third-party without tz
    tick_time TIMESTAMP
);
