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
  async redirects() {
    return [
      {
        source: "/students/:id/feedback_history",
        destination: "/students/:id/feedback-history",
        permanent: true,
      },
      {
        source: "/:tenant/students/:id/feedback_history",
        destination: "/:tenant/students/:id/feedback-history",
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
        source: "/:tenant/activities/:id",
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
