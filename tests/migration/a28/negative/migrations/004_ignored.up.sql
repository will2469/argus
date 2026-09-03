-- argus:ignore ARGUS-A28 maintenance window constraint addition
ALTER TABLE accounts ADD CONSTRAINT chk_balance CHECK (balance >= 0);
