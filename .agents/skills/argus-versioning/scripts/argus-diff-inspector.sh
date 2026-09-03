#!/usr/bin/env bash
# argus-diff-inspector.sh
# Deterministic SemVer 2.0.0 Analyzer for Argus Repository
# Evaluates git diff from latest tag to current HEAD and calculates recommended version bump.

set -euo pipefail

FORMAT="text"
TARGET_REF="HEAD"

for arg in "$@"; do
    case "$arg" in
        --json) FORMAT="json" ;;
        -h|--help)
            echo "Usage: $0 [--json] [target_ref]"
            echo "Inspects git diff from latest tag to target_ref (default: HEAD)"
            exit 0
            ;;
        *) TARGET_REF="$arg" ;;
    esac
done

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "Error: Not inside a git repository." >&2
    exit 1
fi

# 1. Resolve Latest Git Tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || git tag -l --sort=-v:refname | head -n 1 || true)

if [ -z "$LATEST_TAG" ]; then
    HAS_TAG=false
    BASE_REF=$(git rev-list --max-parents=0 HEAD 2>/dev/null | head -n 1 || echo "HEAD")
    CURRENT_VERSION="0.0.0"
    PREFIX=""
else
    HAS_TAG=true
    BASE_REF="$LATEST_TAG"
    if [[ "$LATEST_TAG" =~ ^v([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
        PREFIX="v"
        CURRENT_VERSION="${BASH_REMATCH[1]}"
    elif [[ "$LATEST_TAG" =~ ^([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
        PREFIX=""
        CURRENT_VERSION="${BASH_REMATCH[1]}"
    else
        PREFIX=""
        CURRENT_VERSION="0.1.0"
    fi
fi

# Parse Current Major, Minor, Patch
IFS='.' read -r CUR_MAJOR CUR_MINOR CUR_PATCH <<< "$CURRENT_VERSION"

# 2. Inspect Commits between Base and Target
if [ "$HAS_TAG" = false ]; then
    COMMIT_RANGE="${TARGET_REF}"
    COMMIT_COUNT=$(git rev-list --count "${TARGET_REF}" 2>/dev/null || echo 0)
else
    COMMIT_RANGE="${BASE_REF}..${TARGET_REF}"
    COMMIT_COUNT=$(git rev-list --count "$COMMIT_RANGE" 2>/dev/null || echo 0)
fi

if [ "$COMMIT_COUNT" -eq 0 ]; then
    if [ "$FORMAT" = "json" ]; then
        echo "{\"current_version\":\"${PREFIX}${CURRENT_VERSION}\",\"bump\":\"none\",\"next_version\":\"${PREFIX}${CURRENT_VERSION}\",\"commits\":0}"
    else
        echo "No commits between $BASE_REF and $TARGET_REF. Version unchanged: ${PREFIX}${CURRENT_VERSION}"
    fi
    exit 0
fi

# 3. Analyze Conventional Commits & Breaking Changes
HAS_MAJOR=false
HAS_MINOR=false
HAS_PATCH=false

MAJOR_REASONS=()
MINOR_REASONS=()
PATCH_REASONS=()

# Process commits separated by null bytes (\0)
while IFS= read -r -d '' MSG; do
    [ -z "$MSG" ] && continue

    FIRST_LINE=$(echo "$MSG" | head -n 1 | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
    [ -z "$FIRST_LINE" ] && continue

    # Check for Breaking Change Indicators
    if [[ "$FIRST_LINE" =~ ^[a-zA-Z]+(\([a-zA-Z0-9_\.\/-]+\))?!: ]] || [[ "$MSG" =~ BREAKING[[:space:]\-]CHANGE: ]]; then
        HAS_MAJOR=true
        MAJOR_REASONS+=("$FIRST_LINE")
    elif [[ "$FIRST_LINE" =~ ^feat(\([a-zA-Z0-9_\.\/-]+\))?: ]]; then
        HAS_MINOR=true
        MINOR_REASONS+=("$FIRST_LINE")
    elif [[ "$FIRST_LINE" =~ ^(fix|perf|refactor|docs|style|test|chore|build|ci)(\([a-zA-Z0-9_\.\/-]+\))?: ]]; then
        HAS_PATCH=true
        PATCH_REASONS+=("$FIRST_LINE")
    else
        HAS_PATCH=true
        PATCH_REASONS+=("$FIRST_LINE")
    fi
done < <(git log -z --format="%B" "$COMMIT_RANGE")

# 4. Determine Recommended Bump and Next Version
if [ "$HAS_MAJOR" = true ]; then
    BUMP_TYPE="MAJOR"
    NEXT_MAJOR=$((CUR_MAJOR + 1))
    NEXT_MINOR=0
    NEXT_PATCH=0
elif [ "$HAS_MINOR" = true ]; then
    BUMP_TYPE="MINOR"
    NEXT_MAJOR=$CUR_MAJOR
    NEXT_MINOR=$((CUR_MINOR + 1))
    NEXT_PATCH=0
elif [ "$HAS_PATCH" = true ]; then
    BUMP_TYPE="PATCH"
    NEXT_MAJOR=$CUR_MAJOR
    NEXT_MINOR=$CUR_MINOR
    NEXT_PATCH=$((CUR_PATCH + 1))
else
    BUMP_TYPE="PATCH"
    NEXT_MAJOR=$CUR_MAJOR
    NEXT_MINOR=$CUR_MINOR
    NEXT_PATCH=$((CUR_PATCH + 1))
fi

NEXT_VERSION="${NEXT_MAJOR}.${NEXT_MINOR}.${NEXT_PATCH}"

# 5. Output Results
if [ "$FORMAT" = "json" ]; then
    cat <<EOF
{
  "base_tag": "$BASE_REF",
  "target_ref": "$TARGET_REF",
  "current_version": "${PREFIX}${CURRENT_VERSION}",
  "bump_type": "$BUMP_TYPE",
  "next_version": "${PREFIX}${NEXT_VERSION}",
  "commit_count": $COMMIT_COUNT,
  "has_breaking_changes": $HAS_MAJOR,
  "has_new_features": $HAS_MINOR,
  "has_fixes": $HAS_PATCH
}
EOF
else
    echo "========================================================"
    echo "          ARGUS SEMVER 2.0.0 DIFF ANALYZER              "
    echo "========================================================"
    echo "Base Tag:        $BASE_REF"
    echo "Target Ref:      $TARGET_REF"
    echo "Current Version: ${PREFIX}${CURRENT_VERSION}"
    echo "Commit Count:    $COMMIT_COUNT"
    echo "--------------------------------------------------------"
    echo "Bump Decision:   $BUMP_TYPE"
    echo "Next Version:    ${PREFIX}${NEXT_VERSION}"
    echo "--------------------------------------------------------"
    if [ "$HAS_MAJOR" = true ]; then
        echo "🚨 Breaking Changes Detected (MAJOR):"
        for r in "${MAJOR_REASONS[@]}"; do echo "   - $r"; done
    fi
    if [ "$HAS_MINOR" = true ]; then
        echo "✨ New Features Detected (MINOR):"
        for r in "${MINOR_REASONS[@]}"; do echo "   - $r"; done
    fi
    if [ "$HAS_PATCH" = true ]; then
        echo "🐛 Fixes & Hygiene Detected (PATCH):"
        COUNT=0
        for r in "${PATCH_REASONS[@]}"; do
            echo "   - $r"
            COUNT=$((COUNT + 1))
            if [ "$COUNT" -ge 5 ]; then
                REMAINING=$((${#PATCH_REASONS[@]} - 5))
                if [ "$REMAINING" -gt 0 ]; then
                    echo "   ... and $REMAINING more patch commits"
                fi
                break
            fi
        done
    fi
    echo "========================================================"
fi
