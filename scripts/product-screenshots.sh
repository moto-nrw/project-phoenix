#!/usr/bin/env bash
# product-screenshots.sh: erzeugt die Produkt-Screenshots (#3759) lokal gegen den
# laufenden Dev-Stack. Ohne Upload: die Bilder landen nur im Ausgabeverzeichnis.
#
#   scripts/product-screenshots.sh                       alle Shots
#   scripts/product-screenshots.sh <shot-id>...          nur diese Shots
#   scripts/product-screenshots.sh --out DIR --version NAME [<shot-id>...]
#
# Voraussetzung: der Stack läuft (scripts/dev-native.sh up) und ist mit dem
# Profil `marketing` geseedet (scripts/dev-native.sh backend go run . seed ...).
# Die Shot-Liste steht in frontend/scripts/product-screenshots/shots.yaml.
#
# Das Ausgabeverzeichnis (Standard: tmp/product-screenshots) wird bei jedem Lauf
# ersetzt, aber nur, wenn es leer ist oder von dieser Pipeline stammt. Ein
# kaputter Shot bricht den Lauf ab; dann bleibt die vorige Ausgabe stehen.
set -euo pipefail

die() { echo "product-screenshots: $*" >&2; exit 1; }

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || die "not inside a git repository"
[ -f "$ROOT/frontend/package.json" ] || die "$ROOT is not a project-phoenix checkout"

out="$ROOT/tmp/product-screenshots"
version="lokal"
ids=()
while [ $# -gt 0 ]; do
  case "$1" in
    --out) [ $# -ge 2 ] || die "--out needs a directory"; out=$2; shift 2 ;;
    --version) [ $# -ge 2 ] || die "--version needs a name"; version=$2; shift 2 ;;
    -h | --help) sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) die "unknown option $1" ;;
    *) ids+=("$1"); shift ;;
  esac
done

[ -f "$ROOT/backend/.seed-state.json" ] ||
  die "backend/.seed-state.json fehlt: den Stack zuerst seeden (docs/agents/operations.md, Native dev loop)"
case "$out" in /*) ;; *) out="$PWD/$out" ;; esac

ids_csv=$(IFS=,; echo "${ids[*]:-}")
export SHOT_IDS="$ids_csv" SHOT_OUT="$out" SHOT_VERSION="$version"

cd "$ROOT"
exec scripts/dev-native.sh frontend pnpm run generate:screenshots
