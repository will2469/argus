#!/usr/bin/env bash
# generate-release-notes.sh
# Deterministic & Reusable Release Notes & Changelog Generator for Argus Checker
# Usage: ./generate-release-notes.sh [--write] [--summary "Custom lead text"] [target_ref]

set -euo pipefail

WRITE_MODE=false
CUSTOM_SUMMARY=""
TARGET_REF="HEAD"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --write)
            WRITE_MODE=true
            shift
            ;;
        --summary)
            CUSTOM_SUMMARY="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [--write] [--summary \"text\"] [target_ref]"
            echo ""
            echo "Generates curated, production-grade release notes from latest tag to target_ref."
            echo "Options:"
            echo "  --write              Write release_notes.md and prepend into CHANGELOG.md"
            echo "  --summary \"text\"     Provide custom executive summary for the release"
            echo "  target_ref           Git reference to compare against latest tag (default: HEAD)"
            exit 0
            ;;
        *)
            TARGET_REF="$1"
            shift
            ;;
    esac
done

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "Error: Not inside a git repository." >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ANALYZER_SCRIPT="${SCRIPT_DIR}/argus-diff-inspector.sh"

# 1. Run Diff Inspector to resolve Base Tag and Version Bump
INSPECTION_JSON=$("$ANALYZER_SCRIPT" --json "$TARGET_REF")

BASE_TAG=$(echo "$INSPECTION_JSON" | grep -o '"base_tag": "[^"]*' | cut -d'"' -f4)
NEXT_VERSION=$(echo "$INSPECTION_JSON" | grep -o '"next_version": "[^"]*' | cut -d'"' -f4)
BUMP_TYPE=$(echo "$INSPECTION_JSON" | grep -o '"bump_type": "[^"]*' | cut -d'"' -f4)
COMMIT_COUNT=$(echo "$INSPECTION_JSON" | grep -o '"commit_count": [0-9]*' | cut -d' ' -f2)
CURRENT_DATE=$(date +"%Y-%m-%d")

echo "Evaluating ${COMMIT_COUNT} commits from ${BASE_TAG} to ${TARGET_REF}..." >&2

# 2. Categorize Commits Dynamically
COMMIT_RANGE="${BASE_TAG}..${TARGET_REF}"

BREAKING_LIST=()
FEAT_LIST=()
FIX_LIST=()
PERF_LIST=()
MAINT_LIST=()

# Associative array for detecting repeated batch commits (e.g. across multiple rules/modules)
declare -A REPEATED_ACTIONS
declare -A REPEATED_ACTION_COUNT

