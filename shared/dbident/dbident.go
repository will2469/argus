// Package dbident provides the single source of truth for database driver
// identity verification across all Argus rules. It replaces per-rule
// copies of isKnownDBPackagePath, isKnownDBDriverType, and structural
// interface heuristics with provenance-anchored type checking.
package dbident

import (
	"go/types"
)

// knownDBPackagePaths is the canonical registry of database driver import
// paths. Adding a new driver means editing ONLY this map.
var knownDBPackagePaths = map[string]bool{
	"database/sql":                    true,
	"github.com/jackc/pgx/v5":         true,
	"github.com/jackc/pgx/v5/pgxpool": true,
	"github.com/jackc/pgx/v5/pgconn":  true,
	"github.com/jackc/pgx/v4":         true,
	"github.com/jackc/pgx/v4/pgxpool": true,
	"github.com/jmoiron/sqlx":         true,
	"github.com/lib/pq":               true,
}

// knownDBPoolTypeNames are concrete struct/interface names from known
// packages that represent connection pools.
var knownDBPoolTypeNames = map[string]bool{
	"DB":   true,
	"Pool": true,
	"Conn": true,
}

// knownDBTxTypeNames are concrete struct/interface names from known
// packages that represent database transactions.
var knownDBTxTypeNames = map[string]bool{
	"Tx": true,
}

// knownDBDriverTypeNames are ALL concrete type names from known packages
// that participate in database operations (queriers, results, etc.).
var knownDBDriverTypeNames = map[string]bool{
	"DB": true, "Tx": true, "Conn": true, "Pool": true,
	"Batch": true, "BatchResults": true,
	"Stmt": true, "Rows": true, "Row": true,
	"Result": true, "CommandTag": true,
}

// IsKnownDBPackagePath reports whether path is a recognized database driver
// import path (database/sql, pgx, pgxpool, sqlx, pq, pgconn).
func IsKnownDBPackagePath(path string) bool {
	return knownDBPackagePaths[path]
}

// IsKnownDBDriverType reports whether t is a named type from a known
// database driver package (DB, Tx, Rows, etc.). Pointers are unwrapped.
func IsKnownDBDriverType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = UnwrapPointer(t)
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil {
		return false
	}
	obj := named.Obj()
	if obj.Pkg() == nil || !IsKnownDBPackagePath(obj.Pkg().Path()) {
		return false
	}
	return knownDBDriverTypeNames[obj.Name()]
}

// IsKnownDBPoolType reports whether t is a concrete pool/connection type
// from a known database driver package (sql.DB, pgxpool.Pool, pgx.Conn).
func IsKnownDBPoolType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = UnwrapPointer(t)
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil {
		return false
	}
	obj := named.Obj()
	if obj.Pkg() == nil || !IsKnownDBPackagePath(obj.Pkg().Path()) {
		return false
	}
	return knownDBPoolTypeNames[obj.Name()]
}

// IsKnownDBTxType reports whether t is a concrete transaction type from a
// known database driver package (sql.Tx, pgx.Tx).
func IsKnownDBTxType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = UnwrapPointer(t)
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil {
		return false
	}
	obj := named.Obj()
	if obj.Pkg() == nil || !IsKnownDBPackagePath(obj.Pkg().Path()) {
		return false
	}
	return knownDBTxTypeNames[obj.Name()]
}

// IsDBConstructorMethod reports whether methodName is a known database
// pool/connection constructor.
func IsDBConstructorMethod(methodName string) bool {
	switch methodName {
	case "Open", "OpenDB", "Connect", "ConnectConfig", "New", "NewWithConfig":
		return true
	}
	return false
}
