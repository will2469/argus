-- argus:ignore-a11 approved contract phase cleanup of deprecated column
ALTER TABLE users DROP COLUMN legacy_bio;
