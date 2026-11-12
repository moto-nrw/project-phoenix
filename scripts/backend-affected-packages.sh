#!/usr/bin/env bash
# Prints changed backend packages. Production changes also select every package
# whose production or test graph depends on them. Output is one import per line.
#
# --direct limits the selection to depth 1: only direct importers instead of
# the transitive closure, and without the always-appended repo-wide ./test
# ratchet package. scripts/test-changed.sh --fast separately selects source
# ratchets by their filesystem inputs; CI keeps the default full closure.
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"
direct=false
if [ "${1:-}" = --direct ]; then
  direct=true
  shift
fi
base=${1:-origin/development}
merge_base=$(git merge-base HEAD "$base")

backend_changes=$(
  {
    git diff --no-renames --name-only "$merge_base" -- backend
    git ls-files --others --exclude-standard -- backend
  } | sort -u
)

if printf '%s\n' "$backend_changes" | grep -qxE 'backend/go\.(mod|sum)'; then
  printf './...\n'
  exit 0
fi

# backend/testdata holds fixtures shared across packages. Tests read them by
# relative path, so select every test package that names the changed file.
shared_fixture_dirs=$(
  printf '%s\n' "$backend_changes" | sed -n 's|^backend/testdata/||p' |
    while IFS= read -r fixture_name; do
      git grep -lF -e "testdata/$fixture_name" -- 'backend/*_test.go' || true
    done | sed -e 's|^backend/||' -e 's|/[^/]*$||' -e 's|^|test |'
)

