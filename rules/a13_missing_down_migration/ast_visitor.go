// Package a13_missing_down_migration provides AST node extraction for schema operations.
package a13_missing_down_migration

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func hasDDL(ops []SchemaOp) bool {
	for _, op := range ops {
		if op.Kind != OpDML {
			return true
		}
	}
	return false
}

func extractSchemaOps(tree *pg_query.ParseResult) []SchemaOp {
	var ops []SchemaOp
	if tree == nil {
		return ops
	}

	for _, rawStmt := range tree.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}
		stmt := rawStmt.Stmt

		if create := stmt.GetCreateStmt(); create != nil && create.Relation != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateTable, Target: create.Relation.Relname})
			continue
		}

		if drop := stmt.GetDropStmt(); drop != nil {
			for _, obj := range drop.Objects {
				name := extractObjectName(obj)
				switch drop.RemoveType {
				case pg_query.ObjectType_OBJECT_TABLE:
					ops = append(ops, SchemaOp{Kind: OpDropTable, Target: name})
				case pg_query.ObjectType_OBJECT_INDEX:
					ops = append(ops, SchemaOp{Kind: OpDropIndex, Target: name})
				case pg_query.ObjectType_OBJECT_VIEW, pg_query.ObjectType_OBJECT_MATVIEW:
					ops = append(ops, SchemaOp{Kind: OpDropView, Target: name})
				case pg_query.ObjectType_OBJECT_SEQUENCE:
					ops = append(ops, SchemaOp{Kind: OpDropSequence, Target: name})
				case pg_query.ObjectType_OBJECT_SCHEMA:
					ops = append(ops, SchemaOp{Kind: OpDropSchema, Target: name})
				case pg_query.ObjectType_OBJECT_TYPE, pg_query.ObjectType_OBJECT_DOMAIN:
					ops = append(ops, SchemaOp{Kind: OpDropType, Target: name})
				}
			}
			continue
		}

		if idx := stmt.GetIndexStmt(); idx != nil {
			tbl := ""
			if idx.Relation != nil {
				tbl = idx.Relation.Relname
			}
			ops = append(ops, SchemaOp{Kind: OpCreateIndex, Target: idx.Idxname, SubTarget: tbl})
			continue
		}

		if view := stmt.GetViewStmt(); view != nil && view.View != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateView, Target: view.View.Relname})
			continue
		}

		if seq := stmt.GetCreateSeqStmt(); seq != nil && seq.Sequence != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateSequence, Target: seq.Sequence.Relname})
			continue
		}

		if sch := stmt.GetCreateSchemaStmt(); sch != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateSchema, Target: sch.Schemaname})
			continue
		}

		if rename := stmt.GetRenameStmt(); rename != nil {
			switch rename.RenameType {
			case pg_query.ObjectType_OBJECT_TABLE:
				oldName := rename.Subname
				if rename.Relation != nil && rename.Relation.Relname != "" {
					oldName = rename.Relation.Relname
				}
				ops = append(ops, SchemaOp{Kind: OpRenameTable, Target: oldName, SubTarget: rename.Newname})
			case pg_query.ObjectType_OBJECT_COLUMN:
				tbl := ""
				if rename.Relation != nil {
					tbl = rename.Relation.Relname
				}
				ops = append(ops, SchemaOp{Kind: OpRenameColumn, Target: tbl, SubTarget: rename.Subname, AuxTarget: rename.Newname})
			}
			continue
		}

		if alter := stmt.GetAlterTableStmt(); alter != nil && alter.Relation != nil {
			tbl := alter.Relation.Relname
			for _, rawCmd := range alter.Cmds {
				cmd := rawCmd.GetAlterTableCmd()
				if cmd == nil {
					continue
				}
				switch cmd.Subtype {
				case pg_query.AlterTableType_AT_AddColumn:
					if cmd.Def != nil && cmd.Def.GetColumnDef() != nil {
						ops = append(ops, SchemaOp{Kind: OpAddColumn, Target: tbl, SubTarget: cmd.Def.GetColumnDef().Colname})
					}
				case pg_query.AlterTableType_AT_DropColumn:
					ops = append(ops, SchemaOp{Kind: OpDropColumn, Target: tbl, SubTarget: cmd.Name})
				case pg_query.AlterTableType_AT_AddConstraint:
					con := ""
					if cmd.Def != nil && cmd.Def.GetConstraint() != nil {
						con = cmd.Def.GetConstraint().Conname
					}
					ops = append(ops, SchemaOp{Kind: OpAddConstraint, Target: tbl, SubTarget: con})
				case pg_query.AlterTableType_AT_DropConstraint:
					ops = append(ops, SchemaOp{Kind: OpDropConstraint, Target: tbl, SubTarget: cmd.Name})
				case pg_query.AlterTableType_AT_AlterColumnType:
					ops = append(ops, SchemaOp{Kind: OpAlterColumnType, Target: tbl, SubTarget: cmd.Name})
				}
			}
			continue
		}

		if stmt.GetSelectStmt() != nil || stmt.GetUpdateStmt() != nil || stmt.GetInsertStmt() != nil || stmt.GetDeleteStmt() != nil {
			ops = append(ops, SchemaOp{Kind: OpDML})
		}
	}

	return ops
}

func extractObjectName(obj *pg_query.Node) string {
	if obj == nil {
		return ""
	}
	if list := obj.GetList(); list != nil {
		var parts []string
		for _, item := range list.Items {
			if s := item.GetString_(); s != nil {
				parts = append(parts, s.Sval)
			}
		}
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	if str := obj.GetString_(); str != nil {
		return str.Sval
	}
	return ""
}
