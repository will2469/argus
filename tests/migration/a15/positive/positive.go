package positive // want `\[ARGUS-A15\] 001_unsafe_all.up.sql: Forbidden grant of DDL permission \(ALL PRIVILEGES\) to runtime role "app_user"` `\[ARGUS-A15\] 002_unsafe_create.up.sql: Forbidden grant of DDL permission \(CREATE, DROP, TRUNCATE\) to runtime role "app_user"` `\[ARGUS-A15\] 003_unsafe_owner.up.sql: Forbidden table ownership grant to runtime app role "app_user"`

// Dummy exports a no-op symbol to satisfy package compilation.
func Dummy() {}