while IFS= read -r -d '' MSG; do
    [ -z "$MSG" ] && continue
    FIRST_LINE=$(echo "$MSG" | head -n 1 | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
    [ -z "$FIRST_LINE" ] && continue

    # Filter out CI/Bot Noise and micro-typos
    if [[ "$FIRST_LINE" =~ ^(build\(deps\)|chore\(deps\)|chore\(git\)) ]] || [[ "$FIRST_LINE" =~ (bump\ actions|typo\ in\ comment) ]]; then
        continue
    fi

    # Check for Breaking Changes (SemVer Clause 7)
    if [[ "$FIRST_LINE" =~ ^[a-zA-Z]+(\([^\)]+\))?!: ]] || [[ "$MSG" =~ BREAKING[[:space:]\-]CHANGE:[[:space:]]*(.*) ]]; then
        if [[ "$MSG" =~ BREAKING[[:space:]\-]CHANGE:[[:space:]]*(.*) ]]; then
            BREAKING_LIST+=("${BASH_REMATCH[1]}")
        else
            DESC=$(echo "$FIRST_LINE" | sed -E 's/^[a-zA-Z]+(\([^\)]+\))?!:[[:space:]]*//')
            BREAKING_LIST+=("$DESC")
        fi
        continue
    fi

    # Detect batch pattern: "type(scope): action" where action repeats across scopes
    if [[ "$FIRST_LINE" =~ ^(feat|fix|refactor)\(([a-zA-Z0-9_\.\/-]+)\):[[:space:]]*(.*) ]]; then
        TYPE="${BASH_REMATCH[1]}"
        ACTION="${BASH_REMATCH[3]}"

        # Normalize action by stripping specific scope IDs (e.g. "for ARGUS-AXX")
        NORM_ACTION=$(echo "$ACTION" | sed -E 's/ for ARGUS-A[0-9]+//g; s/ for [a-zA-Z0-9_-]+$//g')

        if [ -n "${REPEATED_ACTIONS["$NORM_ACTION"]:-}" ]; then
            REPEATED_ACTION_COUNT["$NORM_ACTION"]=$((REPEATED_ACTION_COUNT["$NORM_ACTION"] + 1))
            continue
        fi

        # Count occurrences in commit history safely
        MATCH_COUNT=$(git log "$COMMIT_RANGE" --oneline | grep -F -c -- "$NORM_ACTION" || echo 1)
        MATCH_COUNT="${MATCH_COUNT//[$'\t\r\n ']/}"
        [[ "$MATCH_COUNT" =~ ^[0-9]+$ ]] || MATCH_COUNT=1

        if [ "$MATCH_COUNT" -ge 3 ]; then
            REPEATED_ACTIONS["$NORM_ACTION"]="$TYPE"
            REPEATED_ACTION_COUNT["$NORM_ACTION"]="$MATCH_COUNT"
            continue
        fi
    fi

    # Standard Conventional Commits Parsing
    if [[ "$FIRST_LINE" =~ ^feat(\([a-zA-Z0-9_\.\/-]+\))?:[[:space:]]*(.*) ]]; then
        FEAT_LIST+=("${BASH_REMATCH[2]}")
    elif [[ "$FIRST_LINE" =~ ^fix(\([a-zA-Z0-9_\.\/-]+\))?:[[:space:]]*(.*) ]]; then
        FIX_LIST+=("${BASH_REMATCH[2]}")
    elif [[ "$FIRST_LINE" =~ ^perf(\([a-zA-Z0-9_\.\/-]+\))?:[[:space:]]*(.*) ]]; then
        PERF_LIST+=("${BASH_REMATCH[2]}")
    elif [[ "$FIRST_LINE" =~ ^refactor(\([a-zA-Z0-9_\.\/-]+\))?:[[:space:]]*(.*) ]]; then
        DESC="${BASH_REMATCH[2]}"
        if [[ "$DESC" =~ (enhance|improve|support|restructure|modular|implement) ]]; then
            FEAT_LIST+=("$DESC")
        else
            MAINT_LIST+=("$DESC")
        fi
    elif [[ "$FIRST_LINE" =~ ^(docs|chore|test)(\([a-zA-Z0-9_\.\/-]+\))?:[[:space:]]*(.*) ]]; then
        MAINT_LIST+=("${BASH_REMATCH[2]}")
    fi
done < <(git log -z --format="%B" "$COMMIT_RANGE")

# Append synthesized batch actions dynamically
for act in "${!REPEATED_ACTIONS[@]}"; do
    CNT="${REPEATED_ACTION_COUNT[$act]}"
    T="${REPEATED_ACTIONS[$act]}"
    SYNTH_ITEM="$(tr '[:lower:]' '[:upper:]' <<< "${act:0:1}")${act:1} (standardized across ${CNT} components)"
    if [ "$T" = "feat" ]; then
        FEAT_LIST+=("$SYNTH_ITEM")
    elif [ "$T" = "fix" ]; then
        FIX_LIST+=("$SYNTH_ITEM")
    else
        MAINT_LIST+=("$SYNTH_ITEM")
    fi
done

