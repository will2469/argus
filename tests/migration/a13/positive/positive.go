package positive // want `\[ARGUS-A13\] 002_orders.up.sql: Missing required rollback file` `\[ARGUS-A13\] 003_empty.down.sql: Rollback migration .* is empty` `\[ARGUS-A13\] 004_dummy_select.down.sql: Rollback migration .* is not a valid inverse` `\[ARGUS-A13\] 005_target_mismatch.down.sql: Rollback migration .* is not a valid inverse`

// Dummy exports a no-op symbol to satisfy package compilation.
func Dummy() {}
