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
        destination: "/students/:id/feedback-history",
        permanent: true,
      },
      {
        source: "/:tenant((?!api(?:/|$))[^/]+)/students/:id/feedback_history",
        destination: "/:tenant/students/:id/feedback-history",
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
      // The three guide pages became one topic-per-page help area. Their URLs
      // are in circulation (printed onepager, bookmarks, support mails), so
      // they must not 404.
      //
      // Old anchor links keep working without a rule per topic: a browser
      // carries the fragment across a redirect whose target has none, so
      // /help/features#kindersuche arrives as /help#kindersuche, and HelpView
      // turns a hash that names a known topic into /help/kindersuche.
      //
      // /help/nfc/erste-schritte is deliberately absent — that page still
      // exists as the printed NFC onepager, and a redirect on the parent path
      // does not capture it.
      {
        source: "/help/setup",
        destination: "/help",
        permanent: true,
      },
      {
        source: "/help/features",
        destination: "/help",
        permanent: true,
      },
      {
        source: "/help/nfc",
        destination: "/help",
        permanent: true,
      },
    ];
  },
};
const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

export default withSentryConfig(withNextIntl(config), {
  silent: true,

  // Readable stack traces need the source maps in Sentry. The upload runs
  // only when the build has SENTRY_AUTH_TOKEN (plus SENTRY_ORG,
  // SENTRY_PROJECT and SENTRY_RELEASE, all read from the environment); the
  // maps are deleted after upload and never served. Local and CI builds
  // without the token build exactly as before.
  sourcemaps: {
    disable: !process.env.SENTRY_AUTH_TOKEN,
    deleteSourcemapsAfterUpload: true,
  },

  tunnelRoute: "/monitoring",
});
