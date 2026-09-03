package astsafety

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// NullTestDirection indicates whether an expression is asserting IS NULL or IS NOT NULL.
type NullTestDirection int

const (
	NullTestUnknown NullTestDirection = iota
	NullTestIsNull
	NullTestIsNotNull
)

// EvaluateNullTest inspects a NullTest node to safely distinguish IS NULL from IS NOT NULL.
// GOTCHA: Nulltesttype enum is pg_query.NullTestType_IS_NULL (1) vs pg_query.NullTestType_IS_NOT_NULL (2).
// Assuming Nulltesttype == 0 or doing string matching leads to inverted logic bugs.
func EvaluateNullTest(node *pg_query.Node) (direction NullTestDirection, testedArg *pg_query.Node) {
	if node == nil {
		return NullTestUnknown, nil
	}

	nullTest := node.GetNullTest()
	if nullTest == nil {
		return NullTestUnknown, nil
	}

	switch nullTest.Nulltesttype {
	case pg_query.NullTestType_IS_NULL:
		return NullTestIsNull, nullTest.Arg
	case pg_query.NullTestType_IS_NOT_NULL:
		return NullTestIsNotNull, nullTest.Arg
	default:
		return NullTestUnknown, nullTest.Arg
	}
}
