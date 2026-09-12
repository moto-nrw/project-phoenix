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
      // Objektansichten auf Routen (#3115): die alten Auswahl-Parameter der
      // Register (?student=, ?staff=, ?room=) und das Raum-Panel (?room= auf
      // /rooms) führen auf die Objektroute. Nichts läuft ins Leere
      // (BAUARTEN-SPEC Teil 2): Lesezeichen und gemerkte Links kommen an.
      // `from` trägt den Rückweg, wie ihn die Register selbst setzen.
      ...legacySelectionRedirects(),
    ];
  },
};

/**
 * Ein Auswahl-Parameter, der früher ein Pane oder Panel öffnete, wird zur
 * Objektroute; einmal ohne und einmal mit Mandanten-Präfix (Pfad-Routing).
 * Der Name im Regex ist der Wert für die Zielroute; weitere Parameter reicht
 * Next unverändert an das Ziel durch.
 */
function legacySelectionRedirects() {
  // Der Mandanten-Präfix darf keinen festen ersten Pfadteil schlucken:
  // `/database/rooms?room=1` ist das Register, kein Mandant „database".
  const TENANT =
    "/:tenant((?!(?:api|database|rooms|students|staff)(?:/|$))[^/]+)";
  const rules = [
    // Das Panel „Unterwegs" auf /rooms ist die Seite /rooms/unterwegs.
    {
      source: "/rooms",
      query: { key: "room", value: "__transit__" },
      destination: "/rooms/unterwegs",
    },
    {
      source: "/rooms",
      query: { key: "room", value: "(?<room>.+)" },
      destination: "/rooms/:room",
    },
    {
      source: "/database/rooms",
      query: { key: "room", value: "(?<room>.+)" },
      destination: "/rooms/:room?from=%2Fdatabase%2Frooms",
    },
    {
      source: "/database/students",
      query: { key: "student", value: "(?<student>.+)" },
      destination: "/students/:student?from=%2Fdatabase%2Fstudents",
    },
    {
      source: "/database/personal",
      query: { key: "staff", value: "(?<staff>.+)" },
      destination: "/staff/:staff?from=%2Fdatabase%2Fpersonal",
    },
  ];
  /** @type {"query"} */
  const query = "query";
  // Erst alle Regeln ohne Präfix, dann die mit: Next nimmt die erste
  // passende Regel, und eine feste Route darf nie als Mandant gelesen werden.
  return [
    ...rules.map((rule) => ({
      source: rule.source,
      has: [{ type: query, ...rule.query }],
      destination: rule.destination,
      permanent: false,
    })),
    ...rules.map((rule) => ({
      source: `${TENANT}${rule.source}`,
      has: [{ type: query, ...rule.query }],
      destination: `/:tenant${rule.destination}`,
      permanent: false,
    })),
  ];
}
const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

export default withSentryConfig(withNextIntl(config), {
  silent: true,

  sourcemaps: {
    disable: true,
  },

  tunnelRoute: "/monitoring",
});
