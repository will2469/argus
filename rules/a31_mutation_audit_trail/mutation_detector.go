// Package a31_mutation_audit_trail provides mutation recognition utilities
// to identify database state modifications and evaluate table exemption.
package a31_mutation_audit_trail

import (
	"go/ast"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

var dmlMutationRegex = regexp.MustCompile(`(?i)^\s*(UPDATE|DELETE\s+FROM|INSERT\s+INTO|MERGE\s+INTO)\s+([a-zA-Z0-9_.]+)`)

type mutationResult struct {
	isMutation bool
	isExempt   bool
	table      string
}

// checkMutationCall inspects a call expression to determine if it performs a database mutation.
func checkMutationCall(call *ast.CallExpr, exemptTables []string, pass *analysis.Pass) mutationResult {
	if call == nil {
		return mutationResult{}
	}

	methodName := callsite.GetCallMethodName(call.Fun)
	if methodName == "" {
		return mutationResult{}
	}

	// 1. Direct database execution methods (Exec, ExecContext, Queue)
	if callsite.IsDBQueryMethod(methodName) {
		// Read-only queries (Query, QueryRow) are not mutations
		if methodName == "Query" || methodName == "QueryRow" {
			return mutationResult{}
		}

		sqlArg := callsite.ExtractSQLArg(call, pass)
		if sqlArg != nil {
			strVal := extractSimpleQueryString(sqlArg)
			if strVal != "" {
				match := dmlMutationRegex.FindStringSubmatch(strVal)
				if len(match) >= 3 {
					table := strings.ToLower(strings.TrimSpace(match[2]))
					isExempt := isTableExempt(table, exemptTables)
					return mutationResult{
						isMutation: true,
						isExempt:   isExempt,
						table:      table,
					}
				}
				// If query explicitly starts with SELECT, not a mutation
				if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(strVal)), "SELECT") {
					return mutationResult{}
				}
			}
		}

		// Exec / ExecContext on a database/tx without proven read-only query is treated as mutation
		return mutationResult{
			isMutation: true,
			isExempt:   false,
			table:      "",
		}
	}

	// 2. Repository-style mutation method names
	if isRepositoryMutationMethod(methodName) {
		return mutationResult{
			isMutation: true,
			isExempt:   false,
			table:      "",
		}
	}

	return mutationResult{}
}

func isTableExempt(table string, exemptTables []string) bool {
	cleanTable := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(table)), "public.")

	for _, ex := range exemptTables {
		cleanEx := strings.ToLower(strings.TrimSpace(ex))
		if cleanTable == cleanEx || cleanTable == "public."+cleanEx {
			return true
		}
	}
	return false
}

func isRepositoryMutationMethod(methodName string) bool {
	lower := strings.ToLower(methodName)
	switch {
	case strings.HasPrefix(lower, "insert"),
		strings.HasPrefix(lower, "update"),
		strings.HasPrefix(lower, "delete"),
		strings.HasPrefix(lower, "create"),
		strings.HasPrefix(lower, "mutate"):
		return true
	}
	return false
}
