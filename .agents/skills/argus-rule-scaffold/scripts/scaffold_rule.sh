#!/usr/bin/env bash
set -euo pipefail

# Argus Rule Scaffolding Generator (Powered by assets/ templates)
# Usage: ./scaffold_rule.sh <RULE_NUM> <RULE_NAME> <CANONICAL_IDENTIFIER> <go|sql>
# Example: ./scaffold_rule.sh 31 missing_partition_key MISSING_PARTITION_KEY go

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ASSETS_DIR="${SKILL_ROOT}/assets"

if [ "$#" -lt 4 ]; then
    echo "Usage: $0 <RULE_NUM> <RULE_NAME> <CANONICAL_IDENTIFIER> <go|sql>"
    echo "Example: $0 31 missing_partition_key MISSING_PARTITION_KEY go"
    exit 1
fi

RULE_NUM="$1"
RULE_NAME="$2"
IDENTIFIER="$3"
RULE_TYPE="$4"

if [ "$RULE_NUM" -lt 10 ]; then
    NUM_STR="0$RULE_NUM"
else
    NUM_STR="$RULE_NUM"
fi

RULE_CODE="ARGUS-A${NUM_STR}"
PKG_NAME="a${NUM_STR}_${RULE_NAME}"
SHORT_ID="a${NUM_STR}"

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
RULES_DIR="${REPO_ROOT}/rules/${PKG_NAME}"
TESTDATA_DIR="${REPO_ROOT}/testdata/src/${SHORT_ID}"
WIKI_FILE="${REPO_ROOT}/wiki/${RULE_CODE}.md"

echo "=== Scaffolding ${RULE_CODE} (${IDENTIFIER}) ==="
echo "Package Directory: rules/${PKG_NAME}"
echo "Testdata Directory: testdata/src/${SHORT_ID}"
echo "Wiki Documentation: wiki/${RULE_CODE}.md"

mkdir -p "${RULES_DIR}"
mkdir -p "${TESTDATA_DIR}"

render_template() {
    local src="$1"
    local dest="$2"
    sed \
        -e "s|{{PKG_NAME}}|${PKG_NAME}|g" \
        -e "s|{{RULE_CODE}}|${RULE_CODE}|g" \
        -e "s|{{IDENTIFIER}}|${IDENTIFIER}|g" \
        -e "s|{{SHORT_ID}}|${SHORT_ID}|g" \
        -e "s|{{TITLE_CASE_NAME}}|${IDENTIFIER}|g" \
        -e "s|{{SEVERITY}}|HIGH|g" \
        -e "s|{{CATEGORY}}|Security & Data Integrity|g" \
        "${src}" > "${dest}"
}

# 1. analyzer.go
render_template "${ASSETS_DIR}/analyzer.go.tmpl" "${RULES_DIR}/analyzer.go"

# 2. Companion walker
if [ "${RULE_TYPE}" = "go" ]; then
    render_template "${ASSETS_DIR}/ast_visitor.go.tmpl" "${RULES_DIR}/ast_visitor.go"
else
    render_template "${ASSETS_DIR}/sql_walker.go.tmpl" "${RULES_DIR}/sql_walker.go"
fi

# 3. analyzer_test.go
render_template "${ASSETS_DIR}/analyzer_test.go.tmpl" "${RULES_DIR}/analyzer_test.go"

# 4. testdata fixture
render_template "${ASSETS_DIR}/testdata.go.tmpl" "${TESTDATA_DIR}/${SHORT_ID}.go"

# 5. wiki documentation (if not already existing)
if [ ! -f "${WIKI_FILE}" ]; then
    render_template "${ASSETS_DIR}/wiki_rule.md.tmpl" "${WIKI_FILE}"
fi

echo "=== Scaffolding Complete! ==="
echo ""
echo "Next Steps Checklist (See references/wiring_guide.md for exact snippets):"
echo "1. Register in rules/rules.go (AllAnalyzers)"
echo "2. Register alias in shared/directives/alias.go (ruleAliases)"
echo "3. Register meta in runner/rules_meta.go (CanonicalDescriptions)"
if [ "${RULE_TYPE}" = "go" ]; then
    echo "4. Wire standalone in runner/scan_go.go (call ${PKG_NAME}.InspectFile)"
else
    echo "4. Wire standalone in runner/scan_migrations.go (call ${PKG_NAME}.CheckMigration)"
fi
echo "5. Run test: go test -v ./rules/${PKG_NAME}/..."
