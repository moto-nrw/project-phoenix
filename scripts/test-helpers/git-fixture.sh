#!/usr/bin/env bash
# Source only in standalone fixture-test processes, never in production hooks.
# Git child processes (including the script under test) inherit this isolation.
# https://git-scm.com/docs/git#Documentation/git.txt-GITCONFIGGLOBAL
while IFS= read -r git_fixture_variable; do
  unset "$git_fixture_variable"
done < <(compgen -v GIT_)
unset git_fixture_variable
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1

init_git_fixture() {
  local root=$1
  git -C "$root" init --quiet --template=
  git -C "$root" config --local core.hooksPath /dev/null
  git -C "$root" config --local user.name 'Git fixture'
  git -C "$root" config --local user.email fixture@example.invalid
  git -C "$root" config --local commit.gpgsign false
  git -C "$root" config --local gc.auto 0
  git -C "$root" config --local maintenance.auto false
}
