// Package rules provides the central registry of all active Argus analyzers.
package rules

import (
	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/rules/a01_sql_concat"
	"github.com/will2469/argus/rules/a02_unclosed_rows"
	"github.com/will2469/argus/rules/a03_context"
	"github.com/will2469/argus/rules/a04_orderby"
	"github.com/will2469/argus/rules/a05_audit_immutability"
	"github.com/will2469/argus/rules/a06_runtime_ddl"
	"github.com/will2469/argus/rules/a07_error_leak"
	"github.com/will2469/argus/rules/a08_tx_io"
	"github.com/will2469/argus/rules/a09_advisory_lock"
	"github.com/will2469/argus/rules/a10_isolation_level"
	"github.com/will2469/argus/rules/a11_destructive_migration"
	"github.com/will2469/argus/rules/a12_timeout_config"
	"github.com/will2469/argus/rules/a13_missing_down_migration"
	"github.com/will2469/argus/rules/a14_select_star"
	"github.com/will2469/argus/rules/a15_ddl_grant"
	"github.com/will2469/argus/rules/a16_max_conns"
	"github.com/will2469/argus/rules/a17_nplusone"
	"github.com/will2469/argus/rules/a18_rows_err"
	"github.com/will2469/argus/rules/a19_unbounded_limit"
	"github.com/will2469/argus/rules/a20_param_limit"
	"github.com/will2469/argus/rules/a21_row_lock"
	"github.com/will2469/argus/rules/a22_serializable_retry"
	"github.com/will2469/argus/rules/a23_tx_timeout"
	"github.com/will2469/argus/rules/a24_tenant_leak"
	"github.com/will2469/argus/rules/a25_expensive_cpu"
	"github.com/will2469/argus/rules/a26_like_sanitize"
	"github.com/will2469/argus/rules/a27_concurrent_index"
	"github.com/will2469/argus/rules/a28_constraint_lock"
	"github.com/will2469/argus/rules/a29_unindexed_fk"
	"github.com/will2469/argus/rules/a30_timestamptz"
)

// AllAnalyzers returns all currently registered and active analyzers.
var AllAnalyzers = []*analysis.Analyzer{
	a01_sql_concat.Analyzer,
	a02_unclosed_rows.Analyzer,
	a03_context.Analyzer,
	a04_orderby.Analyzer,
	a05_audit_immutability.Analyzer,
	a06_runtime_ddl.Analyzer,
	a07_error_leak.Analyzer,
	a08_tx_io.Analyzer,
	a09_advisory_lock.Analyzer,
	a10_isolation_level.Analyzer,
	a11_destructive_migration.Analyzer,
	a12_timeout_config.Analyzer,
	a13_missing_down_migration.Analyzer,
	a14_select_star.Analyzer,
	a15_ddl_grant.Analyzer,
	a16_max_conns.Analyzer,
	a17_nplusone.Analyzer,
	a18_rows_err.Analyzer,
	a19_unbounded_limit.Analyzer,
	a20_param_limit.Analyzer,
	a21_row_lock.Analyzer,
	a22_serializable_retry.Analyzer,
	a23_tx_timeout.Analyzer,
	a24_tenant_leak.Analyzer,
	a25_expensive_cpu.Analyzer,
	a26_like_sanitize.Analyzer,
	a27_concurrent_index.Analyzer,
	a28_constraint_lock.Analyzer,
	a29_unindexed_fk.Analyzer,
	a30_timestamptz.Analyzer,
}
