package positive // want `\[ARGUS-A28\] 001_unsafe_direct_fk\.up\.sql:1: ADD FOREIGN KEY constraint "fk_orders_user" on existing table "orders" must use NOT VALID \(followed by separate VALIDATE CONSTRAINT\) to prevent multi-table write lockouts` `\[ARGUS-A28\] 002_unsafe_direct_chk\.up\.sql:1: ADD CHECK constraint "chk_users_age" on existing table "users" must use NOT VALID \(followed by separate VALIDATE CONSTRAINT\) to prevent multi-table write lockouts`

// Dummy exports a no-op symbol to satisfy package compilation.
func Dummy() {}
