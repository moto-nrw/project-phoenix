/**
 * Run `build` or `dev` with `SKIP_ENV_VALIDATION` to skip env validation. This is especially useful
 * for Docker builds.
 */
import "./src/env.js";
import { withSentryConfig } from "@sentry/nextjs";

/** @type {import("next").NextConfig} */
const config = {
  // Allow the Next.js dev server (HMR, _next/* assets) to be reached from
  // tenant subdomains. Without this, Next 16+ blocks cross-origin requests
  // and React fails to hydrate on {slug}.localtest.me, leaving the page
  // stuck in its loading state. Dev-only — production is unaffected.
  allowedDevOrigins: [
    "*.localtest.me",
    "localtest.me",
    "*.localhost",
    "localhost",
  ],
};

export default withSentryConfig(config, {
  silent: true,

  sourcemaps: {
    disable: true,
  },

  tunnelRoute: "/monitoring",
});
