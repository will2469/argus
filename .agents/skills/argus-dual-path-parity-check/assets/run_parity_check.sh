#!/usr/bin/env bash
set -euo pipefail

# Argus Dual-Path Parity Verification Script
# Usage: ./run_parity_check.sh <RULE_NUM> <RULE_NAME>
# Example: ./run_parity_check.sh 01 sql_concat

if [ "$#" -lt 2 ]; then
    echo "Usage: $0 <RULE_NUM> <RULE_NAME>"
    echo "Example: $0 01 sql_concat"
    exit 1
fi

RULE_NUM="$1"
RULE_NAME="$2"

if [ "$RULE_NUM" -lt 10 ]; then
    NUM_STR="0$RULE_NUM"
else
    NUM_STR="$RULE_NUM"
fi

RULE_CODE="ARGUS-A${NUM_STR}"
PKG_NAME="a${NUM_STR}_${RULE_NAME}"
SHORT_ID="a${NUM_STR}"

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

echo "=== 1. Testing Path 1: Official Analysis Driver for ${RULE_CODE} (analysistest) ==="
go test -v "${REPO_ROOT}/rules/${PKG_NAME}/..."

echo "=== 2. Building Standalone Binary ==="
make -C "${REPO_ROOT}" build

echo "=== 3. Testing Path 2: Standalone CLI Runner ==="
FIXTURE_DIR="${REPO_ROOT}/testdata/src/${SHORT_ID}"
STANDALONE_OUTPUT="$("${REPO_ROOT}/bin/argus" check --dirs "${FIXTURE_DIR}" --no-report 2>&1 || true)"

echo "Standalone Output Summary:"
echo "${STANDALONE_OUTPUT}"

echo "=== 4. Parity Assertion ==="
if echo "${STANDALONE_OUTPUT}" | grep -qi "FAILED"; then
    echo "✅ Parity Check PASSED: Standalone runner successfully detected violations for ${RULE_CODE} in test fixture!"
else
    echo "❌ PARITY FAILURE: Standalone runner reported 0 violations for ${RULE_CODE} on testdata/src/${SHORT_ID}!"
    echo "This indicates silent divergence (like the A01 bug). Check runner/scan_go.go or runner/scan_migrations.go!"
    exit 1
fi
