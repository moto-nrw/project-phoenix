#!/usr/bin/env bash
# Prints changed backend packages. Production changes also select every package
# whose production or test graph depends on them. Output is one import per line.
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"
base=${1:-origin/development}
merge_base=$(git merge-base HEAD "$base")

backend_changes=$(
  {
    git diff --name-only "$merge_base" -- backend
    git ls-files --others --exclude-standard -- backend
  } | sort -u
)

if printf '%s\n' "$backend_changes" | grep -qxE 'backend/go\.(mod|sum)'; then
  printf './...\n'
  exit 0
fi

changed_dirs=$(printf '%s\n' "$backend_changes" | awk '
  /^backend\/.*\/testdata\// {
    path = $0
    sub(/^backend\//, "", path)
    sub(/\/testdata\/.*/, "", path)
    print "test", path
    next
  }
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
  /^backend\/localization\/locales\.json$/ { print "production localization"; next }
  /^backend\/services\/listexport\/assets\// { print "production services/listexport" }
' | sort -u)

if [ -z "$changed_dirs" ]; then
  exit 0
fi

existing_dirs=()
deleted_production=false
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
    deleted_production=true
  fi
done <<< "$changed_dirs"

if [ "$deleted_production" = true ]; then
  # There is no package left to test, but surviving imports of the removed
  # package must fail instead of turning the changed-test run into a no-op.
  (cd "$repo_root/backend" && go list -deps -test -export ./... >/dev/null)
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

{
  printf '%s\n' "$changed"
  # The test package contains repository-wide source scanners and ratchets.
  # Import-graph selection cannot discover that dependency, so every non-empty
  # backend selection must include it explicitly.
  printf '%s/test\n' "$module"
  if [ -n "$production_changed" ]; then
    {
      go list -f '{{.ImportPath}}{{range .Deps}} {{.}}{{end}}' ./...
      go list -test -f '{{if .ForTest}}{{.ForTest}}{{range .Deps}} {{.}}{{end}}{{end}}' ./...
    } |
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