# 3. Dynamic Lead Summary Construction
if [ -n "$CUSTOM_SUMMARY" ]; then
    LEAD_SUMMARY="$CUSTOM_SUMMARY"
else
    case "$BUMP_TYPE" in
        MAJOR)
            LEAD_SUMMARY="Argus ${NEXT_VERSION} is a **MAJOR** release introducing backward-incompatible API changes, contract updates, and foundational engine enhancements."
            ;;
        MINOR)
            LEAD_SUMMARY="Argus ${NEXT_VERSION} is a **MINOR** release delivering backward-compatible new features, rule extensions, and diagnostic enhancements."
            ;;
        PATCH)
            LEAD_SUMMARY="Argus ${NEXT_VERSION} is a **PATCH** release providing backward-compatible bug fixes, performance optimizations, and stability improvements."
            ;;
        *)
            LEAD_SUMMARY="Argus ${NEXT_VERSION} release update."
            ;;
    esac
fi

# 4. Construct Curated Markdown Output
NOTES=$(cat <<EOF
# Release Notes — Argus ${NEXT_VERSION} (${CURRENT_DATE})

${LEAD_SUMMARY}

---
EOF
)

# 4.1 Breaking Changes Section (Mandatory if Major)
if [ "${#BREAKING_LIST[@]}" -gt 0 ]; then
    NOTES="${NOTES}

### 🚨 Breaking Changes"
    for b in "${BREAKING_LIST[@]}"; do
        ITEM="$(tr '[:lower:]' '[:upper:]' <<< "${b:0:1}")${b:1}"
        NOTES="${NOTES}
* ${ITEM}"
    done
    NOTES="${NOTES}

---"
fi

# 4.2 Features Section
if [ "${#FEAT_LIST[@]}" -gt 0 ]; then
    NOTES="${NOTES}

### 🚀 New Features & Enhancements"
    for f in "${FEAT_LIST[@]}"; do
        ITEM="$(tr '[:lower:]' '[:upper:]' <<< "${f:0:1}")${f:1}"
        [ -z "$ITEM" ] && continue
        NOTES="${NOTES}
* ${ITEM}"
    done
fi

# 4.3 Bug Fixes Section
if [ "${#FIX_LIST[@]}" -gt 0 ]; then
    NOTES="${NOTES}

---

### 🐛 Diagnostic Precision & Bug Fixes"
    for f in "${FIX_LIST[@]}"; do
        ITEM="$(tr '[:lower:]' '[:upper:]' <<< "${f:0:1}")${f:1}"
        [ -z "$ITEM" ] && continue
        NOTES="${NOTES}
* ${ITEM}"
    done
fi

# 4.4 Performance Section
if [ "${#PERF_LIST[@]}" -gt 0 ]; then
    NOTES="${NOTES}

---

### ⚡ Performance Improvements"
    for p in "${PERF_LIST[@]}"; do
        ITEM="$(tr '[:lower:]' '[:upper:]' <<< "${p:0:1}")${p:1}"
        [ -z "$ITEM" ] && continue
        NOTES="${NOTES}
* ${ITEM}"
    done
fi

# 4.5 Maintenance Section (Curated to top 5)
if [ "${#MAINT_LIST[@]}" -gt 0 ]; then
    NOTES="${NOTES}

---

### 🔧 Maintenance & Internal Hygiene"
    COUNT=0
    for m in "${MAINT_LIST[@]}"; do
        ITEM="$(tr '[:lower:]' '[:upper:]' <<< "${m:0:1}")${m:1}"
        [ -z "$ITEM" ] && continue
        NOTES="${NOTES}
* ${ITEM}"
        COUNT=$((COUNT + 1))
        [ "$COUNT" -ge 5 ] && break
    done
fi

NOTES="${NOTES}

---

### 📦 Installation & Upgrade

