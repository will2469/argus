// Package a13_missing_down_migration provides AST node extraction for schema operations.
package a13_missing_down_migration

import (
	"regexp"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var doCreateTypeRegex = regexp.MustCompile(`(?i)\bCREATE\s+TYPE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z0-9_.]+)`)

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
			tbl := extractRangeVarIdent(create.Relation)
			colDefs := make(map[string]string)
			for _, elt := range create.TableElts {
				if cdef := elt.GetColumnDef(); cdef != nil {
					colDefs[strings.ToLower(cdef.Colname)] = extractTypeName(cdef.TypeName)
				}
			}
			ops = append(ops, SchemaOp{Kind: OpCreateTable, Target: tbl, ColDefs: colDefs})
			continue
		}

		if drop := stmt.GetDropStmt(); drop != nil {
			for _, obj := range drop.Objects {
				switch drop.RemoveType {
				case pg_query.ObjectType_OBJECT_TABLE:
					ops = append(ops, SchemaOp{Kind: OpDropTable, Target: extractQualifiedObjectIdent(obj)})
				case pg_query.ObjectType_OBJECT_INDEX:
					ops = append(ops, SchemaOp{Kind: OpDropIndex, Target: extractQualifiedObjectIdent(obj)})
				case pg_query.ObjectType_OBJECT_VIEW, pg_query.ObjectType_OBJECT_MATVIEW:
					ops = append(ops, SchemaOp{Kind: OpDropView, Target: extractQualifiedObjectIdent(obj)})
				case pg_query.ObjectType_OBJECT_SEQUENCE:
					ops = append(ops, SchemaOp{Kind: OpDropSequence, Target: extractQualifiedObjectIdent(obj)})
				case pg_query.ObjectType_OBJECT_SCHEMA:
					ops = append(ops, SchemaOp{Kind: OpDropSchema, Target: NewSchemaIdent(extractSchemaName(obj))})
				case pg_query.ObjectType_OBJECT_TYPE, pg_query.ObjectType_OBJECT_DOMAIN:
					ops = append(ops, SchemaOp{Kind: OpDropType, Target: extractQualifiedObjectIdent(obj)})
				}
			}
			continue
		}

		if idx := stmt.GetIndexStmt(); idx != nil {
			var tbl QualifiedIdent
			if idx.Relation != nil {
				tbl = extractRangeVarIdent(idx.Relation)
			}
			ops = append(ops, SchemaOp{
				Kind:   OpCreateIndex,
				Target: NewQualifiedIdent("", idx.Idxname),
				Table:  tbl,
			})
			continue
		}

		if view := stmt.GetViewStmt(); view != nil && view.View != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateView, Target: extractRangeVarIdent(view.View)})
			continue
		}

		if seq := stmt.GetCreateSeqStmt(); seq != nil && seq.Sequence != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateSequence, Target: extractRangeVarIdent(seq.Sequence)})
			continue
		}

		if sch := stmt.GetCreateSchemaStmt(); sch != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateSchema, Target: NewSchemaIdent(sch.Schemaname)})
			continue
		}

		if enum := stmt.GetCreateEnumStmt(); enum != nil {
			var parts []string
			for _, item := range enum.TypeName {
				if s := item.GetString_(); s != nil {
					parts = append(parts, s.Sval)
				}
			}
			if len(parts) == 1 {
				ops = append(ops, SchemaOp{Kind: OpCreateType, Target: NewQualifiedIdent("", parts[0])})
			} else if len(parts) >= 2 {
				ops = append(ops, SchemaOp{Kind: OpCreateType, Target: NewQualifiedIdent(parts[len(parts)-2], parts[len(parts)-1])})
			}
			continue
		}

		if comp := stmt.GetCompositeTypeStmt(); comp != nil && comp.Typevar != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateType, Target: extractRangeVarIdent(comp.Typevar)})
			continue
		}

		if do := stmt.GetDoStmt(); do != nil {
			for _, argNode := range do.Args {
				if def := argNode.GetDefElem(); def != nil && strings.EqualFold(def.Defname, "as") {
					if s := def.Arg.GetString_(); s != nil {
						matches := doCreateTypeRegex.FindAllStringSubmatch(s.Sval, -1)
						for _, m := range matches {
							if len(m) >= 2 {
								ops = append(ops, SchemaOp{
									Kind:   OpCreateType,
									Target: NewQualifiedIdent("", m[1]),
								})
							}
						}
					}
				}
			}
			continue
		}

		if rename := stmt.GetRenameStmt(); rename != nil {
			switch rename.RenameType {
			case pg_query.ObjectType_OBJECT_TABLE:
				sch := ""
				oldName := rename.Subname
				if rename.Relation != nil {
					sch = rename.Relation.Schemaname
					if rename.Relation.Relname != "" {
						oldName = rename.Relation.Relname
					}
				}
				ops = append(ops, SchemaOp{
					Kind:     OpRenameTable,
					Target:   NewQualifiedIdent(sch, oldName),
					NewTable: NewQualifiedIdent(sch, rename.Newname),
				})
			case pg_query.ObjectType_OBJECT_COLUMN:
				tbl := QualifiedIdent{}
				if rename.Relation != nil {
					tbl = extractRangeVarIdent(rename.Relation)
				}
				ops = append(ops, SchemaOp{
					Kind:      OpRenameColumn,
					Target:    tbl,
					SubTarget: rename.Subname,
					AuxTarget: rename.Newname,
				})
			}
			continue
		}

		if alter := stmt.GetAlterTableStmt(); alter != nil && alter.Relation != nil {
			tbl := extractRangeVarIdent(alter.Relation)
			for _, rawCmd := range alter.Cmds {
				cmd := rawCmd.GetAlterTableCmd()
				if cmd == nil {
					continue
				}
				switch cmd.Subtype {
				case pg_query.AlterTableType_AT_AddColumn:
					if cmd.Def != nil && cmd.Def.GetColumnDef() != nil {
						cdef := cmd.Def.GetColumnDef()
						ops = append(ops, SchemaOp{
							Kind:      OpAddColumn,
							Target:    tbl,
							SubTarget: cdef.Colname,
							AuxTarget: extractTypeName(cdef.TypeName),
						})
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
					newType := ""
					if cmd.Def != nil && cmd.Def.GetColumnDef() != nil {
						newType = extractTypeName(cmd.Def.GetColumnDef().TypeName)
					}
					ops = append(ops, SchemaOp{
						Kind:      OpAlterColumnType,
						Target:    tbl,
						SubTarget: cmd.Name,
						AuxTarget: newType,
					})
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

func extractRangeVarIdent(rv *pg_query.RangeVar) QualifiedIdent {
	if rv == nil {
		return QualifiedIdent{}
	}
	return NewQualifiedIdent(rv.Schemaname, rv.Relname)
}

func extractQualifiedObjectIdent(obj *pg_query.Node) QualifiedIdent {
	if obj == nil {
		return QualifiedIdent{}
	}
	if tn := obj.GetTypeName(); tn != nil {
		var parts []string
		for _, n := range tn.Names {
			if s := n.GetString_(); s != nil {
				parts = append(parts, s.Sval)
			}
		}
		if len(parts) == 1 {
			return NewQualifiedIdent("", parts[0])
		} else if len(parts) >= 2 {
			return NewQualifiedIdent(parts[len(parts)-2], parts[len(parts)-1])
		}
	}
	if list := obj.GetList(); list != nil {
		var parts []string
		for _, item := range list.Items {
			if s := item.GetString_(); s != nil {
				parts = append(parts, s.Sval)
			}
		}
		if len(parts) == 1 {
			return NewQualifiedIdent("", parts[0])
		} else if len(parts) >= 2 {
			return NewQualifiedIdent(parts[len(parts)-2], parts[len(parts)-1])
		}
	}
	if str := obj.GetString_(); str != nil {
		return NewQualifiedIdent("", str.Sval)
	}
	return QualifiedIdent{}
}

func extractSchemaName(obj *pg_query.Node) string {
	if obj == nil {
		return ""
	}
	if list := obj.GetList(); list != nil && len(list.Items) > 0 {
		if s := list.Items[0].GetString_(); s != nil {
			return s.Sval
		}
	}
	if str := obj.GetString_(); str != nil {
		return str.Sval
	}
	return ""
}

func extractTypeName(tn *pg_query.TypeName) string {
	if tn == nil || len(tn.Names) == 0 {
		return ""
	}
	var parts []string
	for _, n := range tn.Names {
		if s := n.GetString_(); s != nil {
			parts = append(parts, s.Sval)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return NormalizeTypeName(parts[len(parts)-1])
}
