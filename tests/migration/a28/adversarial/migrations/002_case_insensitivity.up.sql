alter table accounts add constraint chk_min_balance check (balance > 100);
