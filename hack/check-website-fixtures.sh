#!/usr/bin/env bash
set -euo pipefail

# Verifies that testdata/website/ fixtures match the YAML examples
# published in the dcm-project.github.io Getting Started guides.
#
# Usage: hack/check-website-fixtures.sh [--update]
#   --update  Overwrite local fixtures with upstream content (for refresh)
#
# Exit codes: 0 = success, 1 = runtime/drift error, 2 = usage error

usage() {
    echo "Usage: ${0##*/} [--update]"
    echo "  --update  Overwrite local fixtures with upstream content"
}

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURE_DIR="${REPO_ROOT}/testdata/website"
WEBSITE_REPO="dcm-project/dcm-project.github.io"
WEBSITE_BRANCH="main"
BASE_URL="https://raw.githubusercontent.com/${WEBSITE_REPO}/${WEBSITE_BRANCH}/content/docs/getting-started"

UPDATE=false
case "${1:-}" in
    "")        ;;
    --update)  UPDATE=true ;;
    *)
        echo "Error: unknown argument: $1" >&2
        usage >&2
        exit 2
        ;;
esac
if [[ $# -gt 1 ]]; then
    echo "Error: too many arguments" >&2
    usage >&2
    exit 2
fi

# Maps local fixture filenames to their source markdown files.
declare -A FIXTURE_SOURCES=(
    ["small-vm.yaml"]="create-small-vm-catalog-item.md"
    ["my-vm.yaml"]="create-small-vm-instance.md"
)

extract_yaml_block() {
    # Extracts the first ```yaml ... ``` fenced block from markdown on stdin.
    awk '
        /^```yaml/ { capture=1; next }
        /^```/ && capture { capture=0; next }
        capture { print }
    '
}

errors=0
fetch_failures=0
compared=0

for fixture in "${!FIXTURE_SOURCES[@]}"; do
    source_file="${FIXTURE_SOURCES[$fixture]}"
    local_path="${FIXTURE_DIR}/${fixture}"
    url="${BASE_URL}/${source_file}"

    echo "Checking ${fixture} ← ${source_file}"

    if ! markdown=$(curl -sf --connect-timeout 10 --max-time 30 --retry 3 --retry-delay 2 --retry-all-errors "${url}"); then
        echo "  WARNING: Could not fetch ${url} (network error, 404, or rate limiting)"
        fetch_failures=$((fetch_failures + 1))
        continue
    fi

    upstream_yaml=$(echo "${markdown}" | extract_yaml_block)
    if [[ -z "${upstream_yaml}" ]]; then
        echo "  ERROR: No YAML code block found in ${source_file}"
        errors=$((errors + 1))
        continue
    fi

    if [[ "${UPDATE}" == "true" ]]; then
        echo "${upstream_yaml}" > "${local_path}"
        echo "  Updated ${local_path}"
        continue
    fi

    if [[ ! -f "${local_path}" ]]; then
        echo "  ERROR: Local fixture missing: ${local_path}"
        echo "  Run: hack/check-website-fixtures.sh --update"
        errors=$((errors + 1))
        continue
    fi

    if ! diff_output=$(diff -u "${local_path}" <(echo "${upstream_yaml}")); then
        echo "  DRIFT DETECTED: ${fixture} differs from upstream"
        echo "${diff_output}"
        echo ""
        echo "  To update: hack/check-website-fixtures.sh --update"
        errors=$((errors + 1))
    else
        echo "  OK"
    fi
    compared=$((compared + 1))
done

if [[ ${errors} -gt 0 ]]; then
    echo ""
    echo "FAIL: ${errors} fixture(s) out of sync with ${WEBSITE_REPO}"
    exit 1
fi

if [[ ${fetch_failures} -gt 0 ]]; then
    echo ""
    if [[ "${GITHUB_EVENT_NAME:-}" == "schedule" ]] || [[ ${compared} -eq 0 ]]; then
        echo "FAIL: ${fetch_failures} fixture(s) could not be fetched — no fixtures were checked"
        exit 1
    fi
    echo "WARNING: ${fetch_failures} fixture(s) could not be fetched (skipped; network issue or rate limiting)"
    echo "Re-run or check manually: hack/check-website-fixtures.sh"
    exit 0
fi

echo ""
echo "All fixtures match upstream."
