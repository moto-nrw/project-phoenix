# Product screenshot execution

The full catalog and all device/PNG/WebP variants stay unchanged. Capture runs
up to four independent shots concurrently; each shot's devices stay sequential.
Communication and enrollment views are barriers because reading messages or letters can update
shared read cursors. Every shot before such a view finishes first, and subsequent
shots wait until that view has finished all its devices. Browser contexts remain
isolated, while one in-flight login per portal/role prevents competing MFA flows.

Framing runs on two raw images at once. Each image retains both PNGs and all six
WebP widths, quality settings, native dimensions and transparent device framing.
Output order follows the catalog, not completion order. A failed task drains
already started work before browser cleanup or output abort. No partial manifest
is published and the previous complete output remains until successful replacement.

The generation entry point uses Playwright's `--workers` budget for the internal
capture pool, with framing capped at two images. For a serial comparison:

```sh
pnpm run generate:screenshots --workers=1
```

Normal generation uses the config's four-worker budget. Direct `runPipeline`
callers can set `captureConcurrency` and `frameConcurrency` independently. Invalid
(non-positive or fractional) limits fail before capture. This is script-local
configuration, not application infrastructure or a tenant setting.

Logs include monotonic capture and framing durations, the configured limits and
individual capture durations. Compare identical catalogs, seed data and build
modes. A process-wide clock change cannot corrupt these duration measurements.

The existing GitHub workflow accepts `capture_workers=1` or `4` on manual
runs. Use the same source revision and version for a one-time serial/parallel
comparison. Normal PR/release runs use four without duplicating the workflow.
The disposable CI VM owns the fixed screenshot clock; never change the host
Mac clock to imitate that setup.
