package astsafety

import (
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// StorageMode represents PostgreSQL TOAST storage strategy.
type StorageMode string

const (
	StorageDefault  StorageMode = "default"  // Not specified in DDL
	StoragePlain    StorageMode = "plain"    // PLAIN (no compression, no out-of-line TOAST)
	StorageExternal StorageMode = "external" // EXTERNAL (out-of-line TOAST allowed, no compression)
	StorageExtended StorageMode = "extended" // EXTENDED (both compression and out-of-line TOAST)
	StorageMain     StorageMode = "main"     // MAIN (compression preferred, inline preferred)
)

// ExtractColumnStorage determines the explicit storage mode configured on a ColumnDef.
// GOTCHA: In pg_query_go, ColumnDef.Storage is usually empty, while ColumnDef.StorageName
// holds the lowercase string name (e.g. "plain", "external").
func ExtractColumnStorage(colDef *pg_query.ColumnDef) StorageMode {
	if colDef == nil {
		return StorageDefault
	}

	modeStr := strings.ToLower(strings.TrimSpace(colDef.StorageName))
	if modeStr == "" {
		modeStr = strings.ToLower(strings.TrimSpace(colDef.Storage))
	}

	switch modeStr {
	case "plain", "p":
		return StoragePlain
	case "external", "e":
		return StorageExternal
	case "extended", "x":
		return StorageExtended
	case "main", "m":
		return StorageMain
	default:
		return StorageDefault
	}
}

// GeneratedColumnInfo contains metadata regarding generated columns in PostgreSQL DDL.
type GeneratedColumnInfo struct {
	IsGenerated   bool
	GeneratedWhen string          // "a" for ALWAYS, "d" for BY DEFAULT
	Expression    *pg_query.Node  // AST of the generation expression
	IsStored      bool            // In PG <= 18, all generated columns are STORED
}

// ExtractGeneratedColumnInfo inspects ColumnDef constraints for CONSTR_GENERATED.
// GOTCHA: ColumnDef.Generated is often empty in the raw AST; the generator definition
// is located within ColumnDef.Constraints as a Constraint node with Contype == CONSTR_GENERATED.
func ExtractGeneratedColumnInfo(colDef *pg_query.ColumnDef) GeneratedColumnInfo {
	if colDef == nil || len(colDef.Constraints) == 0 {
		return GeneratedColumnInfo{IsGenerated: false}
	}

	for _, node := range colDef.Constraints {
		c := node.GetConstraint()
		if c == nil {
			continue
		}

		if c.Contype == pg_query.ConstrType_CONSTR_GENERATED {
			return GeneratedColumnInfo{
				IsGenerated:   true,
				GeneratedWhen: c.GeneratedWhen, // "a" (ALWAYS)
				Expression:    c.RawExpr,
				IsStored:      true, // Standard PostgreSQL requires STORED
			}
		}
	}

	return GeneratedColumnInfo{IsGenerated: false}
}
