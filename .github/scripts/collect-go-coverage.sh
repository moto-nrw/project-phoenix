#!/usr/bin/env bash
set -euo pipefail

output="${1:-coverage.out}"
coverpkg="${COVERPKG:-./...}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

packages_file="${tmpdir}/packages.txt"
profiles_dir="${tmpdir}/profiles"
merged_profile="${tmpdir}/coverage.out"
failures_file="${tmpdir}/failures.txt"
mkdir -p "${profiles_dir}"

if [ -n "${COVERAGE_PACKAGES:-}" ]; then
  for package in ${COVERAGE_PACKAGES}; do
    go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' "${package}"
  done > "${packages_file}"
else
  go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./... > "${packages_file}"
fi

package_count="$(grep -c . "${packages_file}" || true)"
if [ "${package_count}" -eq 0 ]; then
  echo "No Go packages with tests found; writing empty coverage profile"
  printf 'mode: atomic\n' > "${output}"
  exit 0
fi

echo "Collecting Go coverage for ${package_count} packages with -coverpkg=${coverpkg}"
printf 'mode: atomic\n' > "${merged_profile}"

while IFS= read -r package; do
  [ -n "${package}" ] || continue

  safe_name="$(printf '%s' "${package}" | tr '/.' '__')"
  package_profile="${profiles_dir}/${safe_name}.out"

  echo "==> ${package}"
  if ! go test -p 1 -count=1 "${package}" -coverpkg="${coverpkg}" -covermode=atomic -coverprofile="${package_profile}"; then
    echo "Coverage failed for ${package}; retrying once"
    if ! go test -p 1 -count=1 "${package}" -coverpkg="${coverpkg}" -covermode=atomic -coverprofile="${package_profile}"; then
      echo "${package}" >> "${failures_file}"
      continue
    fi
  fi

  if [ -s "${package_profile}" ]; then
    tail -n +2 "${package_profile}" >> "${merged_profile}"
  fi
done < "${packages_file}"

if [ -s "${failures_file}" ]; then
  echo "Coverage collection failed for these packages:" >&2
  sed 's/^/  - /' "${failures_file}" >&2
  echo "Refusing to write ${output}; a partial profile would give SonarCloud bogus coverage." >&2
  exit 1
fi

mv "${merged_profile}" "${output}"
