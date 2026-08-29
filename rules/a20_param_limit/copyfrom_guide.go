// Package a20_param_limit provides remediation guidance for bulk ingestion and parameter limits.
package a20_param_limit

// DynamicBatchKind indicates the type of dynamic multi-row operation.
type DynamicBatchKind int

const (
	BatchKindNone DynamicBatchKind = iota
	BatchKindDynamicValues
	BatchKindDynamicInClause
)

// GetRemediationHelp returns actionable guidance for resolving ARGUS-A20 violations.
func GetRemediationHelp(kind DynamicBatchKind) string {
	switch kind {
	case BatchKindDynamicValues:
		return "Use 'pgx.CopyFrom' for high-throughput bulk inserts or wrap multi-row batching in a chunking loop (chunkSize <= 1000)."
	case BatchKindDynamicInClause:
		return "Replace dynamic 'IN ($1, $2, ... $N)' with PostgreSQL array operator 'WHERE col = ANY($1)' using a single slice argument."
	default:
		return "Ensure total parameter count (rows * columns) never exceeds PostgreSQL 65,535 int16 wire protocol ceiling."
	}
}
