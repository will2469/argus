// Package a29_unindexed_fk parses migration ASTs to construct a schema graph of foreign keys and indexes.
package a29_unindexed_fk

import (
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/migration"
)

// FKInfo represents a parsed foreign key relationship.
type FKInfo struct {
	Table       string
	Column      string
	ParentTable string
	FKName      string
	Filename    string
	Line        int
}

// SchemaGraph collects all foreign keys and indexed columns across migrations.
type SchemaGraph struct {
	FKs         []FKInfo
	IndexedCols map[string]bool
}

// NewSchemaGraph initializes an empty SchemaGraph.
func NewSchemaGraph() *SchemaGraph {
	return &SchemaGraph{
		IndexedCols: make(map[string]bool),
	}
}

// CollectFromTree extracts FKs, PKs, UNIQUEs, and INDEXes from a single migration AST.
func (g *SchemaGraph) CollectFromTree(filename, content string, tree *pgquery.ParseResult) {
	if g == nil || tree == nil {
		return
	}

	for _, rawStmt := range tree.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}

		// 1. Check CREATE TABLE
		if createStmt := rawStmt.Stmt.GetCreateStmt(); createStmt != nil && createStmt.Relation != nil {
			tbl := strings.ToLower(createStmt.Relation.Relname)
			for _, elt := range createStmt.TableElts {
				if colDef := elt.GetColumnDef(); colDef != nil {
					colName := strings.ToLower(colDef.Colname)
					for _, c := range colDef.Constraints {
						con := c.GetConstraint()
						if con == nil {
							continue
						}
						if con.Contype == pgquery.ConstrType_CONSTR_PRIMARY ||
							con.Contype == pgquery.ConstrType_CONSTR_UNIQUE {
							g.IndexedCols[tbl+"."+colName] = true
						}
						if con.Contype == pgquery.ConstrType_CONSTR_FOREIGN {
							line := migration.FindLineForKeyword(content, colName)
							parentTbl := ""
							if con.Pktable != nil {
								parentTbl = strings.ToLower(con.Pktable.Relname)
							}
							g.FKs = append(g.FKs, FKInfo{
								Table: tbl, Column: colName, ParentTable: parentTbl,
								FKName: con.Conname, Filename: filename, Line: line,
							})
						}
					}
				}
				if con := elt.GetConstraint(); con != nil {
					if con.Contype == pgquery.ConstrType_CONSTR_PRIMARY ||
						con.Contype == pgquery.ConstrType_CONSTR_UNIQUE {
						if len(con.Keys) > 0 && con.Keys[0].GetString_() != nil {
							leadCol := strings.ToLower(con.Keys[0].GetString_().Sval)
							g.IndexedCols[tbl+"."+leadCol] = true
						}
					}
					if con.Contype == pgquery.ConstrType_CONSTR_FOREIGN && len(con.FkAttrs) > 0 {
						parentTbl := ""
						if con.Pktable != nil {
							parentTbl = strings.ToLower(con.Pktable.Relname)
						}
						for _, attr := range con.FkAttrs {
							if attr.GetString_() != nil {
								colName := strings.ToLower(attr.GetString_().Sval)
								line := migration.FindLineForKeyword(content, colName)
								g.FKs = append(g.FKs, FKInfo{
									Table: tbl, Column: colName, ParentTable: parentTbl,
									FKName: con.Conname, Filename: filename, Line: line,
								})
							}
						}
					}
				}
			}
		}

		// 2. Check ALTER TABLE ADD CONSTRAINT
		if alterStmt := rawStmt.Stmt.GetAlterTableStmt(); alterStmt != nil && alterStmt.Relation != nil {
			tbl := strings.ToLower(alterStmt.Relation.Relname)
			for _, rawCmd := range alterStmt.Cmds {
				cmd := rawCmd.GetAlterTableCmd()
				if cmd != nil && cmd.Def != nil {
					con := cmd.Def.GetConstraint()
					if con != nil {
						if con.Contype == pgquery.ConstrType_CONSTR_PRIMARY ||
							con.Contype == pgquery.ConstrType_CONSTR_UNIQUE {
							if len(con.Keys) > 0 && con.Keys[0].GetString_() != nil {
								leadCol := strings.ToLower(con.Keys[0].GetString_().Sval)
								g.IndexedCols[tbl+"."+leadCol] = true
							}
						}
						if con.Contype == pgquery.ConstrType_CONSTR_FOREIGN && len(con.FkAttrs) > 0 {
							parentTbl := ""
							if con.Pktable != nil {
								parentTbl = strings.ToLower(con.Pktable.Relname)
							}
							for _, attr := range con.FkAttrs {
								if attr.GetString_() != nil {
									colName := strings.ToLower(attr.GetString_().Sval)
									line := migration.FindLineForKeyword(content, colName)
									g.FKs = append(g.FKs, FKInfo{
										Table: tbl, Column: colName, ParentTable: parentTbl,
										FKName: con.Conname, Filename: filename, Line: line,
									})
								}
							}
						}
					}
				}
			}
		}

		// 3. Check CREATE INDEX
		if idxStmt := rawStmt.Stmt.GetIndexStmt(); idxStmt != nil && idxStmt.Relation != nil {
			tbl := strings.ToLower(idxStmt.Relation.Relname)
			if len(idxStmt.IndexParams) > 0 {
				firstParam := idxStmt.IndexParams[0].GetIndexElem()
				if firstParam != nil && firstParam.Name != "" {
					leadCol := strings.ToLower(firstParam.Name)
					g.IndexedCols[tbl+"."+leadCol] = true
				}
			}
		}
	}
}
