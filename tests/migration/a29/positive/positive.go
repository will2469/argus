package positive // want `\[ARGUS-A29\] 001_unsafe_unindexed\.up\.sql:4: Foreign Key on "order_items" column "product_id" \(references "products"\) has no supporting leading-column B-tree index, risking table scan lockups during parent deletes`

// Dummy exports a no-op symbol to satisfy package compilation.
func Dummy() {}
