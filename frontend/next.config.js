/**
 * Run `build` or `dev` with `SKIP_ENV_VALIDATION` to skip env validation. This is especially useful
 * for Docker builds.
 */
import "./src/env.js";
import { withSentryConfig } from "@sentry/nextjs";
import createNextIntlPlugin from "next-intl/plugin";

/** @type {import("next").NextConfig} */
const config = {
  output: "standalone",
  experimental: {
    optimizePackageImports: ["@phosphor-icons/react"],
  },
  async redirects() {
    // :tenant must exclude the literal "api" segment: next.config redirects
    // run before route handlers, so an unguarded /:tenant/... source would
    // also capture /api/... fetches (e.g. /api/activities/123).
    return [
      {
        source: "/students/:id/feedback_history",
        destination: "/students/:id",
        permanent: true,
      },
      {
        source: "/:tenant((?!api(?:/|$))[^/]+)/students/:id/feedback_history",
        destination: "/:tenant/students/:id",
        permanent: true,
      },
      // Legacy deep-link target: the mensa history page was removed.
      {
        source: "/students/:id/mensa_history",
        destination: "/students/:id",
        permanent: true,
      },
      {
        source: "/:tenant((?!api(?:/|$))[^/]+)/students/:id/mensa_history",
        destination: "/:tenant/students/:id",
        permanent: true,
      },
      // Legacy deep-link target: activity management now happens from the
      // canonical /activities list/modal flow.
      {
        source: "/activities/:id",
        destination: "/activities",
        permanent: true,
      },
      {
        source: "/:tenant((?!api(?:/|$))[^/]+)/activities/:id",
        destination: "/:tenant/activities",
        permanent: true,
      },
    ];
  },
};
const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

export default withSentryConfig(withNextIntl(config), {
  silent: true,

  sourcemaps: {
    disable: true,
  },

  tunnelRoute: "/monitoring",
});
