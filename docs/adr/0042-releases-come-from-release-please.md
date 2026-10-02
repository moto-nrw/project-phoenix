---
status: accepted
---

# Releases are versioned by release-please, and each release produces product screenshots

## Context

moto had no release concept. Production deployed on every push to `main`,
and the only release identifier was the commit SHA passed to Sentry
(`SENTRY_RELEASE`). `frontend/package.json` carried a placeholder `0.1.0`
and git tags were ad-hoc PR-evidence tags.

Product screenshots for the website, sales material and social media need a
stable, human-readable version: a folder per release in Google Drive, and a
"current" folder that is replaced when a new release exists. A commit SHA is
meaningless to marketing, and deploying on every push is too frequent to
publish screenshots each time.

## Decision

Releases are cut by release-please from the Conventional Commits already
used in the repository (`feat`, `fix`, ...). release-please keeps a release
PR with the next semantic version and changelog; merging it creates the tag
`vX.Y.Z` and the GitHub release. The product-screenshot pipeline runs on
that release event (and manually), not on every deploy.

## Alternatives

- Manual tags before merging to `main`: depends on someone remembering it.
- Calendar versions (`2026.10.02`): carry no signal about what changed.
- Every push to `main`: too frequent, and there is no version to name folders.

## Consequences

- Version numbers exist from now on, independent of the deploy cadence.
- Deploys and releases are separate: not every deploy is a release.
- Commit types now affect the version, so a mislabelled `feat`/`fix` changes
  the next version number.
- A release created with `GITHUB_TOKEN` triggers no other workflow, so the
  release workflow calls the screenshot pipeline itself, and the release PR
  runs no PR checks (`main` requires none).
