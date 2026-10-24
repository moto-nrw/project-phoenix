---
status: accepted
---

# Product-screenshot consumers pin a release version

## Context

The product-screenshot pipeline (ADR 0042) produces every shot of the shot
list per release. Three places should show them: the website
(`moto-nrw/website`, static Next.js export), the downloadable PDFs on the
website (parent info, installing the parent app, push notifications), and
the help pages in this repository. Today the website and the PDFs carry
hand-made copies, and help articles set no image.

A pipeline run can produce wrong but formally valid images. In release
`v1.0.0`, avatars were captured before their photos loaded, and an
onboarding checklist covered half of a shot. Anything that shows the
newest output without a human step would have published that.

## Decision

The pipeline is the only producer of product-screenshot images. Each
release publishes its output under the release version to a public,
S3-compatible bucket on Hetzner Object Storage (where moto is hosted),
next to a `manifest.json` listing every shot by ID. The ZIP on the GitHub
release stays.

Each consumer pins one release version in its own repository and refers to
images by shot ID, never by file name. After a release, an automated PR
raises the pinned version in each consumer. A human reviews the preview and
merges it; no consumer follows "latest".

Shot IDs in `shots.yaml` are therefore a contract. Each consumer checks in
CI that every ID it uses exists in the manifest of its pinned version.

## Considered options

- **Follow "latest"**: no review step; a broken run reaches customers
  directly.
- **GitHub release assets as the source**: the repository is private, so
  browsers and other repositories' builds need a token.
- **Commit images into each consumer's git history**: every run changes
  every image, which adds megabytes of binaries per release.

## Consequences

- Help pages show the screenshots of the previous release. A PR that changes
  a documented flow adds or adjusts the shot; its image arrives with the
  next release and the following version bump.
- Renaming or removing a shot ID breaks a consumer's CI instead of silently
  breaking a page.
- The bucket is public. It holds only the synthetic marketing school.
- This is the target state, not yet built; consumers keep their manual
  copies until they switch. Tracked in #3760.
