ALTER TABLE payments ADD CONSTRAINT chk_amt CHECK (amount > 0);
