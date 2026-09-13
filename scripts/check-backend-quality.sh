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
  vulnerabilities) govulncheck ./... ;;
  *) echo 'Unknown backend quality mode' >&2; exit 2 ;;
esac
