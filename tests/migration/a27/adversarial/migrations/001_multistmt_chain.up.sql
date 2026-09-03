ALTER TABLE users ADD COLUMN age INT;
CREATE INDEX idx_users_age ON users (age);
