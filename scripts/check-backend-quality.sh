#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$root/scripts/quality-timing.sh"
cd "$root/backend"
formatting() {
  local unformatted
  unformatted=$(gofmt -l .)
  if [[ -n "$unformatted" ]]; then
    printf 'Run gofmt on:\n%s\n' "$unformatted" >&2
    exit 1
  fi
}
# Advisories whose vuln.go.dev record is demonstrably wrong. Entries are data
# defects, not accepted risk, so each one needs an upstream correction request
# and goes away once the database is fixed. govulncheck has no ignore flag.
#   GO-2026-6452: published without a fixed version although the fix from
#   qax-os/excelize#2331 shipped in excelize v2.11.0, the version backend/go.mod
#   requires. Correction requested in golang/vulndb#6501.
misreported_advisories='["GO-2026-6452"]'
vulnerabilities() {
  local findings
  # JSON output always exits 0, so the verdict is built here. Findings without a
  # symbol in the trace are module-level notices that plain govulncheck does not
  # fail on either.
  findings=$(govulncheck -format json ./... | jq -rs --argjson allowed "$misreported_advisories" '
    [.[] | select(has("finding")).finding | select(.trace[0].function != null).osv]
      | (unique - $allowed)
      | .[]')
  if [[ -n "$findings" ]]; then
    printf 'Vulnerabilities found:\n%s\n' "$findings" >&2
    govulncheck ./... >&2 || true
    exit 1
  fi
}
case "${1:-quality}" in
  quality)
    quality_step go-format formatting
    quality_step source-ratchets go test ./test -run Ratchet -count=1
    quality_step migration-validation go run main.go migrate validate
    quality_step migration-collisions go test ./database/migrations/ -run TestNoDuplicateMigrationVersions -v
    quality_step deadcode-regressions bash "$root/scripts/check-deadcode_test.sh"
    quality_step deadcode bash "$root/scripts/check-deadcode.sh"
    quality_step go-licenses go tool go-licenses check ./... --ignore github.com/moto-nrw/project-phoenix --disallowed_types=restricted,reciprocal
    ;;
  lint)
    base=${2:?base revision is required}
    changed=$(git diff --name-only "$(git merge-base HEAD "$base")" HEAD)
    if printf '%s\n' "$changed" | grep -qE '^(backend/go\.(mod|sum)|backend/\.golangci.yml|scripts/(backend-affected-packages.*|backend-lint-packages.sh|check-backend-quality.sh))$'; then
      output=./...
    else
      output=$("$root/scripts/backend-affected-packages.sh" "$base")
    fi
    packages=()
    module=$(go list -m)
    while IFS= read -r package; do
      case "$package" in
        "") ;;
        "$module") packages+=(.) ;;
        "$module"/*) packages+=(".${package#"$module"}") ;;
        *) packages+=("$package") ;;
      esac
    done <<< "$output"
    [[ ${#packages[@]} -gt 0 ]] || packages=(./...)
    golangci-lint run --allow-serial-runners --timeout 20m "${packages[@]}"
    ;;
  vulnerabilities) vulnerabilities ;;
  *) echo 'Unknown backend quality mode' >&2; exit 2 ;;
esac
