package a13_missing_down_migration

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

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
