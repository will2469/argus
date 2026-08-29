-- argus:ignore ARGUS-A27 intentional offline maintenance migration
CREATE INDEX idx_users_legacy ON users (legacy_id);
