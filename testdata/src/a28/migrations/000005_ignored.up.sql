-- argus:ignore ARGUS-A28 intentional offline maintenance migration
ALTER TABLE users ADD CONSTRAINT chk_users_legacy CHECK (legacy_id > 0);