changed_dirs=$({ printf '%s\n' "$shared_fixture_dirs"; printf '%s\n' "$backend_changes" | awk -v direct="$direct" '
  /^backend\/.*\/testdata\// {
    path = $0
    sub(/^backend\//, "", path)
    sub(/\/testdata\/.*/, "", path)
    print "test", path
    next
  }
  /^backend\/architecture\/policy(\.json|\/)/ { print "test internal/architecture"; next }
  # Read by path from tests; no import edge leads to these files. Their
  # readers in ./test come with every selection, or in --direct mode with the
  # source ratchets of test-changed.sh.
  /^backend\/architecture\/composition\.json$/ { print "test cmd"; next }
  /^backend\/auth\/authorize\/permissions\/catalog\.json$/ { print "test modules/identityaccess/internal/domain"; next }
  /^backend\/architecture\/(command-effects|passthrough)(\.json|\/)/ { print "test internal/architecture"; print "test internal/passthrough"; print "test test"; next }
  /^backend\/test\/query-budgets\// { print "production test/query-budgets"; print "test internal/architecture"; next }
  # CI needs the audit readers; --direct uses the source-test inventory instead.
  /^backend\/database\/audits\/[^/]+\.sql$/ { if (direct != "true") print "test test"; next }
  /\.go$/ {
    path = $0
    kind = path ~ /_test\.go$/ ? "test" : "production"
    sub(/^backend\//, "", path)
    if (path !~ /\//) {
      path = "."
    } else {
      sub(/\/[^\/]+$/, "", path)
    }
    print kind, path
    next
  }
  /^backend\/templates\/email\/.*\.html$/ { print "test templates/email"; next }
  /^backend\/test\/[^\/]+\.json$/ { print "test test"; next }
  /^backend\/modules\/delivery\/internal\/adapters\/localization\/locales\.json$/ {
    print "production modules/delivery/internal/adapters/localization"
    next
  }
  # Embedded registers are production data of their package. The migration
  # coverage test also reads the lifecycle catalog by path.
  /^backend\/modules\/organizationtenancy\/(return_register|data_lifecycle_catalog)\.json$/ {
    print "production modules/organizationtenancy"
    if ($0 ~ /data_lifecycle_catalog/) print "test database/migrations"
    next
  }
  # The OpenAPI description of the data return is embedded by its route.
  /^backend\/modules\/organizationtenancy\/inbound\/datareturn\/openapi\.json$/ {
    print "production modules/organizationtenancy/inbound/datareturn"
    next
  }
  /^backend\/modules\/documentrendering\/internal\/application\/listrenderer\/assets\// {
    print "production modules/documentrendering/internal/application/listrenderer"
  }
'; } | sed '/^$/d' | sort -u)

if [ -z "$changed_dirs" ]; then
  exit 0
fi

existing_dirs=()
deleted_production_dirs=()
while read -r kind dir; do
  package_dir="$repo_root/backend"
  if [ "$dir" != "." ]; then
    package_dir="$package_dir/$dir"
  fi
  # A deleted production package may leave only external test files behind.
  package_exists=
  for go_file in "$package_dir"/*.go; do
    [ -e "$go_file" ] || continue
    if [ "$kind" = test ] || [[ "$go_file" != *_test.go ]]; then
      package_exists=$go_file
      break
    fi
  done
  if [ -n "$package_exists" ]; then
    existing_dirs+=("$kind $dir")
  elif [ "$kind" = production ]; then
    deleted_production_dirs+=("$dir")
  fi
done <<< "$changed_dirs"

if [ "${#deleted_production_dirs[@]}" -gt 0 ]; then
  # There is no package left to test, but surviving imports of the removed
  # package must fail instead of turning the changed-test run into a no-op.
  # -export would compile every test dependency before selection even starts.
  # Load the graph only, and also reject imports of packages left with just
  # test files: go list accepts those, but an ordinary import cannot build.
  module=$(cd "$repo_root/backend" && go list -m)
  deleted_packages=$(printf '%s\n' "${deleted_production_dirs[@]}" | awk -v module="$module" '
    { print ($0 == "." ? module : module "/" $0) }
  ')
  (cd "$repo_root/backend" && go list -tags=testhooks -deps -test -f '{{range .Imports}}{{println .}}{{end}}' ./...) |
    awk 'NR == FNR { deleted[$0] = 1; next }
      $0 in deleted {
        print "deleted production package is still imported: " $0 > "/dev/stderr"
        failed = 1
      }
      END { exit failed }
    ' <(printf '%s\n' "$deleted_packages") -
fi

if [ "${#existing_dirs[@]}" -eq 0 ]; then
  exit 0
fi
changed_dirs=$(printf '%s\n' "${existing_dirs[@]}")

cd "$repo_root/backend"
module=$(go list -m)
changed=$(printf '%s\n' "$changed_dirs" | awk -v module="$module" '
  { dirs[$2] = 1 }
  END {
    for (dir in dirs) {
      if (dir == ".") print module
      else print module "/" dir
    }
  }
')
production_changed=$(printf '%s\n' "$changed_dirs" | awk -v module="$module" '
  $1 != "production" { next }
  { dirs[$2] = 1 }
  END {
    for (dir in dirs) {
      if (dir == ".") print module
      else print module "/" dir
    }
  }
')

# Depth of the importer expansion: .Deps is the transitive closure, .Imports
# only the direct edges (--direct).
deps_field=Deps
if [ "$direct" = true ]; then
  deps_field=Imports
fi

{
  printf '%s\n' "$changed"
  # The test package contains repository-wide source scanners and ratchets.
  # Import-graph selection cannot discover that dependency, so every non-empty
  # backend selection must include it explicitly. --direct leaves the targeted
  # filesystem dependency selection to test-changed.sh instead of running all
  # of ./test (which includes database integration tests).
  if [ "$direct" != true ]; then
    printf '%s/test\n' "$module"
  fi
  if [ -n "$production_changed" ]; then
    # -test includes the ordinary packages too, so load both graphs once.
    # ForTest maps test variants to their owner; Match retains ordinary
    # packages without tests and excludes generated test mains. Do not filter
    # by a .test suffix: a real package may legitimately have that name.
    go list -tags=testhooks -test -f "{{if .ForTest}}{{.ForTest}}{{range .$deps_field}} {{.}}{{end}}{{else if .Match}}{{.ImportPath}}{{range .$deps_field}} {{.}}{{end}}{{end}}" ./... |
      awk 'NR == FNR { changed[$0] = 1; next }
        {
          for (field = 2; field <= NF; field++) {
            if ($field in changed) {
              print $1
              next
            }
          }
        }' <(printf '%s\n' "$production_changed") -
  fi
} | sort -u