#### In-Place Self-Update (Existing Installations)
\`\`\`bash
argus update # or: argus --update, argus -u
\`\`\`

#### Linux & macOS (One-Line Installer)
\`\`\`bash
curl -fsSL https://raw.githubusercontent.com/will2469/argus/main/install.sh | bash
\`\`\`

#### Windows (PowerShell)
\`\`\`powershell
irm https://raw.githubusercontent.com/will2469/argus/main/install.ps1 | iex
\`\`\`

#### Via Go Toolchain
\`\`\`bash
go install github.com/will2469/argus/cmd/argus@${NEXT_VERSION}
\`\`\`

_Or download pre-compiled binaries directly from [GitHub Releases](https://github.com/will2469/argus/releases/tag/${NEXT_VERSION})._
"

# 5. Handle Output or Write Mode
if [ "$WRITE_MODE" = false ]; then
    echo "$NOTES"
    echo ""
    echo "💡 Run with --write to update release_notes.md and prepend into CHANGELOG.md"
else
    REPO_ROOT=$(git rev-parse --show-toplevel)
    RELEASE_NOTES_PATH="${REPO_ROOT}/release_notes.md"
    CHANGELOG_PATH="${REPO_ROOT}/CHANGELOG.md"

    # Write release_notes.md
    echo "$NOTES" > "$RELEASE_NOTES_PATH"
    echo "✅ Updated ${RELEASE_NOTES_PATH}"

    # Initialize CHANGELOG.md if not present
    if [ ! -f "$CHANGELOG_PATH" ]; then
        cat << 'CHDR' > "$CHANGELOG_PATH"
# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

---

CHDR
        # Archive v1.0.0 initial release entry
        cat << 'C100' >> "$CHANGELOG_PATH"
## [v1.0.0] - 2026-08-30

### Initial Public Release
* Production-grade compile-time static analyzer for Go & PostgreSQL 18.x.
* 30 Built-in database safety and hygiene rules covering Security, Connection Lifecycle, Performance, and Zero-Downtime Migrations.
* Dual execution modes: Official `go/analysis` multichecker driver and standalone CLI runner (`argus check`, `argus check-migrations`).
* Comprehensive 8-Pillars wiki documentation for all 30 rules.

---
C100
        echo "✅ Initialized historical CHANGELOG.md with v1.0.0 baseline"
    fi

    # Check if section already exists in CHANGELOG.md to prevent duplicate entries
    if grep -q "## \[${NEXT_VERSION}\]" "$CHANGELOG_PATH"; then
        # Replace existing entry
        TEMP_FILE=$(mktemp)
        awk -v target="## [${NEXT_VERSION}]" -v new_block="## [${NEXT_VERSION}] - ${CURRENT_DATE}\n$(echo "$NOTES" | sed '1d')\n---" '
            BEGIN { skipping = 0 }
            $0 ~ target {
                skipping = 1
                print new_block
                next
            }
            skipping && /^## \[/ {
                skipping = 0
            }
            !skipping { print }
        ' "$CHANGELOG_PATH" > "$TEMP_FILE"
        mv "$TEMP_FILE" "$CHANGELOG_PATH"
        echo "✅ Updated existing ${NEXT_VERSION} entry in ${CHANGELOG_PATH}"
    else
        # Prepend new release block right below the top delimiter in CHANGELOG.md
        TEMP_FILE=$(mktemp)
        awk -v next_ver="${NEXT_VERSION}" -v date="${CURRENT_DATE}" -v notes_body="$(echo "$NOTES" | sed '1d')" '
            BEGIN { inserted = 0 }
            /^---/ && inserted == 0 {
                print $0
                print ""
                print "## [" next_ver "] - " date
                print notes_body
                print "---"
                inserted = 1
                next
            }
            { print }
        ' "$CHANGELOG_PATH" > "$TEMP_FILE"
        mv "$TEMP_FILE" "$CHANGELOG_PATH"
        echo "✅ Prepended ${NEXT_VERSION} into ${CHANGELOG_PATH}"
    fi
fi
