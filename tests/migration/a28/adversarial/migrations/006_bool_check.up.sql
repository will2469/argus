ALTER TABLE users ADD CONSTRAINT chk_active CHECK (is_active IS TRUE);
